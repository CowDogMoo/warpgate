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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewResourceCleaner(t *testing.T) {
	t.Parallel()

	cleaner := NewResourceCleaner(nil)
	assert.NotNil(t, cleaner)
	assert.Nil(t, cleaner.clients)
}

func TestGroupResourcesByType(t *testing.T) {
	t.Parallel()

	cleaner := NewResourceCleaner(nil)

	resources := []ResourceInfo{
		{Type: "Component", Name: "comp1", ARN: "arn:comp1"},
		{Type: "ImagePipeline", Name: "pipe1", ARN: "arn:pipe1"},
		{Type: "Component", Name: "comp2", ARN: "arn:comp2"},
		{Type: "ImageRecipe", Name: "recipe1", ARN: "arn:recipe1"},
		{Type: "ImagePipeline", Name: "pipe2", ARN: "arn:pipe2"},
	}

	grouped := cleaner.groupResourcesByType(resources)

	assert.Len(t, grouped["Component"], 2)
	assert.Len(t, grouped["ImagePipeline"], 2)
	assert.Len(t, grouped["ImageRecipe"], 1)
	assert.Len(t, grouped["DistributionConfiguration"], 0)
}

func TestGroupResourcesByTypeEmpty(t *testing.T) {
	t.Parallel()

	cleaner := NewResourceCleaner(nil)
	grouped := cleaner.groupResourcesByType(nil)
	assert.Empty(t, grouped)
}
