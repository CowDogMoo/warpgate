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
	"sync"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStagedEntry_URL(t *testing.T) {
	e := StagedEntry{BlobName: "warpgate-staging/build-1/0/install.sh"}
	assert.Equal(t, "https://acct.blob.core.windows.net/ctr/warpgate-staging/build-1/0/install.sh", e.URL("acct", "ctr"))
}

func TestIsStageableLocalPath(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"https://example.com/x.sh", false},
		{"http://example.com/x.sh", false},
		{"HTTPS://EXAMPLE.com/x.sh", false},
		{"/abs/path/x.sh", true},
		{"./rel/path/x.sh", true},
		{"x.sh", true},
		{"file:///etc/x.sh", true},
		{"ftp://example.com/x", true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, isStageableLocalPath(tt.in), "isStageableLocalPath(%q)", tt.in)
	}
}

func TestDestinationForEntry(t *testing.T) {
	t.Run("single file passes destination through", func(t *testing.T) {
		s := &StagedFile{IsDirectory: false}
		got := destinationForEntry("/opt/app/script.sh", s, StagedEntry{RelPath: ""})
		assert.Equal(t, "/opt/app/script.sh", got)
	})
	t.Run("directory entry appends rel path", func(t *testing.T) {
		s := &StagedFile{IsDirectory: true}
		got := destinationForEntry("/opt/app", s, StagedEntry{RelPath: "lib/util.sh"})
		assert.Equal(t, "/opt/app/lib/util.sh", got)
	})
}

func TestNewFileStager_DisabledWhenAnyArgEmpty(t *testing.T) {
	// Real *azblob.Client would require Azure credentials; nil exercises the
	// disabled path which is the only thing this constructor decides.
	tests := []struct {
		account, container string
	}{
		{"", ""},
		{"acct", ""},
		{"", "ctr"},
	}
	for _, tt := range tests {
		stager := NewFileStager(nil, tt.account, tt.container)
		assert.Nil(t, stager, "expected nil stager for account=%q container=%q", tt.account, tt.container)
	}
}

// fakeStager records every call so tests can assert behavior without touching
// real Azure storage. Implements FileStagerAPI.
type fakeStager struct {
	mu        sync.Mutex
	uploads   []fakeUpload
	deletes   []*StagedFile
	stageErr  error
	urlPrefix string
}

type fakeUpload struct {
	source    string
	keyPrefix string
}

func newFakeStager() *fakeStager {
	return &fakeStager{urlPrefix: "https://fake.blob.core.windows.net/c/"}
}

func (f *fakeStager) Stage(_ context.Context, source, keyPrefix string) (*StagedFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stageErr != nil {
		return nil, f.stageErr
	}
	f.uploads = append(f.uploads, fakeUpload{source: source, keyPrefix: keyPrefix})
	return &StagedFile{
		Account:   "fake",
		Container: "c",
		KeyPrefix: keyPrefix,
		Entries:   []StagedEntry{{BlobName: keyPrefix + "/blob"}},
	}, nil
}

func (f *fakeStager) Cleanup(_ context.Context, staged *StagedFile) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes = append(f.deletes, staged)
}

func TestImageBuilder_StageFileProvisioners_NoFileProvisioners(t *testing.T) {
	b := &ImageBuilder{buildID: "build-1", fileStager: newFakeStager()}
	cfg := buildAzureCfg()
	cfg.Provisioners = []builder.Provisioner{
		{Type: "shell", Inline: []string{"echo hi"}},
		{Type: "powershell", Inline: []string{"Get-Service"}},
	}

	staged, err := b.stageFileProvisioners(context.Background(), cfg)
	require.NoError(t, err)
	assert.Empty(t, staged)
}

func TestImageBuilder_StageFileProvisioners_PassesThroughURLs(t *testing.T) {
	stager := newFakeStager()
	b := &ImageBuilder{buildID: "build-1", fileStager: stager}
	cfg := buildAzureCfg()
	cfg.Provisioners = []builder.Provisioner{
		{Type: "file", Source: "https://example.com/setup.sh", Destination: "/tmp/setup.sh"},
	}

	staged, err := b.stageFileProvisioners(context.Background(), cfg)
	require.NoError(t, err)
	assert.Empty(t, staged, "remote URLs must not produce staged entries")
	assert.Empty(t, stager.uploads)
}

