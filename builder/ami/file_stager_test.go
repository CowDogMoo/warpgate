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
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStagedFile_SourceURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		staged StagedFile
		want   string
	}{
		{
			name:   "single file",
			staged: StagedFile{Bucket: "b", KeyPrefix: "p/", BaseName: "f.tar"},
			want:   "s3://b/p/f.tar",
		},
		{
			name:   "directory uses wildcard",
			staged: StagedFile{Bucket: "b", KeyPrefix: "p/", IsDirectory: true, BaseName: "configs"},
			want:   "s3://b/p/configs/*",
		},
		{
			name:   "empty prefix",
			staged: StagedFile{Bucket: "b", BaseName: "f"},
			want:   "s3://b/f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.staged.SourceURI())
		})
	}
}

func TestNewFileStager(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when bucket empty", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, NewFileStager(&MockS3Client{}, ""))
	})

	t.Run("returns stager when bucket set", func(t *testing.T) {
		t.Parallel()
		client := &MockS3Client{}
		s := NewFileStager(client, "my-bucket")
		require.NotNil(t, s)
		assert.Equal(t, "my-bucket", s.Bucket)
		assert.Equal(t, S3API(client), s.Client)
	})
}

func TestFileStager_Stage_File(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "payload.txt")
	require.NoError(t, os.WriteFile(src, []byte("hello"), 0o600))

	var mu sync.Mutex
	var puts []string
	mock := &MockS3Client{
		PutObjectFunc: func(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			mu.Lock()
			defer mu.Unlock()
			puts = append(puts, *params.Key)
			return &s3.PutObjectOutput{}, nil
		},
	}

	stager := NewFileStager(mock, "bkt")
	require.NotNil(t, stager)

	staged, err := stager.Stage(context.Background(), src, "warpgate-staging/build/0")
	require.NoError(t, err)
	assert.Equal(t, "bkt", staged.Bucket)
	assert.Equal(t, "warpgate-staging/build/0/", staged.KeyPrefix)
	assert.False(t, staged.IsDirectory)
	assert.Equal(t, "payload.txt", staged.BaseName)
	assert.Equal(t, "s3://bkt/warpgate-staging/build/0/payload.txt", staged.SourceURI())
	assert.Equal(t, []string{"warpgate-staging/build/0/payload.txt"}, puts)
}

func TestFileStager_Stage_Directory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "configs")
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "nested"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a.cfg"), []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "nested", "b.cfg"), []byte("b"), 0o600))

	mock := &MockS3Client{}
	stager := NewFileStager(mock, "bkt")
	staged, err := stager.Stage(context.Background(), srcDir, "warpgate-staging/build/1/")
	require.NoError(t, err)
	assert.True(t, staged.IsDirectory)
	assert.Equal(t, "configs", staged.BaseName)
	assert.Equal(t, "s3://bkt/warpgate-staging/build/1/configs/*", staged.SourceURI())
}

func TestFileStager_Stage_StatError(t *testing.T) {
	t.Parallel()

	stager := NewFileStager(&MockS3Client{}, "bkt")
	_, err := stager.Stage(context.Background(), "/nonexistent/path/that/does-not/exist", "p")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stat file provisioner source")
}

func TestFileStager_Stage_UploadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "f.txt")
	require.NoError(t, os.WriteFile(src, []byte("x"), 0o600))

	mock := &MockS3Client{
		PutObjectFunc: func(_ context.Context, _ *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			return nil, errors.New("boom")
		},
	}
	stager := NewFileStager(mock, "bkt")
	_, err := stager.Stage(context.Background(), src, "p")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload")
}

func TestFileStager_Cleanup(t *testing.T) {
	t.Parallel()

	t.Run("nil staged is a no-op", func(t *testing.T) {
		t.Parallel()
		stager := NewFileStager(&MockS3Client{}, "bkt")
		stager.Cleanup(context.Background(), nil)
	})

	t.Run("deletes single file", func(t *testing.T) {
		t.Parallel()
		var deletedKey string
		mock := &MockS3Client{
			DeleteObjectFunc: func(_ context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
				deletedKey = *params.Key
				return &s3.DeleteObjectOutput{}, nil
			},
		}
		stager := NewFileStager(mock, "bkt")
		stager.Cleanup(context.Background(), &StagedFile{
			Bucket:    "bkt",
			KeyPrefix: "p/",
			BaseName:  "f.txt",
		})
		assert.Equal(t, "p/f.txt", deletedKey)
	})

	t.Run("logs but does not panic on cleanup error", func(t *testing.T) {
		t.Parallel()
		mock := &MockS3Client{
			DeleteObjectFunc: func(_ context.Context, _ *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
				return nil, errors.New("delete failed")
			},
		}
		stager := NewFileStager(mock, "bkt")
		stager.Cleanup(context.Background(), &StagedFile{
			Bucket:    "bkt",
			KeyPrefix: "p/",
			BaseName:  "f.txt",
		})
	})
}
