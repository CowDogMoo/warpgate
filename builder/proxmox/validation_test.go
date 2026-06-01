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
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/builder"
)

func TestRequireProxmoxFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		target  *builder.Target
		wantErr string
	}{
		{"nil target", nil, "target is nil"},
		{"missing node", &builder.Target{SourceTemplate: 9000, TemplateName: "kali"}, "node is required"},
		{
			name: "missing source ref",
			target: &builder.Target{
				Node:         "pve1",
				TemplateName: "kali",
			},
			wantErr: "source_template",
		},
		{
			name: "source vmid too low",
			target: &builder.Target{
				Node:           "pve1",
				SourceTemplate: 50,
				TemplateName:   "kali",
			},
			wantErr: ">= 100",
		},
		{
			name: "missing template name",
			target: &builder.Target{
				Node:           "pve1",
				SourceTemplate: 9000,
			},
			wantErr: "template_name",
		},
		{
			name: "happy path with vmid",
			target: &builder.Target{
				Node:           "pve1",
				SourceTemplate: 9000,
				TemplateName:   "kali",
			},
			wantErr: "",
		},
		{
			name: "happy path with name",
			target: &builder.Target{
				Node:               "pve1",
				SourceTemplateName: "kali-template",
				TemplateName:       "kali",
			},
			wantErr: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := requireProxmoxFields(tc.target)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestFindProxmoxTarget(t *testing.T) {
	t.Parallel()
	cfg := builder.Config{
		Targets: []builder.Target{
			{Type: "container"},
			{Type: "proxmox", Node: "pve1"},
		},
	}
	target, err := findProxmoxTarget(cfg)
	if err != nil {
		t.Fatalf("expected target, got error %v", err)
	}
	if target.Node != "pve1" {
		t.Fatalf("expected node pve1, got %q", target.Node)
	}

	if _, err := findProxmoxTarget(builder.Config{Targets: []builder.Target{{Type: "ami"}}}); err == nil {
		t.Fatalf("expected error when no proxmox target present")
	}
}

func TestValidatePrerequisites(t *testing.T) {
	t.Parallel()
	if err := ValidatePrerequisites("", "pve1", 9000); err == nil {
		t.Fatalf("expected endpoint error")
	}
	if err := ValidatePrerequisites("https://pve", "", 9000); err == nil {
		t.Fatalf("expected node error")
	}
	if err := ValidatePrerequisites("https://pve", "pve1", 50); err == nil {
		t.Fatalf("expected vmid >= 100 error")
	}
	if err := ValidatePrerequisites("https://pve", "pve1", 9000); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