func TestImageBuilder_StageFileProvisioners_StagesLocalPaths(t *testing.T) {
	stager := newFakeStager()
	b := &ImageBuilder{buildID: "build-1", fileStager: stager}
	cfg := buildAzureCfg()
	cfg.Provisioners = []builder.Provisioner{
		{Type: "shell", Inline: []string{"echo"}},
		{Type: "file", Source: "scripts/install.sh", Destination: "/tmp/install.sh"},
		{Type: "file", Source: "https://example.com/x.sh", Destination: "/tmp/x.sh"},
		{Type: "file", Source: "/abs/path/data.tar", Destination: "/tmp/data.tar"},
	}

	staged, err := b.stageFileProvisioners(context.Background(), cfg)
	require.NoError(t, err)
	require.Len(t, staged, 2, "two local-path provisioners should produce two entries")
	require.Len(t, stager.uploads, 2)

	assert.Equal(t, "scripts/install.sh", stager.uploads[0].source)
	assert.Equal(t, "warpgate-staging/build-1/1", stager.uploads[0].keyPrefix)
	assert.Equal(t, "/abs/path/data.tar", stager.uploads[1].source)
	assert.Equal(t, "warpgate-staging/build-1/3", stager.uploads[1].keyPrefix)

	// Map keys mirror provisioner indices.
	assert.NotNil(t, staged[1])
	assert.NotNil(t, staged[3])
	assert.Nil(t, staged[0], "shell provisioner must not be staged")
	assert.Nil(t, staged[2], "remote URL must not be staged")
}

func TestImageBuilder_StageFileProvisioners_DoesNotMutateInput(t *testing.T) {
	in := []builder.Provisioner{
		{Type: "file", Source: "scripts/install.sh", Destination: "/tmp/install.sh"},
	}
	original := in[0].Source
	b := &ImageBuilder{buildID: "build-1", fileStager: newFakeStager()}
	cfg := buildAzureCfg()
	cfg.Provisioners = in

	_, err := b.stageFileProvisioners(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, original, in[0].Source, "input slice must not be mutated")
}

func TestImageBuilder_StageFileProvisioners_ErrorsWhenStagerMissing(t *testing.T) {
	b := &ImageBuilder{buildID: "build-1", fileStager: nil}
	cfg := buildAzureCfg()
	cfg.Provisioners = []builder.Provisioner{
		{Type: "file", Source: "scripts/install.sh", Destination: "/tmp/install.sh"},
	}

	_, err := b.stageFileProvisioners(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file_staging_storage_account")
}

func TestImageBuilder_StageFileProvisioners_StagerNilTolerantWhenAllRemote(t *testing.T) {
	b := &ImageBuilder{buildID: "build-1", fileStager: nil}
	cfg := buildAzureCfg()
	cfg.Provisioners = []builder.Provisioner{
		{Type: "file", Source: "https://example.com/x.sh", Destination: "/tmp/x.sh"},
	}

	staged, err := b.stageFileProvisioners(context.Background(), cfg)
	require.NoError(t, err)
	assert.Empty(t, staged)
}

func TestImageBuilder_CleanupStagedFiles(t *testing.T) {
	stager := newFakeStager()
	b := &ImageBuilder{fileStager: stager}
	staged := map[int]*StagedFile{
		0: {Account: "a", Container: "c", KeyPrefix: "p0", Entries: []StagedEntry{{BlobName: "p0/x"}}},
		2: {Account: "a", Container: "c", KeyPrefix: "p2", Entries: []StagedEntry{{BlobName: "p2/y"}}},
	}
	b.cleanupStagedFiles(context.Background(), staged)
	assert.Len(t, stager.deletes, 2)
}

func TestImageBuilder_CleanupStagedFiles_NilStager(t *testing.T) {
	b := &ImageBuilder{fileStager: nil}
	// Should not panic.
	b.cleanupStagedFiles(context.Background(), map[int]*StagedFile{0: {}})
}
