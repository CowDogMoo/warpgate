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

	"github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/stretchr/testify/assert"
)

func TestDeterminePlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		provisioner builder.Provisioner
		want        types.Platform
	}{
		{
			name: "powershell provisioner returns Windows",
			provisioner: builder.Provisioner{
				Type: "powershell",
			},
			want: types.PlatformWindows,
		},
		{
			name: "ansible with powershell shell type returns Windows",
			provisioner: builder.Provisioner{
				Type: "ansible",
				ExtraVars: map[string]string{
					"ansible_shell_type": "powershell",
				},
			},
			want: types.PlatformWindows,
		},
		{
			name: "ansible with cmd shell type returns Windows",
			provisioner: builder.Provisioner{
				Type: "ansible",
				ExtraVars: map[string]string{
					"ansible_shell_type": "cmd",
				},
			},
			want: types.PlatformWindows,
		},
		{
			name: "ansible with ssm and powershell returns Windows",
			provisioner: builder.Provisioner{
				Type: "ansible",
				ExtraVars: map[string]string{
					"ansible_connection": "aws_ssm",
					"ansible_shell_type": "powershell",
				},
			},
			want: types.PlatformWindows,
		},
		{
			name: "ansible with ssm but no powershell returns Linux",
			provisioner: builder.Provisioner{
				Type: "ansible",
				ExtraVars: map[string]string{
					"ansible_connection": "aws_ssm",
					"ansible_shell_type": "bash",
				},
			},
			want: types.PlatformLinux,
		},
		{
			name: "ansible without extra vars returns Linux",
			provisioner: builder.Provisioner{
				Type:      "ansible",
				ExtraVars: map[string]string{},
			},
			want: types.PlatformLinux,
		},
		{
			name: "shell provisioner returns Linux",
			provisioner: builder.Provisioner{
				Type: "shell",
			},
			want: types.PlatformLinux,
		},
		{
			name:        "empty provisioner returns Linux",
			provisioner: builder.Provisioner{},
			want:        types.PlatformLinux,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := determinePlatform(tc.provisioner)
			assert.Equal(t, tc.want, got)
		})
	}
}
