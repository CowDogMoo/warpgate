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

package proxmox

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapWithRemediation_NilError(t *testing.T) {
	t.Parallel()
	if err := WrapWithRemediation(nil, "anything"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWrapWithRemediation_KnownPatterns(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		raw       string
		wantHint  string
		wantUnwrp bool
	}{
		{"auth failure", "401 Unauthorized", "PVEVMAdmin", true},
		{"vmid collision", "configuration file already exists", "auto-allocate", true},
		{"missing node", "no such node 'pve9'", "target.node value", true},
		{"locked vm", "VM is locked (clone)", "another task is running", true},
		{"tls error", "x509: certificate signed by unknown authority", "insecure_skip_verify", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw := errors.New(tc.raw)
			wrapped := WrapWithRemediation(raw, "doing a thing")
			if wrapped == nil {
				t.Fatalf("expected wrapped error, got nil")
			}
			if !strings.Contains(wrapped.Error(), tc.wantHint) {
				t.Fatalf("wrapped error missing remediation hint %q; got: %v", tc.wantHint, wrapped)
			}
			if tc.wantUnwrp && !errors.Is(wrapped, raw) {
				t.Fatalf("errors.Is should unwrap to original; got: %v", wrapped)
			}
		})
	}
}

func TestWrapWithRemediation_UnknownPattern(t *testing.T) {
	t.Parallel()
	raw := errors.New("something totally unexpected")
	wrapped := WrapWithRemediation(raw, "ctx")
	if !errors.Is(wrapped, raw) {
		t.Fatalf("expected errors.Is to find raw; got %v", wrapped)
	}
	if !strings.Contains(wrapped.Error(), "ctx:") {
		t.Fatalf("expected context prefix in error; got %v", wrapped)
	}
}

func TestBuildError_Error(t *testing.T) {
	t.Parallel()
	be := &BuildError{
		Context:     "clone template",
		Remediation: "check ACLs",
		Err:         errors.New("boom"),
	}
	got := be.Error()
	for _, want := range []string{"clone template", "boom", "check ACLs", "remediation:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("BuildError.Error() missing %q; got %q", want, got)
		}
	}
}
