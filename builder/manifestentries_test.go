/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

package builder

import (
	"context"
	"strings"
	"testing"
)

// TestCreateManifestEntriesRefusesUnparseableDigest pins the failure mode the
// function needs to have: a result it cannot describe drops an architecture out
// of the manifest list, so it has to be reported rather than quietly skipped.
func TestCreateManifestEntriesRefusesUnparseableDigest(t *testing.T) {
	t.Parallel()

	results := []BuildResult{
		{
			ImageRef:     "ghcr.io/cowdogmoo/mealie:amd64",
			Architecture: "amd64",
			Platform:     "linux/amd64",
			Digest:       "sha256:" + strings.Repeat("a", 64),
		},
		{
			ImageRef:     "ghcr.io/cowdogmoo/mealie:arm64",
			Architecture: "arm64",
			Platform:     "linux/arm64",
			Digest:       "not-a-digest",
		},
	}

	entries, err := CreateManifestEntries(context.Background(), results)
	if err == nil {
		t.Fatalf("CreateManifestEntries() returned %d entries and no error, want an error naming the architecture it could not describe", len(entries))
	}
	if entries != nil {
		t.Errorf("CreateManifestEntries() returned %d entries alongside its error, want none", len(entries))
	}
	if !strings.Contains(err.Error(), "arm64") {
		t.Errorf("error = %v, want it to name the architecture that could not be described", err)
	}
	if !strings.Contains(err.Error(), "1 of 2") {
		t.Errorf("error = %v, want it to report how many architectures were describable", err)
	}
}

// TestCreateManifestEntriesNamesResultWithoutArchitecture checks the report stays
// useful when a push came back with nothing but a platform, which is the shape a
// registry failure leaves behind.
func TestCreateManifestEntriesNamesResultWithoutArchitecture(t *testing.T) {
	t.Parallel()

	results := []BuildResult{
		{ImageRef: "ghcr.io/cowdogmoo/mealie:arm64", Platform: "linux/arm64", Digest: "not-a-digest"},
		{ImageRef: "ghcr.io/cowdogmoo/mealie:amd64", Digest: "not-a-digest"},
		{Digest: "not-a-digest"},
	}

	_, err := CreateManifestEntries(context.Background(), results)
	if err == nil {
		t.Fatal("CreateManifestEntries() returned no error for three undescribable results")
	}
	for _, want := range []string{"linux/arm64", "ghcr.io/cowdogmoo/mealie:amd64", "unknown architecture", "0 of 3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to mention %q", err, want)
		}
	}
}
