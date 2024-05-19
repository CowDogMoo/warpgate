package packer_test

import (
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/stretchr/testify/assert"
)

func TestParseImageHashes(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedHashes map[string]string
	}{
		{
			name: "valid output with hashes",
			output: `==> docker.arm64: Pulling Docker image: ubuntu:jammy
docker.arm64: Digest: sha256:a6d2b38300ce017add71440577d5b0a90460d0e57fd7aec21dd0d1b0761bbfb2
==> docker.arm64: Status: Image is up to date for ubuntu:jammy`,
			expectedHashes: map[string]string{"arm64": "a6d2b38300ce017add71440577d5b0a90460d0e57fd7aec21dd0d1b0761bbfb2"},
		},
		{
			name:           "invalid output without hashes",
			output:         "No Docker image imported",
			expectedHashes: map[string]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pTmpl := &packer.PackerTemplate{}
			pTmpl.ParseImageHashes(tc.output)
			assert.Equal(t, tc.expectedHashes, pTmpl.Container.ImageHashes)
		})
	}
}
