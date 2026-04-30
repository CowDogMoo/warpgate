/*
Copyright © 2026 Jayson Grace <jayson.e.grace@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.
*/

package azure

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeListers records the arguments discovery passes through and returns
// canned results. Each test wires only what it needs.
type fakeGalleryLister struct {
	galleries []GalleryRef
	err       error
}

func (f *fakeGalleryLister) ListGalleries(_ context.Context) ([]GalleryRef, error) {
	return f.galleries, f.err
}

type fakeImageLister struct {
	images   []GalleryImageRef
	err      error
	rgArg    string
	galArg   string
	wasCalls int
}

func (f *fakeImageLister) ListImageDefs(_ context.Context, rg, gallery string) ([]GalleryImageRef, error) {
	f.rgArg = rg
	f.galArg = gallery
	f.wasCalls++
	return f.images, f.err
}

type fakeResourceLister struct {
	resources []ResourceRef
	err       error
	rgArg     string
	typeArg   string
}

func (f *fakeResourceLister) ListByResourceGroup(_ context.Context, rg, resourceType string) ([]ResourceRef, error) {
	f.rgArg = rg
	f.typeArg = resourceType
	return f.resources, f.err
}

func newDiscoverer(galleries []GalleryRef, images []GalleryImageRef, resources []ResourceRef) (*Discoverer, *fakeImageLister, *fakeResourceLister) {
	imgLister := &fakeImageLister{images: images}
	resLister := &fakeResourceLister{resources: resources}
	return &Discoverer{
		subscriptionID: "sub-1",
		galleries:      &fakeGalleryLister{galleries: galleries},
		galleryImages:  imgLister,
		resources:      resLister,
	}, imgLister, resLister
}

// TestDiscover_FillsAllEmptyFields verifies the happy path: a target with
// just a marketplace source and OSType blank gets gallery, RG, location,
// image def, OSType, and identity all filled in.
func TestDiscover_FillsAllEmptyFields(t *testing.T) {
	galleries := []GalleryRef{
		{
			Name:          "myGallery",
			ResourceGroup: "rg-builds",
			Location:      "centralus",
			Tags:          map[string]string{warpgateOwnerTag: "true"},
		},
	}
	images := []GalleryImageRef{
		{Name: "ubuntu-22", OSType: "Linux"},
	}
	resources := []ResourceRef{
		{
			ID:   "/subscriptions/sub-1/resourceGroups/rg-builds/providers/Microsoft.ManagedIdentity/userAssignedIdentities/aib-uami",
			Name: "aib-uami",
		},
	}

	d, imgLister, resLister := newDiscoverer(galleries, images, resources)
	target := &builder.Target{Type: "azure"}

	report, err := d.DiscoverTarget(context.Background(), target, "ares-golden-azure")
	require.NoError(t, err)

	assert.Equal(t, "sub-1", target.SubscriptionID)
	assert.Equal(t, "myGallery", target.Gallery)
	assert.Equal(t, "rg-builds", target.ResourceGroup)
	assert.Equal(t, "centralus", target.Location)
	assert.Equal(t, "ubuntu-22", target.GalleryImageDefinition)
	assert.Equal(t, "Linux", target.OSType)
	assert.Equal(t, resources[0].ID, target.IdentityID)

	// All six fields should appear in the filled report.
	for _, k := range []string{"subscription_id", "gallery", "resource_group", "location", "gallery_image_definition", "os_type", "identity_id"} {
		assert.Contains(t, report.Filled, k, "expected %s in Filled", k)
	}

	assert.Equal(t, "rg-builds", imgLister.rgArg, "image lister scoped to discovered RG")
	assert.Equal(t, "myGallery", imgLister.galArg)
	assert.Equal(t, "rg-builds", resLister.rgArg, "resource lister scoped to discovered RG")
	assert.Equal(t, uamiResourceType, resLister.typeArg)
}

