package packer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cowdogmoo/warpgate/pkg/packer"
)

func TestParseImageHashes(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedHashes map[string]string
	}{
		{
			name:           "valid output with hashes",
			output:         "Imported Docker image: sha256: null.example: sha256: ad5246ae942c8c9169eda8726e7e330bba72e4e6f8815e7f1e7112844caca4c3",
			expectedHashes: map[string]string{"example": "ad5246ae942c8c9169eda8726e7e330bba72e4e6f8815e7f1e7112844caca4c3"},
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
