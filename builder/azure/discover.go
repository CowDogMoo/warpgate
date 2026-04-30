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

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package azure

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// uamiResourceType is the ARM resource type filter used to list user-assigned
// managed identities in a resource group.
const uamiResourceType = "Microsoft.ManagedIdentity/userAssignedIdentities"

// warpgateOwnerTag is the tag key used to disambiguate when multiple resources
// of the same kind are present in the search scope. A resource tagged with this
// key wins over an untagged peer.
const warpgateOwnerTag = "warpgate"

// GalleryRef is the discovery view of a Compute Gallery: enough to pick one
// and continue scoping subsequent lookups.
type GalleryRef struct {
	Name          string
	ResourceGroup string
	Location      string
	Tags          map[string]string
}

// GalleryImageRef is the discovery view of a Compute Gallery image definition.
type GalleryImageRef struct {
	Name   string
	OSType string
	Tags   map[string]string
}

// ResourceRef is the discovery view of a generic ARM resource (used today for
// listing user-assigned managed identities).
type ResourceRef struct {
	ID   string
	Name string
	Tags map[string]string
}

// galleryLister lists Compute Galleries in a subscription. The interface keeps
// discovery testable without standing up real Azure clients.
type galleryLister interface {
	ListGalleries(ctx context.Context) ([]GalleryRef, error)
}

// galleryImageLister lists image definitions inside a single Compute Gallery.
type galleryImageLister interface {
	ListImageDefs(ctx context.Context, resourceGroup, gallery string) ([]GalleryImageRef, error)
}

// resourceLister lists generic ARM resources scoped to a resource group, with
// an optional resourceType filter (used to find UAMIs today).
type resourceLister interface {
	ListByResourceGroup(ctx context.Context, resourceGroup, resourceTypeFilter string) ([]ResourceRef, error)
}

// Discoverer fills in Azure target fields the user did not specify. Discovery
// only ever populates empty fields; explicit YAML/CLI values are preserved.
type Discoverer struct {
	subscriptionID string
	galleries      galleryLister
	galleryImages  galleryImageLister
	resources      resourceLister
}

// DiscoveryReport captures what discovery filled in (and what it could not).
// It is logged so users can see which of their target fields came from Azure.
type DiscoveryReport struct {
	// Filled maps target field name to a short human-readable source.
	Filled map[string]string
	// Skipped maps target field name to a reason it was left untouched.
	Skipped map[string]string
}

// NewDiscoverer wires a Discoverer against real Azure SDK clients. Pass the
// same credential the build will use so we make a single auth handshake.
func NewDiscoverer(cred azcore.TokenCredential, subscriptionID string) (*Discoverer, error) {
	if subscriptionID == "" {
		return nil, fmt.Errorf("azure discovery: subscription_id is required")
	}
	galClient, err := armcompute.NewGalleriesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azure discovery: galleries client: %w", err)
	}
	imgClient, err := armcompute.NewGalleryImagesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azure discovery: gallery images client: %w", err)
	}
	resClient, err := armresources.NewClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azure discovery: resources client: %w", err)
	}
	return &Discoverer{
		subscriptionID: subscriptionID,
		galleries:      &galleriesAdapter{c: galClient},
		galleryImages:  &galleryImagesAdapter{c: imgClient},
		resources:      &resourcesAdapter{c: resClient},
	}, nil
}

// DiscoverTarget fills empty fields on target by inspecting the Azure
// subscription. Order is intentional: gallery first, then RG/Location/OSType
// flow from it; image definition narrows scope to a single gallery; UAMI is
// scoped to the chosen RG.
//
// templateName disambiguates name-based matches (e.g., a gallery image
// definition with the same name as the template wins over single-match fallback).
func (d *Discoverer) DiscoverTarget(ctx context.Context, target *builder.Target, templateName string) (DiscoveryReport, error) {
	report := DiscoveryReport{
		Filled:  map[string]string{},
		Skipped: map[string]string{},
	}

	if target.SubscriptionID == "" {
		target.SubscriptionID = d.subscriptionID
		report.Filled["subscription_id"] = "from azure credential"
	}

	gallery, err := d.resolveGallery(ctx, target, templateName, &report)
	if err != nil {
		return report, err
	}

	if target.ResourceGroup == "" && gallery.ResourceGroup != "" {
		target.ResourceGroup = gallery.ResourceGroup
		report.Filled["resource_group"] = fmt.Sprintf("from gallery %q", gallery.Name)
	}
	if target.Location == "" && gallery.Location != "" {
		target.Location = gallery.Location
		report.Filled["location"] = fmt.Sprintf("from gallery %q", gallery.Name)
	}

	if err := d.resolveGalleryImage(ctx, target, templateName, &report); err != nil {
		return report, err
	}

	if err := d.resolveIdentity(ctx, target, &report); err != nil {
		return report, err
	}

	logDiscovery(ctx, report)
	return report, nil
}

