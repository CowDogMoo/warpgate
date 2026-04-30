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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
