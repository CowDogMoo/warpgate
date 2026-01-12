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

package ami

import (
	"context"
	"testing"
)

func TestNewBatchOperations(t *testing.T) {
	tests := []struct {
		name    string
		clients *AWSClients
	}{
		{
			name:    "nil clients",
			clients: nil,
		},
		{
			name:    "empty clients",
			clients: &AWSClients{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bo := NewBatchOperations(tt.clients)
			if bo == nil {
				t.Fatalf("NewBatchOperations() returned nil")
			}
			if bo.clients != tt.clients {
				t.Errorf("NewBatchOperations() clients = %v, want %v", bo.clients, tt.clients)
			}
		})
	}
}

func TestBatchTagResources_EmptyInput(t *testing.T) {
	bo := NewBatchOperations(nil)

	tests := []struct {
		name        string
		resourceIDs []string
		tags        map[string]string
		wantErr     bool
	}{
		{
			name:        "empty resource IDs",
			resourceIDs: []string{},
			tags:        map[string]string{"key": "value"},
			wantErr:     false,
		},
		{
			name:        "empty tags",
			resourceIDs: []string{"i-12345"},
			tags:        map[string]string{},
			wantErr:     false,
		},
		{
			name:        "both empty",
			resourceIDs: []string{},
			tags:        map[string]string{},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bo.BatchTagResources(context.Background(), tt.resourceIDs, tt.tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchTagResources() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBatchDeleteComponents_EmptyInput(t *testing.T) {
	bo := NewBatchOperations(nil)

	err := bo.BatchDeleteComponents(context.Background(), []string{})
	if err != nil {
		t.Errorf("BatchDeleteComponents() with empty input should return nil, got %v", err)
	}
}

func TestBatchDescribeImages_EmptyInput(t *testing.T) {
	bo := NewBatchOperations(nil)

	images, err := bo.BatchDescribeImages(context.Background(), []string{})
	if err != nil {
		t.Errorf("BatchDescribeImages() error = %v", err)
	}
	if images != nil {
		t.Errorf("BatchDescribeImages() with empty input should return nil, got %v", images)
	}
}

func TestBatchGetComponentVersions_EmptyInput(t *testing.T) {
	bo := NewBatchOperations(nil)

	versions, err := bo.BatchGetComponentVersions(context.Background(), []string{})
	if err != nil {
		t.Errorf("BatchGetComponentVersions() error = %v", err)
	}
	if versions != nil {
		t.Errorf("BatchGetComponentVersions() with empty input should return nil, got %v", versions)
	}
}

func TestBatchCheckResourceExistence_EmptyInput(t *testing.T) {
	bo := NewBatchOperations(nil)

	results := bo.BatchCheckResourceExistence(context.Background(), []ResourceCheck{})
	if results == nil {
		t.Errorf("BatchCheckResourceExistence() should return empty map, not nil")
	}
	if len(results) != 0 {
		t.Errorf("BatchCheckResourceExistence() with empty input should return empty map, got %v", results)
	}
}

func TestResourceCheck(t *testing.T) {
	tests := []struct {
		name     string
		check    ResourceCheck
		wantType string
		wantName string
	}{
		{
			name:     "pipeline resource check",
			check:    ResourceCheck{Type: "pipeline", Name: "test-pipeline"},
			wantType: "pipeline",
			wantName: "test-pipeline",
		},
		{
			name:     "recipe resource check",
			check:    ResourceCheck{Type: "recipe", Name: "test-recipe"},
			wantType: "recipe",
			wantName: "test-recipe",
		},
		{
			name:     "infra resource check",
			check:    ResourceCheck{Type: "infra", Name: "test-infra"},
			wantType: "infra",
			wantName: "test-infra",
		},
		{
			name:     "dist resource check",
			check:    ResourceCheck{Type: "dist", Name: "test-dist"},
			wantType: "dist",
			wantName: "test-dist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.check.Type != tt.wantType {
				t.Errorf("ResourceCheck.Type = %v, want %v", tt.check.Type, tt.wantType)
			}
			if tt.check.Name != tt.wantName {
				t.Errorf("ResourceCheck.Name = %v, want %v", tt.check.Name, tt.wantName)
			}
		})
	}
}