// resolveGallery either uses the gallery the user already named (looking it up
// to fill RG/Location/Tags) or picks one from the subscription using tag and
// single-match heuristics. The returned GalleryRef is always populated with at
// least Name and ResourceGroup so downstream lookups can proceed.
func (d *Discoverer) resolveGallery(ctx context.Context, target *builder.Target, templateName string, report *DiscoveryReport) (GalleryRef, error) {
	if target.Gallery != "" && target.ResourceGroup != "" && target.Location != "" {
		report.Skipped["gallery"] = "explicit"
		return GalleryRef{Name: target.Gallery, ResourceGroup: target.ResourceGroup, Location: target.Location}, nil
	}

	galleries, err := d.galleries.ListGalleries(ctx)
	if err != nil {
		return GalleryRef{}, fmt.Errorf("azure discovery: list galleries: %w", err)
	}
	if len(galleries) == 0 {
		return GalleryRef{}, fmt.Errorf("azure discovery: no Compute Galleries found in subscription %s; create one or set target.gallery explicitly", d.subscriptionID)
	}

	chosen, err := pickGallery(galleries, target.Gallery, templateName)
	if err != nil {
		return GalleryRef{}, err
	}

	if target.Gallery == "" {
		target.Gallery = chosen.Name
		report.Filled["gallery"] = matchSourceForGallery(chosen, templateName)
	} else {
		report.Skipped["gallery"] = "explicit"
	}
	return chosen, nil
}

// resolveGalleryImage fills target.GalleryImageDefinition (and target.OSType
// when the matched image definition exposes one) by listing image defs inside
// the resolved gallery. If the user already named the image def we leave it
// alone.
func (d *Discoverer) resolveGalleryImage(ctx context.Context, target *builder.Target, templateName string, report *DiscoveryReport) error {
	if target.GalleryImageDefinition != "" && target.OSType != "" {
		report.Skipped["gallery_image_definition"] = "explicit"
		return nil
	}
	if target.ResourceGroup == "" || target.Gallery == "" {
		// resolveGallery would have errored already if these were missing.
		return nil
	}

	images, err := d.galleryImages.ListImageDefs(ctx, target.ResourceGroup, target.Gallery)
	if err != nil {
		return fmt.Errorf("azure discovery: list image definitions in gallery %q: %w", target.Gallery, err)
	}
	if len(images) == 0 {
		if target.GalleryImageDefinition == "" {
			return fmt.Errorf("azure discovery: no image definitions in gallery %q (rg=%s); create one or set target.gallery_image_definition", target.Gallery, target.ResourceGroup)
		}
		return nil
	}

	chosen, err := pickGalleryImage(images, target.GalleryImageDefinition, templateName)
	if err != nil {
		return err
	}
	if target.GalleryImageDefinition == "" {
		target.GalleryImageDefinition = chosen.Name
		report.Filled["gallery_image_definition"] = matchSourceForImage(chosen, templateName)
	} else {
		report.Skipped["gallery_image_definition"] = "explicit"
	}
	if target.OSType == "" && chosen.OSType != "" {
		target.OSType = chosen.OSType
		report.Filled["os_type"] = fmt.Sprintf("from image definition %q", chosen.Name)
	}
	return nil
}

// resolveIdentity finds a single user-assigned managed identity in the
// resolved resource group and stamps its full ARM ID onto target.IdentityID.
// Multi-match disambiguates by the warpgate tag.
func (d *Discoverer) resolveIdentity(ctx context.Context, target *builder.Target, report *DiscoveryReport) error {
	if target.IdentityID != "" {
		report.Skipped["identity_id"] = "explicit"
		return nil
	}
	if target.ResourceGroup == "" {
		return nil
	}

	uamis, err := d.resources.ListByResourceGroup(ctx, target.ResourceGroup, uamiResourceType)
	if err != nil {
		return fmt.Errorf("azure discovery: list UAMIs in resource group %q: %w", target.ResourceGroup, err)
	}
	if len(uamis) == 0 {
		return fmt.Errorf("azure discovery: no user-assigned managed identities in resource group %q; create one or set target.identity_id", target.ResourceGroup)
	}

	chosen, err := pickResource(uamis, "user-assigned managed identity")
	if err != nil {
		return err
	}
	target.IdentityID = chosen.ID
	report.Filled["identity_id"] = fmt.Sprintf("UAMI %q in rg %q", chosen.Name, target.ResourceGroup)
	return nil
}

