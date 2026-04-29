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
	"context"
	"fmt"
	"os"
	"path/filepath"

	bcptransfer "github.com/cowdogmoo/bcp/pkg/transfer"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// StagedFile records an upload performed by FileStager so it can be cleaned
// up after the build, and so the AMI component can reference the S3 source.
type StagedFile struct {
	Bucket      string
	KeyPrefix   string
	IsDirectory bool
	BaseName    string
}

// SourceURI returns the s3:// URI suitable for an Image Builder S3Download
// step. For directories it appends a wildcard so all objects under the
// prefix are pulled.
func (s *StagedFile) SourceURI() string {
	if s.IsDirectory {
		return fmt.Sprintf("s3://%s/%s%s/*", s.Bucket, s.KeyPrefix, s.BaseName)
	}
	return fmt.Sprintf("s3://%s/%s%s", s.Bucket, s.KeyPrefix, s.BaseName)
}

// FileStager uploads `file` provisioner sources to an S3 bucket so the AMI
// build instance can fetch them via S3Download. It delegates the upload and
// cleanup primitives to bcp.
type FileStager struct {
	Client S3API
	Bucket string
}

// NewFileStager constructs a stager. Returns nil if bucket is empty so callers
// can detect the disabled state.
func NewFileStager(client S3API, bucket string) *FileStager {
	if bucket == "" {
		return nil
	}
	return &FileStager{Client: client, Bucket: bucket}
}

// Stage uploads source to S3 under keyPrefix. Caller is responsible for
// keying prefixes uniquely (e.g., per build + provisioner index) so concurrent
// or repeated builds don't collide.
func (s *FileStager) Stage(ctx context.Context, source, keyPrefix string) (*StagedFile, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("stat file provisioner source %q: %w", source, err)
	}

	logging.InfoContext(ctx, "Staging %q to s3://%s/%s", source, s.Bucket, keyPrefix)

	if err := bcptransfer.UploadToS3WithPrefix(ctx, s.Client, s.Bucket, source, keyPrefix); err != nil {
		return nil, fmt.Errorf("upload %q to s3://%s/%s: %w", source, s.Bucket, keyPrefix, err)
	}

	prefix := keyPrefix
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	return &StagedFile{
		Bucket:      s.Bucket,
		KeyPrefix:   prefix,
		IsDirectory: info.IsDir(),
		BaseName:    filepath.Base(source),
	}, nil
}

// Cleanup removes the staged objects from S3. Errors are logged but not
// returned fatally — leaving stray objects shouldn't block reporting build
// outcomes.
func (s *FileStager) Cleanup(ctx context.Context, staged *StagedFile) {
	if staged == nil {
		return
	}
	key := staged.KeyPrefix + staged.BaseName
	err := bcptransfer.CleanupS3Objects(ctx, s.Client, staged.Bucket, key, staged.IsDirectory)
	if err != nil {
		logging.WarnContext(ctx, "Failed to clean up staged S3 objects at s3://%s/%s: %v", staged.Bucket, key, err)
	}
}
