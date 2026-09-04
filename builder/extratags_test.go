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

package builder

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"testing"
)

// requestedTagsConfig is a template declaring one floating alias, built for the
// given architectures. The release tags come from the command line.
func requestedTagsConfig(architectures ...string) Config {
	return Config{
		Name:          "mealie",
		Version:       "v3.24.0",
		Registry:      "ghcr.io/cowdogmoo",
		Architectures: architectures,
		Targets:       []Target{{Type: "container", Registry: "ghcr.io/cowdogmoo", Tags: []string{"stable"}}},
	}
}

// TestPushMultiArchPublishesEveryRequestedTag pins the rule that every --tag
// reaches the registry. The first names the release; the rest publish beside it,
// as the target's declared tags do. Dropping them published a release under
// fewer names than the operator asked for, with no error to say so.
func TestPushMultiArchPublishesEveryRequestedTag(t *testing.T) {
	var mu sync.Mutex
	var built, pushed []string

	bldr := recordingBuilder(&built, &pushed, &mu)
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })

	cfg := requestedTagsConfig("amd64", "arm64")
	opts := BuildOptions{
		Registry: "ghcr.io/cowdogmoo",
		Push:     true,
		Tags:     []string{"v9.9.9", "v9.9", "rc"},
	}

	results, err := svc.ExecuteContainerBuild(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("ExecuteContainerBuild() error = %v", err)
	}
	if err := svc.Push(context.Background(), cfg, results, opts); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	want := []string{
		"ghcr.io/cowdogmoo/mealie:rc",
		"ghcr.io/cowdogmoo/mealie:stable",
		"ghcr.io/cowdogmoo/mealie:v9.9",
		"ghcr.io/cowdogmoo/mealie:v9.9.9",
	}
	if got := bldr.manifestNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("manifest lists = %v, want %v", got, want)
	}
}

// TestPushSingleArchAppliesEveryRequestedTag checks the same rule on the
// single-architecture path, where the extra tags are applied to the image itself
// rather than to a manifest list.
func TestPushSingleArchAppliesEveryRequestedTag(t *testing.T) {
	var mu sync.Mutex
	var built, pushed []string

	bldr := recordingBuilder(&built, &pushed, &mu)
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })

	cfg := requestedTagsConfig("amd64")
	opts := BuildOptions{
		Registry: "ghcr.io/cowdogmoo",
		Push:     true,
		Tags:     []string{"v9.9.9", "v9.9"},
	}

	results, err := svc.ExecuteContainerBuild(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("ExecuteContainerBuild() error = %v", err)
	}
	if err := svc.Push(context.Background(), cfg, results, opts); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	want := []string{
		"ghcr.io/cowdogmoo/mealie:stable",
		"ghcr.io/cowdogmoo/mealie:v9.9",
		"ghcr.io/cowdogmoo/mealie:v9.9.9",
	}

	sort.Strings(built)
	if !reflect.DeepEqual(built, want) {
		t.Errorf("tagged %v, want %v", built, want)
	}

	sort.Strings(pushed)
	if !reflect.DeepEqual(pushed, want) {
		t.Errorf("pushed %v, want %v", pushed, want)
	}
}