// pickGallery selects a gallery from candidates. If the user already named one
// we trust that name; otherwise we prefer (1) tag warpgate=<template>, (2)
// name == template, (3) the only candidate. Anything else is ambiguous.
func pickGallery(galleries []GalleryRef, explicitName, templateName string) (GalleryRef, error) {
	if explicitName != "" {
		for _, g := range galleries {
			if g.Name == explicitName {
				return g, nil
			}
		}
		return GalleryRef{}, fmt.Errorf("azure discovery: gallery %q not found in subscription", explicitName)
	}

	if tagged := matchByTag(galleries, templateName, func(g GalleryRef) map[string]string { return g.Tags }); len(tagged) == 1 {
		return tagged[0], nil
	}

	if templateName != "" {
		for _, g := range galleries {
			if g.Name == templateName {
				return g, nil
			}
		}
	}

	if len(galleries) == 1 {
		return galleries[0], nil
	}

	names := galleryNames(galleries)
	return GalleryRef{}, fmt.Errorf("azure discovery: multiple galleries found (%s); set target.gallery or tag one with %q=%q to disambiguate", strings.Join(names, ", "), warpgateOwnerTag, templateName)
}

// pickGalleryImage selects an image definition. Same precedence as galleries:
// explicit name, then tag, then template-name match, then single candidate.
func pickGalleryImage(images []GalleryImageRef, explicitName, templateName string) (GalleryImageRef, error) {
	if explicitName != "" {
		for _, img := range images {
			if img.Name == explicitName {
				return img, nil
			}
		}
		return GalleryImageRef{}, fmt.Errorf("azure discovery: gallery image definition %q not found", explicitName)
	}

	if tagged := matchByTag(images, templateName, func(g GalleryImageRef) map[string]string { return g.Tags }); len(tagged) == 1 {
		return tagged[0], nil
	}

	if templateName != "" {
		for _, img := range images {
			if img.Name == templateName {
				return img, nil
			}
		}
	}

	if len(images) == 1 {
		return images[0], nil
	}

	names := imageNames(images)
	return GalleryImageRef{}, fmt.Errorf("azure discovery: multiple gallery image definitions found (%s); set target.gallery_image_definition or name one after the template", strings.Join(names, ", "))
}

// pickResource handles UAMI selection within a resource group: prefer the
// warpgate-tagged candidate, otherwise the only one, otherwise fail with the
// list of candidates.
func pickResource(resources []ResourceRef, kind string) (ResourceRef, error) {
	if tagged := matchByTag(resources, "", func(r ResourceRef) map[string]string { return r.Tags }); len(tagged) == 1 {
		return tagged[0], nil
	}
	if len(resources) == 1 {
		return resources[0], nil
	}
	names := resourceNames(resources)
	return ResourceRef{}, fmt.Errorf("azure discovery: multiple %s candidates (%s); set the field explicitly or tag one with %q=true", kind, strings.Join(names, ", "), warpgateOwnerTag)
}

// matchByTag returns the subset of items whose tag map contains warpgateOwnerTag.
// When templateName is non-empty, only entries whose tag value equals it
// (or "true") are kept; otherwise any value matches. The generic shape lets
// gallery / image / resource lookups share a single rule.
func matchByTag[T any](items []T, templateName string, tagsOf func(T) map[string]string) []T {
	out := make([]T, 0, len(items))
	for _, item := range items {
		tags := tagsOf(item)
		if tags == nil {
			continue
		}
		v, ok := tags[warpgateOwnerTag]
		if !ok {
			continue
		}
		if templateName != "" && v != templateName && !strings.EqualFold(v, "true") {
			continue
		}
		out = append(out, item)
	}
	return out
}

func matchSourceForGallery(g GalleryRef, templateName string) string {
	if v, ok := g.Tags[warpgateOwnerTag]; ok {
		return fmt.Sprintf("tag %s=%s", warpgateOwnerTag, v)
	}
	if templateName != "" && g.Name == templateName {
		return "name matches template"
	}
	return "single gallery in subscription"
}

func matchSourceForImage(img GalleryImageRef, templateName string) string {
	if v, ok := img.Tags[warpgateOwnerTag]; ok {
		return fmt.Sprintf("tag %s=%s", warpgateOwnerTag, v)
	}
	if templateName != "" && img.Name == templateName {
		return "name matches template"
	}
	return "single image definition in gallery"
}

func galleryNames(galleries []GalleryRef) []string {
	names := make([]string, len(galleries))
	for i, g := range galleries {
		names[i] = g.Name
	}
	sort.Strings(names)
	return names
}

func imageNames(images []GalleryImageRef) []string {
	names := make([]string, len(images))
	for i, img := range images {
		names[i] = img.Name
	}
	sort.Strings(names)
	return names
}

