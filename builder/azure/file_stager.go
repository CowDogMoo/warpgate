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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// blobClientAPI is the subset of azblob.Client used by FileStager. Declaring
// the interface here (following the roleAssignmentsAPI pattern) lets tests
// inject a fake without standing up real Azure storage.
type blobClientAPI interface {
	UploadFile(ctx context.Context, containerName, blobName string, file *os.File, opts *azblob.UploadFileOptions) (azblob.UploadFileResponse, error)
	DeleteBlob(ctx context.Context, containerName, blobName string, opts *azblob.DeleteBlobOptions) (azblob.DeleteBlobResponse, error)
}

// StagedFile records the result of uploading a single `file` provisioner
// source to the staging container. For a single-file source it has one Entry;
// for a directory source it has one Entry per file under the source.
//
// Mirrors builder/ami/file_stager.go's StagedFile so the staging concept reads
// the same across cloud backends.
type StagedFile struct {
	Account     string
	Container   string
	IsDirectory bool
	// KeyPrefix is the blob-name prefix shared by every Entry (no trailing /).
	// Used for cleanup so we can delete the whole prefix on directory uploads.
	KeyPrefix string
	Entries   []StagedEntry
}

// StagedEntry is an individual uploaded blob.
type StagedEntry struct {
	// BlobName is the full blob name within the container, including KeyPrefix.
	BlobName string
	// RelPath is the path of the file relative to the original source.
	// Empty for single-file uploads (the destination is taken verbatim from
	// the provisioner). Always uses forward slashes.
	RelPath string
}

// URL returns the full https URL for the staged entry. Suitable for the AIB
// File customizer when the build identity (UAMI) has Storage Blob Data Reader
// on the staging account.
func (e StagedEntry) URL(account, container string) string {
	return fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", account, container, e.BlobName)
}

// FileStagerAPI is the subset of FileStager used by ImageBuilder. Tests can
// substitute fakes to avoid touching real Azure storage.
type FileStagerAPI interface {
	Stage(ctx context.Context, source, keyPrefix string) (*StagedFile, error)
	Cleanup(ctx context.Context, staged *StagedFile)
}

// FileStager uploads file provisioner sources to an Azure storage container so
// the AIB build VM can fetch them via the AIB File customizer. The credential
// principal must have permission to write to the staging container (typically
// Storage Blob Data Contributor) and the build's UAMI must have read access
// (typically Storage Blob Data Reader).
type FileStager struct {
	client    blobClientAPI
	account   string
	container string
}

// NewFileStager constructs a stager. Returns nil if any of client, account, or
// container are zero-valued so callers can detect the disabled state without
// an error path. Mirrors builder/ami/file_stager.go.NewFileStager.
func NewFileStager(client *azblob.Client, account, container string) *FileStager {
	if client == nil || account == "" || container == "" {
		return nil
	}
	return &FileStager{client: client, account: account, container: container}
}

// Stage uploads source (a local file or directory) to the staging container.
// All blobs share keyPrefix; for a directory each file is placed at
// keyPrefix/<rel-path-from-source-root>.
//
// Caller is responsible for keying prefixes uniquely (e.g., per build +
// provisioner index) so concurrent or repeated builds don't collide.
func (s *FileStager) Stage(ctx context.Context, source, keyPrefix string) (*StagedFile, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("stat file provisioner source %q: %w", source, err)
	}

	prefix := strings.TrimRight(keyPrefix, "/")

	if !info.IsDir() {
		blobName := prefix + "/" + filepath.Base(source)
		if err := s.uploadFile(ctx, source, blobName); err != nil {
			return nil, err
		}
		return &StagedFile{
			Account:     s.account,
			Container:   s.container,
			IsDirectory: false,
			KeyPrefix:   prefix,
			Entries:     []StagedEntry{{BlobName: blobName}},
		}, nil
	}

	entries, err := s.uploadDirectory(ctx, source, prefix)
	if err != nil {
		return nil, err
	}
	return &StagedFile{
		Account:     s.account,
		Container:   s.container,
		IsDirectory: true,
		KeyPrefix:   prefix,
		Entries:     entries,
	}, nil
}

// uploadFile uploads a single local file at path source to the named blob.
func (s *FileStager) uploadFile(ctx context.Context, source, blobName string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open file provisioner source %q: %w", source, err)
	}
	defer func() { _ = f.Close() }()

	logging.InfoContext(ctx, "Staging %q to https://%s.blob.core.windows.net/%s/%s", source, s.account, s.container, blobName)
	if _, err := s.client.UploadFile(ctx, s.container, blobName, f, nil); err != nil {
		return fmt.Errorf("upload %q to https://%s.blob.core.windows.net/%s/%s: %w", source, s.account, s.container, blobName, err)
	}
	return nil
}

// uploadDirectory walks root and uploads each regular file under keyPrefix
// preserving the relative path (with forward slashes).
func (s *FileStager) uploadDirectory(ctx context.Context, root, keyPrefix string) ([]StagedEntry, error) {
	logging.InfoContext(ctx, "Staging directory %q to https://%s.blob.core.windows.net/%s/%s/", root, s.account, s.container, keyPrefix)

	var entries []StagedEntry
	err := filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return fmt.Errorf("compute relative path for %q: %w", p, err)
		}
		rel = filepath.ToSlash(rel)
		blobName := keyPrefix + "/" + rel
		if err := s.uploadFile(ctx, p, blobName); err != nil {
			return err
		}
		entries = append(entries, StagedEntry{BlobName: blobName, RelPath: rel})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("upload directory %q: %w", root, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("file provisioner source %q is an empty directory", root)
	}
	return entries, nil
}

// Cleanup removes every blob recorded on staged. Errors are logged but not
// returned fatally — leaving stray blobs shouldn't block reporting build
// outcomes.
func (s *FileStager) Cleanup(ctx context.Context, staged *StagedFile) {
	if staged == nil {
		return
	}
	for _, e := range staged.Entries {
		if _, err := s.client.DeleteBlob(ctx, staged.Container, e.BlobName, nil); err != nil {
			logging.WarnContext(ctx, "Failed to delete staged blob https://%s.blob.core.windows.net/%s/%s: %v", staged.Account, staged.Container, e.BlobName, err)
		}
	}
}

// isStageableLocalPath returns true when source is a local filesystem path,
// i.e. not an http or https URL. Any other URL scheme is also treated as a
// local path so the stager can fail loudly rather than silently passing it
// through.
func isStageableLocalPath(source string) bool {
	if source == "" {
		return false
	}
	u, err := url.Parse(source)
	if err != nil {
		return true
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return false
	}
	return true
}

// destinationForEntry computes the on-VM destination path for a staged entry.
// For single-file uploads it returns the provisioner destination verbatim.
// For directory uploads it appends the entry's relative path so the directory
// structure is preserved on the build VM.
func destinationForEntry(provisionerDestination string, staged *StagedFile, entry StagedEntry) string {
	if !staged.IsDirectory || entry.RelPath == "" {
		return provisionerDestination
	}
	// path.Join collapses adjacent separators and forward-slash-normalizes.
	// AIB on Windows is generally forgiving of forward slashes in destinations.
	return path.Join(provisionerDestination, entry.RelPath)
}
