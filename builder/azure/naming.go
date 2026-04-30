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
	"fmt"
	"time"
)

// galleryImageVersionID returns the full resource ID of a gallery image
// version given its components. AIB's SharedImageDistributor expects the
// gallery image (definition) ID, and the produced gallery image version
// inherits that path with a /versions/<v> suffix.
func galleryImageDefinitionID(subscriptionID, resourceGroup, gallery, definition string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s/images/%s",
		subscriptionID, resourceGroup, gallery, definition)
}

// imageTemplateName returns a stable image-template resource name for a build.
// AIB image template names must be 1-64 chars, alphanumeric, dashes, periods,
// and underscores. Names should be unique per build; we use the warpgate config
// name plus the build timestamp.
func imageTemplateName(configName, buildTimestamp string) string {
	return fmt.Sprintf("warpgate-%s-%s", sanitize(configName), buildTimestamp)
}

// nextGalleryVersion returns a semver-style gallery image version using the
// current UTC time. Format: YYYY.MMDD.HHMMSS so it sorts correctly and stays
// within Azure's gallery version constraints (each component <= 32 bits).
func nextGalleryVersion(now time.Time) string {
	t := now.UTC()
	return fmt.Sprintf("%d.%02d%02d.%02d%02d%02d",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// sanitize lowercases and replaces invalid characters with dashes for use in
// Azure resource names.
func sanitize(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '.', c == '_':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c+('a'-'A'))
		default:
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "image"
	}
	return string(out)
}