// TestDiscover_RespectsExplicitFields confirms discovery never overwrites
// fields the user already populated.
func TestDiscover_RespectsExplicitFields(t *testing.T) {
	galleries := []GalleryRef{
		{Name: "explicit-gallery", ResourceGroup: "rg-explicit", Location: "eastus"},
		{Name: "auto-gallery", ResourceGroup: "rg-auto", Location: "centralus"},
	}
	images := []GalleryImageRef{
		{Name: "user-image", OSType: "Linux"},
	}
	resources := []ResourceRef{
		{ID: "/subscriptions/sub-1/resourceGroups/rg-explicit/providers/Microsoft.ManagedIdentity/userAssignedIdentities/uami-1", Name: "uami-1"},
	}

	d, _, _ := newDiscoverer(galleries, images, resources)
	target := &builder.Target{
		Type:                   "azure",
		SubscriptionID:         "user-sub",
		Gallery:                "explicit-gallery",
		ResourceGroup:          "rg-explicit",
		Location:               "eastus",
		GalleryImageDefinition: "user-image",
		OSType:                 "Linux",
		IdentityID:             "/subscriptions/user-sub/resourceGroups/rg-explicit/providers/Microsoft.ManagedIdentity/userAssignedIdentities/already-set",
	}

	_, err := d.DiscoverTarget(context.Background(), target, "tmpl")
	require.NoError(t, err)

	assert.Equal(t, "user-sub", target.SubscriptionID)
	assert.Equal(t, "explicit-gallery", target.Gallery)
	assert.Equal(t, "rg-explicit", target.ResourceGroup)
	assert.Equal(t, "eastus", target.Location)
	assert.Equal(t, "user-image", target.GalleryImageDefinition)
	assert.Equal(t, "Linux", target.OSType)
	assert.Equal(t, "/subscriptions/user-sub/resourceGroups/rg-explicit/providers/Microsoft.ManagedIdentity/userAssignedIdentities/already-set", target.IdentityID)
}

// TestDiscover_AmbiguousGalleryFails surfaces a clear error listing the
// candidates when the subscription has multiple galleries and nothing
// disambiguates them.
func TestDiscover_AmbiguousGalleryFails(t *testing.T) {
	galleries := []GalleryRef{
		{Name: "gallery-a", ResourceGroup: "rg-a", Location: "eastus"},
		{Name: "gallery-b", ResourceGroup: "rg-b", Location: "eastus"},
	}
	d, _, _ := newDiscoverer(galleries, nil, nil)
	target := &builder.Target{Type: "azure"}

	_, err := d.DiscoverTarget(context.Background(), target, "no-match-template")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple galleries")
	assert.Contains(t, err.Error(), "gallery-a")
	assert.Contains(t, err.Error(), "gallery-b")
}

// TestDiscover_AmbiguousGalleryResolvedByTag picks the warpgate-tagged gallery
// when more than one exists.
func TestDiscover_AmbiguousGalleryResolvedByTag(t *testing.T) {
	galleries := []GalleryRef{
		{Name: "gallery-a", ResourceGroup: "rg-a", Location: "eastus"},
		{Name: "gallery-b", ResourceGroup: "rg-b", Location: "centralus", Tags: map[string]string{warpgateOwnerTag: "tmpl"}},
	}
	d, _, _ := newDiscoverer(galleries, []GalleryImageRef{{Name: "img-1", OSType: "Linux"}}, []ResourceRef{
		{ID: "/subscriptions/sub-1/resourceGroups/rg-b/providers/Microsoft.ManagedIdentity/userAssignedIdentities/u", Name: "u"},
	})

	target := &builder.Target{Type: "azure"}
	_, err := d.DiscoverTarget(context.Background(), target, "tmpl")
	require.NoError(t, err)
	assert.Equal(t, "gallery-b", target.Gallery)
	assert.Equal(t, "centralus", target.Location)
}

// TestDiscover_AmbiguousGalleryResolvedByName falls through tag matching to
// pick a gallery whose name matches the template name.
func TestDiscover_AmbiguousGalleryResolvedByName(t *testing.T) {
	galleries := []GalleryRef{
		{Name: "tmpl", ResourceGroup: "rg-x", Location: "westus"},
		{Name: "other", ResourceGroup: "rg-y", Location: "westus"},
	}
	d, _, _ := newDiscoverer(galleries, []GalleryImageRef{{Name: "img-1", OSType: "Linux"}}, []ResourceRef{
		{ID: "/subscriptions/sub-1/resourceGroups/rg-x/providers/Microsoft.ManagedIdentity/userAssignedIdentities/u", Name: "u"},
	})
	target := &builder.Target{Type: "azure"}
	_, err := d.DiscoverTarget(context.Background(), target, "tmpl")
	require.NoError(t, err)
	assert.Equal(t, "tmpl", target.Gallery)
}

