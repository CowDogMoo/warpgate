/*
Copyright (c) 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

package main

import (
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/logging"
	"github.com/spf13/cobra"
)

func TestRunTemplatesList_BadFormat(t *testing.T) {
	ctx := setupTestContext(t)
	logger := logging.FromContext(ctx)
	ctx = logging.WithLogger(ctx, logger)

	cmd := &cobra.Command{Use: "list"}
	cmd.SetContext(ctx)

	// Save and restore globals
	oldFormat := templatesListFormat
	defer func() { templatesListFormat = oldFormat }()
	templatesListFormat = "invalid-format"

	err := runTemplatesList(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("error should mention invalid format, got: %v", err)
	}
}

func TestRunTemplatesAdd_NilConfig(t *testing.T) {
	cmd := newTestCmdNoConfig()

	err := runTemplatesAdd(cmd, []string{"/some/path"})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config not available") {
		t.Errorf("error should mention config not available, got: %v", err)
	}
}

func TestRunTemplatesRemove_NilConfig(t *testing.T) {
	cmd := newTestCmdNoConfig()

	err := runTemplatesRemove(cmd, []string{"some-source"})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config not available") {
		t.Errorf("error should mention config not available, got: %v", err)
	}
}
