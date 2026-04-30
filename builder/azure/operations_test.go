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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func TestMergeTargetRegions_NilProps(t *testing.T) {
	merged := mergeTargetRegions(nil, []string{"eastus", "westus"})
	require.Len(t, merged, 2)
	assert.Equal(t, "eastus", *merged[0].Name)
	assert.Equal(t, "westus", *merged[1].Name)
}

func TestMergeTargetRegions_NilPublishingProfile(t *testing.T) {
	props := &armcompute.GalleryImageVersionProperties{}
	merged := mergeTargetRegions(props, []string{"centralus"})
	require.Len(t, merged, 1)
	assert.Equal(t, "centralus", *merged[0].Name)
}

func TestMergeTargetRegions_PreservesExistingRegions(t *testing.T) {
	existing := &armcompute.GalleryImageVersionProperties{
		PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{
			TargetRegions: []*armcompute.TargetRegion{
				{Name: ptr("eastus")},
				{Name: ptr("westus")},
			},
		},
	}
	merged := mergeTargetRegions(existing, []string{"centralus"})
	require.Len(t, merged, 3)
	assert.Equal(t, "eastus", *merged[0].Name)
	assert.Equal(t, "westus", *merged[1].Name)
	assert.Equal(t, "centralus", *merged[2].Name)
}

func TestMergeTargetRegions_DeduplicatesCaseInsensitive(t *testing.T) {
	existing := &armcompute.GalleryImageVersionProperties{
		PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{
			TargetRegions: []*armcompute.TargetRegion{
				{Name: ptr("EastUS")},
			},
		},
	}
	merged := mergeTargetRegions(existing, []string{"eastus", "EASTUS", "WestUS"})
	require.Len(t, merged, 2, "eastus duplicate must be dropped, westus added")
	assert.Equal(t, "EastUS", *merged[0].Name)
	assert.Equal(t, "WestUS", *merged[1].Name)
}

func TestMergeTargetRegions_NoNewRegions(t *testing.T) {
	existing := &armcompute.GalleryImageVersionProperties{
		PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{
			TargetRegions: []*armcompute.TargetRegion{
				{Name: ptr("eastus")},
			},
		},
	}
	merged := mergeTargetRegions(existing, nil)
	require.Len(t, merged, 1)
	assert.Equal(t, "eastus", *merged[0].Name)
}

func TestMergeTargetRegions_EmptyNewRegions(t *testing.T) {
	merged := mergeTargetRegions(nil, []string{})
	assert.Empty(t, merged)
}

func TestMergeTargetRegions_NilEntryInExisting(t *testing.T) {
	existing := &armcompute.GalleryImageVersionProperties{
		PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{
			TargetRegions: []*armcompute.TargetRegion{
				nil,
				{Name: ptr("eastus")},
			},
		},
	}
	merged := mergeTargetRegions(existing, []string{"westus"})
	require.Len(t, merged, 3, "nil entry preserved in slice, westus appended")
	assert.Nil(t, merged[0])
	assert.Equal(t, "eastus", *merged[1].Name)
	assert.Equal(t, "westus", *merged[2].Name)
}

func TestParseGalleryVersionID_Valid(t *testing.T) {
	id := "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/i/versions/1.2.3"
	ref, err := parseGalleryVersionID(id)
	require.NoError(t, err)
	assert.Equal(t, "sub-1", ref.SubscriptionID)
	assert.Equal(t, "rg-1", ref.ResourceGroup)
	assert.Equal(t, "g", ref.Gallery)
	assert.Equal(t, "i", ref.Definition)
	assert.Equal(t, "1.2.3", ref.Version)
}

func TestParseGalleryVersionID_CaseInsensitiveSegments(t *testing.T) {
	id := "/SUBSCRIPTIONS/sub-1/RESOURCEGROUPS/rg-1/PROVIDERS/microsoft.compute/GALLERIES/g/IMAGES/i/VERSIONS/1.2.3"
	ref, err := parseGalleryVersionID(id)
	require.NoError(t, err)
	assert.Equal(t, "sub-1", ref.SubscriptionID)
	assert.Equal(t, "1.2.3", ref.Version)
}

func TestParseGalleryVersionID_Errors(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"too short", "/subscriptions/sub-1/resourceGroups/rg-1"},
		{"wrong segment", "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/i/snapshots/1.2.3"},
		{"wrong provider", "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Storage/galleries/g/images/i/versions/1.2.3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseGalleryVersionID(tc.id)
			assert.Error(t, err)
		})
	}
}