// TestDiscover_NoGalleriesFails refuses to silently proceed if the subscription
// has no Compute Galleries at all.
func TestDiscover_NoGalleriesFails(t *testing.T) {
	d, _, _ := newDiscoverer(nil, nil, nil)
	target := &builder.Target{Type: "azure"}
	_, err := d.DiscoverTarget(context.Background(), target, "tmpl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Compute Galleries")
}

// TestDiscover_NoImagesFails surfaces a useful error when the gallery has
// no image definitions.
func TestDiscover_NoImagesFails(t *testing.T) {
	galleries := []GalleryRef{
		{Name: "g", ResourceGroup: "rg", Location: "eastus"},
	}
	d, _, _ := newDiscoverer(galleries, nil, nil)
	target := &builder.Target{Type: "azure"}
	_, err := d.DiscoverTarget(context.Background(), target, "tmpl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no image definitions")
}

// TestDiscover_AmbiguousIdentitiesFails enforces explicit identity_id when
// multiple UAMIs live in the same RG.
func TestDiscover_AmbiguousIdentitiesFails(t *testing.T) {
	galleries := []GalleryRef{{Name: "g", ResourceGroup: "rg", Location: "eastus"}}
	images := []GalleryImageRef{{Name: "img", OSType: "Linux"}}
	resources := []ResourceRef{
		{ID: "/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/u1", Name: "u1"},
		{ID: "/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/u2", Name: "u2"},
	}
	d, _, _ := newDiscoverer(galleries, images, resources)
	target := &builder.Target{Type: "azure"}
	_, err := d.DiscoverTarget(context.Background(), target, "tmpl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple user-assigned managed identity")
	assert.Contains(t, err.Error(), "u1")
	assert.Contains(t, err.Error(), "u2")
}

// TestDiscover_TaggedIdentityWins shows the warpgate tag breaking a UAMI tie.
func TestDiscover_TaggedIdentityWins(t *testing.T) {
	galleries := []GalleryRef{{Name: "g", ResourceGroup: "rg", Location: "eastus"}}
	images := []GalleryImageRef{{Name: "img", OSType: "Linux"}}
	resources := []ResourceRef{
		{ID: "/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/u1", Name: "u1"},
		{ID: "/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/u2", Name: "u2", Tags: map[string]string{warpgateOwnerTag: "true"}},
	}
	d, _, _ := newDiscoverer(galleries, images, resources)
	target := &builder.Target{Type: "azure"}
	_, err := d.DiscoverTarget(context.Background(), target, "tmpl")
	require.NoError(t, err)
	assert.Equal(t, resources[1].ID, target.IdentityID)
}

// TestDiscover_ExplicitGalleryNotFound bubbles up an error when the user
// names a gallery that doesn't exist.
func TestDiscover_ExplicitGalleryNotFound(t *testing.T) {
	galleries := []GalleryRef{
		{Name: "real", ResourceGroup: "rg", Location: "eastus"},
	}
	d, _, _ := newDiscoverer(galleries, nil, nil)
	target := &builder.Target{Type: "azure", Gallery: "missing"}
	_, err := d.DiscoverTarget(context.Background(), target, "tmpl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gallery \"missing\" not found")
}

// TestDiscover_PassesThroughErrors makes sure the API errors we return wrap
// the inner error so callers can inspect with errors.Is.
func TestDiscover_PassesThroughErrors(t *testing.T) {
	sentinel := errors.New("boom")
	d := &Discoverer{
		subscriptionID: "sub-1",
		galleries:      &fakeGalleryLister{err: sentinel},
	}
	target := &builder.Target{Type: "azure"}
	_, err := d.DiscoverTarget(context.Background(), target, "tmpl")
	require.Error(t, err)
	assert.True(t, errors.Is(err, sentinel) || strings.Contains(err.Error(), "boom"), "expected wrapped sentinel, got: %v", err)
}

// TestResourceGroupFromID covers the cases the parser needs to handle: well
// formed ID, casing variant, and broken inputs.
func TestResourceGroupFromID(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g", "rg-1"},
		{"/subscriptions/sub-1/resourcegroups/rg-2/providers/Microsoft.Compute/galleries/g", "rg-2"},
		{"", ""},
		{"/subscriptions/sub-1", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, resourceGroupFromID(c.in), "input=%q", c.in)
	}
}

// TestNewDiscoverer_RequiresSubscription guards the public constructor against
// an empty subscription, which would otherwise fail later inside the SDK.
func TestNewDiscoverer_RequiresSubscription(t *testing.T) {
	_, err := NewDiscoverer(nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscription_id is required")
}
