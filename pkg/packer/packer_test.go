package packer_test

import (
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestParseImageHashes(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedHashes map[string]string
	}{
		{
			name: "Valid json output with hashes",
			output: "2024/06/30 00:48:27 machine readable: docker.arm64,artifact []string" +
				"{\"0\", \"string\", \"Imported Docker image: sha256:d53e4d1efeecb0af6d7142e15e352256748cd08a1b14c2255297e420005e5553\"}",
			expectedHashes: map[string]string{
				"arm64": "d53e4d1efeecb0af6d7142e15e352256748cd08a1b14c2255297e420005e5553",
			},
		},
		{
			name: "Valid output with hashes",
			output: `==> docker.arm64: Imported Docker image: sha256:9f01c52a412f6094205a94a65b10a9534483bba7f27b68a87779d50fb8e56c68
==> docker.amd64: Imported Docker image: sha256:f19949237afd3ac31f1344b40207bca433523ec2939e9321d7f7e8a39a4c2ef6`,
			expectedHashes: map[string]string{
				"arm64": "9f01c52a412f6094205a94a65b10a9534483bba7f27b68a87779d50fb8e56c68",
				"amd64": "f19949237afd3ac31f1344b40207bca433523ec2939e9321d7f7e8a39a4c2ef6",
			},
		},
		{
			name:           "Invalid output without hashes",
			output:         "No Docker image imported",
			expectedHashes: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Mock the viper configuration for image hashes
			viper.Set("container.image_hashes", []interface{}{
				map[string]interface{}{"arch": "arm64", "os": "linux"},
				map[string]interface{}{"arch": "amd64", "os": "linux"},
			})

			pTmpl := &packer.PackerTemplates{}
			hashes := pTmpl.ParseImageHashes(tc.output)

			actualHashes := make(map[string]string)
			for _, hash := range hashes {
				actualHashes[hash.Arch] = hash.Hash
			}

			assert.Equal(t, tc.expectedHashes, actualHashes)
		})
	}
}
