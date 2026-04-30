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
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

// galleryVersionRef captures the components of a gallery image version
// resource ID, e.g.:
//
//	/subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Compute/
//	galleries/<gallery>/images/<def>/versions/<ver>
type galleryVersionRef struct {
	SubscriptionID string
	ResourceGroup  string
	Gallery        string
	Definition     string
	Version        string
}

// parseGalleryVersionID parses a gallery image version resource ID. Returns an
// error if the ID does not match the expected ARM path layout.
func parseGalleryVersionID(id string) (galleryVersionRef, error) {
	parts := strings.Split(strings.TrimPrefix(id, "/"), "/")
	// Expected length 12:
	// subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Compute/
	// galleries/<g>/images/<i>/versions/<v>
	if len(parts) != 12 {
		return galleryVersionRef{}, fmt.Errorf("not a gallery image version ID: %s", id)
	}
	expect := func(idx int, want string) error {
		if !strings.EqualFold(parts[idx], want) {
			return fmt.Errorf("malformed gallery image version ID: expected %q at position %d, got %q", want, idx, parts[idx])
		}
		return nil
	}
	for i, want := range map[int]string{
		0:  "subscriptions",
		2:  "resourceGroups",
		4:  "providers",
		5:  "Microsoft.Compute",
		6:  "galleries",
		8:  "images",
		10: "versions",
	} {
		if err := expect(i, want); err != nil {
			return galleryVersionRef{}, err
		}
	}
	return galleryVersionRef{
		SubscriptionID: parts[1],
		ResourceGroup:  parts[3],
		Gallery:        parts[7],
		Definition:     parts[9],
		Version:        parts[11],
	}, nil
}

// updateGalleryVersionRegions extends the version's publishing profile target
// regions to include newRegions. Existing regions are preserved.
func updateGalleryVersionRegions(ctx context.Context, clients *AzureClients, versionID string, newRegions []string) error {
	ref, err := parseGalleryVersionID(versionID)
	if err != nil {
		return err
	}

	cur, err := clients.GalleryImageVersions.Get(ctx, ref.ResourceGroup, ref.Gallery, ref.Definition, ref.Version, nil)
	if err != nil {
		return fmt.Errorf("get gallery image version %s: %w", versionID, err)
	}

	merged := mergeTargetRegions(cur.Properties, newRegions)
	update := armcompute.GalleryImageVersionUpdate{
		Properties: &armcompute.GalleryImageVersionProperties{
			PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{
				TargetRegions: merged,
			},
			StorageProfile: &armcompute.GalleryImageVersionStorageProfile{},
		},
	}
	if cur.Properties != nil && cur.Properties.StorageProfile != nil {
		update.Properties.StorageProfile = cur.Properties.StorageProfile
	}

	poller, err := clients.GalleryImageVersions.BeginUpdate(ctx, ref.ResourceGroup, ref.Gallery, ref.Definition, ref.Version, update, nil)
	if err != nil {
		return fmt.Errorf("update gallery image version %s: %w", versionID, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("wait for gallery image version update %s: %w", versionID, err)
	}
	return nil
}

// mergeTargetRegions returns the union of the current publishing profile's
// target regions and newRegions. Existing entries are preserved verbatim;
// duplicates (case-insensitive name match) are dropped from newRegions.
func mergeTargetRegions(props *armcompute.GalleryImageVersionProperties, newRegions []string) []*armcompute.TargetRegion {
	existing := map[string]struct{}{}
	merged := []*armcompute.TargetRegion{}
	if props != nil && props.PublishingProfile != nil {
		for _, r := range props.PublishingProfile.TargetRegions {
			if r != nil && r.Name != nil {
				existing[strings.ToLower(*r.Name)] = struct{}{}
			}
		}
		merged = append(merged, props.PublishingProfile.TargetRegions...)
	}
	for _, r := range newRegions {
		if _, dup := existing[strings.ToLower(r)]; dup {
			continue
		}
		r := r
		merged = append(merged, &armcompute.TargetRegion{Name: &r})
	}
	return merged
}

// deleteGalleryVersion removes the gallery image version identified by versionID.
func deleteGalleryVersion(ctx context.Context, clients *AzureClients, versionID string) error {
	ref, err := parseGalleryVersionID(versionID)
	if err != nil {
		return err
	}
	poller, err := clients.GalleryImageVersions.BeginDelete(ctx, ref.ResourceGroup, ref.Gallery, ref.Definition, ref.Version, nil)
	if err != nil {
		return fmt.Errorf("delete gallery image version %s: %w", versionID, err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("wait for gallery image version delete %s: %w", versionID, err)
	}
	return nil
}
