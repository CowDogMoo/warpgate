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
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNextGalleryVersion(t *testing.T) {
	// Fixed UTC time so the assertion is deterministic.
	now := time.Date(2026, 4, 29, 13, 5, 9, 0, time.UTC)
	got := nextGalleryVersion(now)
	assert.Equal(t, "2026.0429.130509", got)
}

func TestNextGalleryVersionTimezoneIndependent(t *testing.T) {
	// Same instant in two zones produces the same UTC-based version.
	loc, err := time.LoadLocation("America/New_York")
	assert.NoError(t, err)
	utc := time.Date(2026, 4, 29, 13, 5, 9, 0, time.UTC)
	ny := utc.In(loc)
	assert.Equal(t, nextGalleryVersion(utc), nextGalleryVersion(ny))
}

func TestImageTemplateName(t *testing.T) {
	got := imageTemplateName("Attack-Box", "20260429130509")
	assert.Equal(t, "warpgate-attack-box-20260429130509", got)
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"abc", "abc"},
		{"AbC", "abc"},
		{"my image!", "my-image-"},
		{"keep_underscores-and.dots", "keep_underscores-and.dots"},
		{"", "image"},
		{"123abc", "123abc"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, sanitize(tt.in), "sanitize(%q)", tt.in)
	}
}

func TestGalleryImageDefinitionID(t *testing.T) {
	got := galleryImageDefinitionID("sub-1", "rg-1", "myGallery", "myDef")
	want := "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/myGallery/images/myDef"
	assert.Equal(t, want, got)
}
