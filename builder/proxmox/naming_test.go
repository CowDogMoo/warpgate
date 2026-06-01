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

package proxmox

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeVMName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		input       string
		defaultName string
		want        string
	}{
		{"valid pass-through", "kali-build", "fallback", "kali-build"},
		{"underscores become dashes", "kali_build_v2", "fallback", "kali-build-v2"},
		{"spaces become dashes", "kali build", "fallback", "kali-build"},
		{"empty uses default", "", "fallback", "fallback"},
		{"empty default uses constant", "", "", "warpgate-build"},
		{"strips leading/trailing dashes", "---kali---", "fallback", "kali"},
		{
			name:        "truncates to 63 chars and trims trailing dash",
			input:       strings.Repeat("a", 70),
			defaultName: "fallback",
			want:        strings.Repeat("a", 63),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeVMName(tt.input, tt.defaultName)
			if got != tt.want {
				t.Fatalf("normalizeVMName(%q, %q) = %q, want %q", tt.input, tt.defaultName, got, tt.want)
			}
		})
	}
}

func TestFmtBuildStamp(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 6, 1, 12, 30, 45, 0, time.UTC)
	want := "20260601123045"
	if got := fmtBuildStamp(ts); got != want {
		t.Fatalf("fmtBuildStamp = %q, want %q", got, want)
	}
}

func TestBuildResourceName(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 6, 1, 12, 30, 45, 0, time.UTC)
	if got, want := buildResourceName("kali", ts), "kali-20260601123045"; got != want {
		t.Fatalf("buildResourceName(kali) = %q, want %q", got, want)
	}
	// Special chars in base get normalized.
	if got := buildResourceName("kali rolling!", ts); !strings.HasPrefix(got, "kali-rolling-") {
		t.Fatalf("buildResourceName did not normalize special chars: %q", got)
	}
	// Empty base falls back to warpgate-build prefix.
	if got, want := buildResourceName("", ts), "warpgate-build-20260601123045"; got != want {
		t.Fatalf("buildResourceName(\"\") = %q, want %q", got, want)
	}
}
