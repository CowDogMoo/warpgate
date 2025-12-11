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

package cli

import (
	"testing"
)

func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKey   string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "valid key-value pair",
			input:     "key=value",
			wantKey:   "key",
			wantValue: "value",
			wantErr:   false,
		},
		{
			name:      "key-value with spaces",
			input:     "key = value ",
			wantKey:   "key",
			wantValue: "value",
			wantErr:   false,
		},
		{
			name:      "value with equals sign",
			input:     "key=value=with=equals",
			wantKey:   "key",
			wantValue: "value=with=equals",
			wantErr:   false,
		},
		{
			name:      "empty value",
			input:     "key=",
			wantKey:   "key",
			wantValue: "",
			wantErr:   false,
		},
		{
			name:    "no equals sign",
			input:   "keyvalue",
			wantErr: true,
		},
		{
			name:    "empty key",
			input:   "=value",
			wantErr: true,
		},
		{
			name:    "only spaces as key",
			input:   "  =value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotValue, err := ParseKeyValue(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKeyValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotKey != tt.wantKey {
					t.Errorf("ParseKeyValue() key = %v, want %v", gotKey, tt.wantKey)
				}
				if gotValue != tt.wantValue {
					t.Errorf("ParseKeyValue() value = %v, want %v", gotValue, tt.wantValue)
				}
			}
		})
	}
}

func TestParseKeyValuePairs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "valid pairs",
			input: []string{"key1=value1", "key2=value2"},
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name:  "multiple pairs with special characters",
			input: []string{"VERSION=1.0.0", "REPO_PATH=/path/to/repo", "DEBUG=true"},
			want: map[string]string{
				"VERSION":   "1.0.0",
				"REPO_PATH": "/path/to/repo",
				"DEBUG":     "true",
			},
			wantErr: false,
		},
		{
			name:    "empty slice",
			input:   []string{},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "nil slice",
			input:   nil,
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "invalid pair in list",
			input:   []string{"key1=value1", "invalidpair"},
			wantErr: true,
		},
		{
			name:  "duplicate keys - last wins",
			input: []string{"key1=value1", "key1=value2"},
			want: map[string]string{
				"key1": "value2",
			},
			wantErr: false,
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseKeyValuePairs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKeyValuePairs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("ParseKeyValuePairs() got %d pairs, want %d", len(got), len(tt.want))
				}
				for key, wantValue := range tt.want {
					if gotValue, ok := got[key]; !ok || gotValue != wantValue {
						t.Errorf("ParseKeyValuePairs() key %s = %v, want %v", key, gotValue, wantValue)
					}
				}
			}
		})
	}
}

func TestParseLabels(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "valid labels",
			input: []string{"version=1.0", "maintainer=team"},
			want: map[string]string{
				"version":    "1.0",
				"maintainer": "team",
			},
			wantErr: false,
		},
		{
			name:    "empty labels",
			input:   []string{},
			want:    nil,
			wantErr: false,
		},
		{
			name:    "nil labels",
			input:   nil,
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseLabels(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tt.want == nil && got != nil {
					t.Errorf("ParseLabels() = %v, want nil", got)
				} else if tt.want != nil {
					if len(got) != len(tt.want) {
						t.Errorf("ParseLabels() got %d labels, want %d", len(got), len(tt.want))
					}
					for key, wantValue := range tt.want {
						if gotValue, ok := got[key]; !ok || gotValue != wantValue {
							t.Errorf("ParseLabels() key %s = %v, want %v", key, gotValue, wantValue)
						}
					}
				}
			}
		})
	}
}

func TestParseBuildArgs(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "valid build args",
			input: []string{"GO_VERSION=1.25", "BUILD_DATE=2025-12-10"},
			want: map[string]string{
				"GO_VERSION": "1.25",
				"BUILD_DATE": "2025-12-10",
			},
			wantErr: false,
		},
		{
			name:    "empty build args",
			input:   []string{},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseBuildArgs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBuildArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tt.want == nil && got != nil {
					t.Errorf("ParseBuildArgs() = %v, want nil", got)
				} else if tt.want != nil {
					if len(got) != len(tt.want) {
						t.Errorf("ParseBuildArgs() got %d args, want %d", len(got), len(tt.want))
					}
					for key, wantValue := range tt.want {
						if gotValue, ok := got[key]; !ok || gotValue != wantValue {
							t.Errorf("ParseBuildArgs() key %s = %v, want %v", key, gotValue, wantValue)
						}
					}
				}
			}
		})
	}
}

func TestParseVariables(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "valid variables",
			input: []string{"VAR1=value1", "VAR2=value2"},
			want: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			wantErr: false,
		},
		{
			name:    "empty variables",
			input:   []string{},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseVariables(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tt.want == nil && got != nil {
					t.Errorf("ParseVariables() = %v, want nil", got)
				} else if tt.want != nil {
					if len(got) != len(tt.want) {
						t.Errorf("ParseVariables() got %d vars, want %d", len(got), len(tt.want))
					}
					for key, wantValue := range tt.want {
						if gotValue, ok := got[key]; !ok || gotValue != wantValue {
							t.Errorf("ParseVariables() key %s = %v, want %v", key, gotValue, wantValue)
						}
					}
				}
			}
		})
	}
}

func TestValidateKeyValueFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid format",
			input: "key=value",
			want:  true,
		},
		{
			name:  "valid with spaces",
			input: " key = value ",
			want:  true,
		},
		{
			name:  "no equals",
			input: "keyvalue",
			want:  false,
		},
		{
			name:  "empty key",
			input: "=value",
			want:  false,
		},
		{
			name:  "spaces as key",
			input: "  =value",
			want:  false,
		},
		{
			name:  "valid empty value",
			input: "key=",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateKeyValueFormat(tt.input); got != tt.want {
				t.Errorf("ValidateKeyValueFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}
