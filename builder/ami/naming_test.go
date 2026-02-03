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

package ami

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeAMIName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		amiName     string
		defaultName string
		want        string
	}{
		{
			name:        "empty name uses default with timestamp",
			amiName:     "",
			defaultName: "my-ami",
			want:        "my-ami-{{ imagebuilder:buildDate }}",
		},
		{
			name:        "replaces {{timestamp}}",
			amiName:     "my-ami-{{timestamp}}",
			defaultName: "default",
			want:        "my-ami-{{ imagebuilder:buildDate }}",
		},
		{
			name:        "replaces {{ timestamp }}",
			amiName:     "my-ami-{{ timestamp }}",
			defaultName: "default",
			want:        "my-ami-{{ imagebuilder:buildDate }}",
		},
		{
			name:        "replaces {{imagebuilder:buildDate}} without spaces",
			amiName:     "my-ami-{{imagebuilder:buildDate}}",
			defaultName: "default",
			want:        "my-ami-{{ imagebuilder:buildDate }}",
		},
		{
			name:        "already correct format unchanged",
			amiName:     "my-ami-{{ imagebuilder:buildDate }}",
			defaultName: "default",
			want:        "my-ami-{{ imagebuilder:buildDate }}",
		},
		{
			name:        "no placeholder appends timestamp",
			amiName:     "my-ami",
			defaultName: "default",
			want:        "my-ami-{{ imagebuilder:buildDate }}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeAMIName(tc.amiName, tc.defaultName)
			assert.Equal(t, tc.want, got)
		})
	}
}
