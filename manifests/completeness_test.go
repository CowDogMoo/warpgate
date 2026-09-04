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

package manifests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiscoverDigestFilesRefusesUnparseableFile pins the completeness rule for
// the standalone `warpgate manifests create` path: an architecture whose digest
// file will not parse must stop the run, because skipping it publishes a
// manifest list that silently covers fewer architectures than were built.
func TestDiscoverDigestFilesRefusesUnparseableFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	write("digest-mealie-amd64.txt", "sha256:"+strings.Repeat("a", 64))
	write("digest-mealie-arm64.txt", "not-a-digest")

	files, err := DiscoverDigestFiles(context.Background(), DiscoveryOptions{
		ImageName: "mealie",
		Directory: dir,
	})
	if err == nil {
		t.Fatalf("DiscoverDigestFiles() returned %d file(s) and no error, want a refusal naming the file it could not read", len(files))
	}
	if files != nil {
		t.Errorf("DiscoverDigestFiles() returned %d file(s) alongside its error, want none", len(files))
	}
	for _, want := range []string{"digest-mealie-arm64.txt", "1 of 2", "--best-effort"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to mention %q", err, want)
		}
	}
}
