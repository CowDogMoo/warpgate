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

import "fmt"

// ImageRef composes the reference for one tag of the configured image, applying
// the resolved registry when the configuration carries one.
func ImageRef(cfg Config, tag string) string {
	ref := fmt.Sprintf("%s:%s", cfg.Name, tag)
	if cfg.Registry != "" {
		ref = fmt.Sprintf("%s/%s", cfg.Registry, ref)
	}

	return ref
}

// PrimaryImageRef composes the reference the build is named after: the image
// tagged with the version the template resolved to.
func PrimaryImageRef(cfg Config) string {
	return ImageRef(cfg, cfg.Version)
}

// AdditionalTagRefs returns a reference for every tag the container targets
// declare beyond the version the image already carries. Order follows the
// template; the version itself and any repeat are dropped so no image is tagged
// twice with the same reference.
//
// A configuration with SkipTargetTags set declares none: the per-architecture
// images of a multi-arch build are components tagged by architecture, and the
// template's tags name the manifest list that unites them.
func AdditionalTagRefs(cfg Config) []string {
	if cfg.SkipTargetTags {
		return nil
	}

	var refs []string
	seen := map[string]bool{cfg.Version: true}

	for i := range cfg.Targets {
		if cfg.Targets[i].Type != "container" {
			continue
		}

		for _, tag := range cfg.Targets[i].Tags {
			if tag == "" || seen[tag] {
				continue
			}

			seen[tag] = true
			refs = append(refs, ImageRef(cfg, tag))
		}
	}

	return refs
}
