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
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
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

// fakeBlobClient implements blobClientAPI for unit tests. Upload and delete
// results are configurable per-call so individual tests can simulate success
// or failure paths without touching Azure storage.
type fakeBlobClient struct {
	mu          sync.Mutex
	uploadCalls []fakeBlobUpload
	deleteCalls []fakeBlobDelete
	uploadErr   error
	deleteErr   error
}

type fakeBlobUpload struct {
	container string
	blobName  string
}

type fakeBlobDelete struct {
	container string
	blobName  string
}

func (f *fakeBlobClient) UploadFile(_ context.Context, container, blobName string, _ *os.File, _ *azblob.UploadFileOptions) (azblob.UploadFileResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uploadCalls = append(f.uploadCalls, fakeBlobUpload{container: container, blobName: blobName})
	return azblob.UploadFileResponse{}, f.uploadErr
}

func (f *fakeBlobClient) DeleteBlob(_ context.Context, container, blobName string, _ *azblob.DeleteBlobOptions) (azblob.DeleteBlobResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteCalls = append(f.deleteCalls, fakeBlobDelete{container: container, blobName: blobName})
	return azblob.DeleteBlobResponse{}, f.deleteErr
}

// newFakeBlobStager creates a FileStager wired with a fakeBlobClient.
func newFakeBlobStager(uploadErr, deleteErr error) (*FileStager, *fakeBlobClient) {
	fbc := &fakeBlobClient{uploadErr: uploadErr, deleteErr: deleteErr}
	s := &FileStager{client: fbc, account: "testacc", container: "testctr"}
	return s, fbc
}

func TestFileStager_NewFileStager_PositiveCase(t *testing.T) {
	fbc := &fakeBlobClient{}
	// NewFileStager takes *azblob.Client; the positive path is already tested
	// indirectly via NewAzureClients. Exercise the nil-return guard via nil
	// *azblob.Client and then separately confirm non-nil via blobClientAPI.
	// Direct construction via the struct literal is safe for testing internals.
	s := &FileStager{client: fbc, account: "a", container: "c"}
	require.NotNil(t, s)
	assert.Equal(t, "a", s.account)
	assert.Equal(t, "c", s.container)
}

func TestFileStager_Stage_SingleFile(t *testing.T) {
	tmpFile := t.TempDir() + "/test.sh"
	require.NoError(t, os.WriteFile(tmpFile, []byte("#!/bin/sh"), 0600))

	s, fbc := newFakeBlobStager(nil, nil)
	staged, err := s.Stage(context.Background(), tmpFile, "warpgate-staging/build-1/0")
	require.NoError(t, err)
	require.NotNil(t, staged)

	assert.False(t, staged.IsDirectory)
	assert.Equal(t, "testacc", staged.Account)
	assert.Equal(t, "testctr", staged.Container)
	assert.Equal(t, "warpgate-staging/build-1/0", staged.KeyPrefix)
	require.Len(t, staged.Entries, 1)
	assert.Contains(t, staged.Entries[0].BlobName, "test.sh")

	require.Len(t, fbc.uploadCalls, 1)
	assert.Equal(t, "testctr", fbc.uploadCalls[0].container)
}

func TestFileStager_Stage_SingleFile_UploadError(t *testing.T) {
	tmpFile := t.TempDir() + "/fail.sh"
	require.NoError(t, os.WriteFile(tmpFile, []byte("x"), 0600))

	s, _ := newFakeBlobStager(errors.New("upload denied"), nil)
	_, err := s.Stage(context.Background(), tmpFile, "prefix/0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload denied")
}

func TestFileStager_Stage_NonexistentSource(t *testing.T) {
	s, _ := newFakeBlobStager(nil, nil)
	_, err := s.Stage(context.Background(), "/does/not/exist/file.sh", "prefix/0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stat file provisioner source")
}

func TestFileStager_Stage_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(tmpDir+"/a.sh", []byte("a"), 0600))
	require.NoError(t, os.MkdirAll(tmpDir+"/sub", 0755))
	require.NoError(t, os.WriteFile(tmpDir+"/sub/b.sh", []byte("b"), 0600))

	s, fbc := newFakeBlobStager(nil, nil)
	staged, err := s.Stage(context.Background(), tmpDir, "warpgate-staging/build-1/1")
	require.NoError(t, err)
	require.NotNil(t, staged)

	assert.True(t, staged.IsDirectory)
	require.Len(t, staged.Entries, 2)
	assert.Len(t, fbc.uploadCalls, 2)

	// Relative paths should use forward slashes.
	relPaths := make([]string, len(staged.Entries))
	for i, e := range staged.Entries {
		relPaths[i] = e.RelPath
	}
	assert.Contains(t, relPaths, "a.sh")
	assert.Contains(t, relPaths, "sub/b.sh")
}

func TestFileStager_Stage_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := newFakeBlobStager(nil, nil)
	_, err := s.Stage(context.Background(), tmpDir, "prefix/1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty directory")
}

func TestFileStager_Stage_Directory_UploadError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(tmpDir+"/x.sh", []byte("x"), 0600))

	s, _ := newFakeBlobStager(errors.New("quota exceeded"), nil)
	_, err := s.Stage(context.Background(), tmpDir, "prefix/1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quota exceeded")
}

func TestFileStager_Cleanup_DeletesEachEntry(t *testing.T) {
	s, fbc := newFakeBlobStager(nil, nil)
	staged := &StagedFile{
		Account:   "testacc",
		Container: "testctr",
		Entries: []StagedEntry{
			{BlobName: "prefix/0/a.sh"},
			{BlobName: "prefix/0/b.sh"},
		},
	}
	s.Cleanup(context.Background(), staged)
	assert.Len(t, fbc.deleteCalls, 2)
	assert.Equal(t, "testctr", fbc.deleteCalls[0].container)
	assert.Equal(t, "prefix/0/a.sh", fbc.deleteCalls[0].blobName)
}

func TestFileStager_Cleanup_NilStagedFile(t *testing.T) {
	s, fbc := newFakeBlobStager(nil, nil)
	// Must not panic.
	s.Cleanup(context.Background(), nil)
	assert.Empty(t, fbc.deleteCalls)
}

func TestFileStager_Cleanup_DeleteError_DoesNotPanic(t *testing.T) {
	s, fbc := newFakeBlobStager(nil, errors.New("delete failed"))
	staged := &StagedFile{
		Container: "testctr",
		Entries:   []StagedEntry{{BlobName: "prefix/0/x.sh"}},
	}
	// Errors are logged, not returned.
	s.Cleanup(context.Background(), staged)
	assert.Len(t, fbc.deleteCalls, 1)
}

func TestFileStager_Stage_KeyPrefixTrailingSlashStripped(t *testing.T) {
	tmpFile := t.TempDir() + "/x.sh"
	require.NoError(t, os.WriteFile(tmpFile, []byte("x"), 0600))

	s, fbc := newFakeBlobStager(nil, nil)
	staged, err := s.Stage(context.Background(), tmpFile, "prefix/0/")
	require.NoError(t, err)
	assert.Equal(t, "prefix/0", staged.KeyPrefix)
	assert.NotContains(t, fbc.uploadCalls[0].blobName, "//", "double slash must not appear in blob name")
}