func resourceNames(resources []ResourceRef) []string {
	names := make([]string, len(resources))
	for i, r := range resources {
		names[i] = r.Name
	}
	sort.Strings(names)
	return names
}

func logDiscovery(ctx context.Context, report DiscoveryReport) {
	if len(report.Filled) == 0 {
		return
	}
	keys := make([]string, 0, len(report.Filled))
	for k := range report.Filled {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		logging.InfoContext(ctx, "azure discovery: %s = %s", k, report.Filled[k])
	}
}

// galleriesAdapter wraps the real armcompute.GalleriesClient pager into the
// galleryLister slice-returning interface used by Discoverer.
type galleriesAdapter struct {
	c *armcompute.GalleriesClient
}

func (a *galleriesAdapter) ListGalleries(ctx context.Context) ([]GalleryRef, error) {
	pager := a.c.NewListPager(nil)
	var out []GalleryRef
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range page.Value {
			if g == nil || g.Name == nil || g.ID == nil {
				continue
			}
			ref := GalleryRef{
				Name:          *g.Name,
				ResourceGroup: resourceGroupFromID(*g.ID),
				Tags:          ptrMapToStringMap(g.Tags),
			}
			if g.Location != nil {
				ref.Location = *g.Location
			}
			out = append(out, ref)
		}
	}
	return out, nil
}

// galleryImagesAdapter wraps the real armcompute.GalleryImagesClient pager.
type galleryImagesAdapter struct {
	c *armcompute.GalleryImagesClient
}

func (a *galleryImagesAdapter) ListImageDefs(ctx context.Context, resourceGroup, gallery string) ([]GalleryImageRef, error) {
	pager := a.c.NewListByGalleryPager(resourceGroup, gallery, nil)
	var out []GalleryImageRef
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, img := range page.Value {
			if img == nil || img.Name == nil {
				continue
			}
			ref := GalleryImageRef{
				Name: *img.Name,
				Tags: ptrMapToStringMap(img.Tags),
			}
			if img.Properties != nil && img.Properties.OSType != nil {
				ref.OSType = string(*img.Properties.OSType)
			}
			out = append(out, ref)
		}
	}
	return out, nil
}

// resourcesAdapter wraps armresources.Client list-by-RG with a resourceType
// filter so we can find UAMIs (and any other ARM resource type) generically.
type resourcesAdapter struct {
	c *armresources.Client
}

func (a *resourcesAdapter) ListByResourceGroup(ctx context.Context, resourceGroup, resourceTypeFilter string) ([]ResourceRef, error) {
	opts := &armresources.ClientListByResourceGroupOptions{}
	if resourceTypeFilter != "" {
		filter := fmt.Sprintf("resourceType eq '%s'", resourceTypeFilter)
		opts.Filter = &filter
	}
	pager := a.c.NewListByResourceGroupPager(resourceGroup, opts)
	var out []ResourceRef
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, res := range page.Value {
			if res == nil || res.ID == nil || res.Name == nil {
				continue
			}
			out = append(out, ResourceRef{
				ID:   *res.ID,
				Name: *res.Name,
				Tags: ptrMapToStringMap(res.Tags),
			})
		}
	}
	return out, nil
}

// resourceGroupFromID extracts the resource group segment from a full ARM ID
// like "/subscriptions/<sub>/resourceGroups/<rg>/...". Returns empty string on
// malformed input — callers handle that as "couldn't fill RG".
func resourceGroupFromID(id string) string {
	parts := strings.Split(id, "/")
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], "resourceGroups") {
			return parts[i+1]
		}
	}
	return ""
}

// ptrMapToStringMap is the inverse of stringMapToPointerMap in template.go,
// for the Azure SDK's tag-map-of-pointers convention.
func ptrMapToStringMap(in map[string]*string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

// DiscoverWithDefaultCredential is a convenience wrapper that creates a
// DefaultAzureCredential, runs DiscoverTarget, and returns the report. Used
// from the build command path.
func DiscoverWithDefaultCredential(ctx context.Context, target *builder.Target, templateName, subscriptionID, tenantID string) (DiscoveryReport, error) {
	credOpts := &azidentity.DefaultAzureCredentialOptions{}
	if tenantID != "" {
		credOpts.TenantID = tenantID
	}
	cred, err := newCredential(credOpts)
	if err != nil {
		return DiscoveryReport{}, fmt.Errorf("azure discovery: credential: %w", err)
	}
	d, err := NewDiscoverer(cred, subscriptionID)
	if err != nil {
		return DiscoveryReport{}, err
	}
	return d.DiscoverTarget(ctx, target, templateName)
}
