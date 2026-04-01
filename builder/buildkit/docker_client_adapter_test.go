package buildkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dockercontainer "github.com/moby/moby/api/types/container"
	dockerimage "github.com/moby/moby/api/types/image"
	dockerregistry "github.com/moby/moby/api/types/registry"
	dockerclient "github.com/moby/moby/client"
)

// writeJSON encodes v as JSON to w, writing a 500 if encoding fails.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, fmt.Sprintf("test handler: encode failed: %v", err), http.StatusInternalServerError)
	}
}

// newTestAdapter creates a dockerClientAdapter backed by a test HTTP server.
// The handler receives all Docker API requests and can return canned responses.
func newTestAdapter(t *testing.T, handler http.Handler) *dockerClientAdapter {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cli, err := dockerclient.New(
		dockerclient.WithHost("tcp://"+srv.Listener.Addr().String()),
		dockerclient.WithHTTPClient(srv.Client()),
		dockerclient.WithAPIVersion("v1.47"),
	)
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })

	return &dockerClientAdapter{Client: cli}
}

func TestAdapter_ImagePush(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/images/") && strings.Contains(r.URL.Path, "/push") {
			w.Header().Set("Content-Type", "application/json")
			if _, err := fmt.Fprintln(w, `{"status":"pushing"}`); err != nil {
				t.Errorf("writing push response: %v", err)
			}
			return
		}
		http.NotFound(w, r)
	}))

	ctx := context.Background()
	rc, err := adapter.ImagePush(ctx, "test:latest", ImagePushOptions{RegistryAuth: "auth123"})
	if err != nil {
		t.Fatalf("ImagePush() error = %v", err)
	}
	defer func() { _ = rc.Close() }()

	body, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading push response: %v", err)
	}
	if !strings.Contains(string(body), "pushing") {
		t.Errorf("expected push status in response, got %s", body)
	}
}

func TestAdapter_ImageTag(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/images/") && strings.Contains(r.URL.Path, "/tag") {
			w.WriteHeader(http.StatusCreated)
			return
		}
		http.NotFound(w, r)
	}))

	ctx := context.Background()
	if err := adapter.ImageTag(ctx, "source:latest", "target:latest"); err != nil {
		t.Fatalf("ImageTag() error = %v", err)
	}
}

func TestAdapter_ImageRemove(t *testing.T) {
	expected := []dockerimage.DeleteResponse{{Deleted: "sha256:abc123"}}
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/images/") {
			writeJSON(w, expected)
			return
		}
		http.NotFound(w, r)
	}))

	ctx := context.Background()
	items, err := adapter.ImageRemove(ctx, "test:latest", ImageRemoveOptions{Force: true, PruneChildren: true})
	if err != nil {
		t.Fatalf("ImageRemove() error = %v", err)
	}
	if len(items) != 1 || items[0].Deleted != "sha256:abc123" {
		t.Errorf("ImageRemove() = %v, want %v", items, expected)
	}
}

func TestAdapter_ImageRemove_Error(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if _, err := fmt.Fprintln(w, `{"message":"no such image"}`); err != nil {
			t.Errorf("writing error response: %v", err)
		}
	}))

	ctx := context.Background()
	if _, err := adapter.ImageRemove(ctx, "nonexistent:latest", ImageRemoveOptions{}); err == nil {
		t.Fatal("ImageRemove() expected error for missing image")
	}
}

func TestAdapter_ImageLoad(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/images/load") {
			w.Header().Set("Content-Type", "application/json")
			if _, err := fmt.Fprintln(w, `{"stream":"Loaded image: test:latest"}`); err != nil {
				t.Errorf("writing load response: %v", err)
			}
			return
		}
		http.NotFound(w, r)
	}))

	ctx := context.Background()
	resp, err := adapter.ImageLoad(ctx, strings.NewReader("fake-tar-data"))
	if err != nil {
		t.Fatalf("ImageLoad() error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading load response: %v", err)
	}
	if !strings.Contains(string(body), "Loaded image") {
		t.Errorf("expected load response, got %s", body)
	}
}

func TestAdapter_ImageLoad_Error(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := fmt.Fprintln(w, `{"message":"server error"}`); err != nil {
			t.Errorf("writing error response: %v", err)
		}
	}))

	ctx := context.Background()
	if _, err := adapter.ImageLoad(ctx, strings.NewReader("fake")); err == nil {
		t.Fatal("ImageLoad() expected error")
	}
}

func TestAdapter_ImageInspect(t *testing.T) {
	inspectResp := dockerimage.InspectResponse{
		ID:       "sha256:abc123",
		RepoTags: []string{"test:latest"},
	}
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/images/") && strings.Contains(r.URL.Path, "/json") {
			writeJSON(w, inspectResp)
			return
		}
		http.NotFound(w, r)
	}))

	ctx := context.Background()
	result, err := adapter.ImageInspect(ctx, "test:latest")
	if err != nil {
		t.Fatalf("ImageInspect() error = %v", err)
	}
	if result.ID != "sha256:abc123" {
		t.Errorf("ImageInspect() ID = %s, want sha256:abc123", result.ID)
	}
}

func TestAdapter_ImageInspect_Error(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if _, err := fmt.Fprintln(w, `{"message":"no such image"}`); err != nil {
			t.Errorf("writing error response: %v", err)
		}
	}))

	ctx := context.Background()
	if _, err := adapter.ImageInspect(ctx, "nonexistent"); err == nil {
		t.Fatal("ImageInspect() expected error")
	}
}

func TestAdapter_ImageInspectWithRaw(t *testing.T) {
	inspectResp := dockerimage.InspectResponse{
		ID:       "sha256:def456",
		RepoTags: []string{"raw:latest"},
	}
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/images/") && strings.Contains(r.URL.Path, "/json") {
			writeJSON(w, inspectResp)
			return
		}
		http.NotFound(w, r)
	}))

	ctx := context.Background()
	result, raw, err := adapter.ImageInspectWithRaw(ctx, "raw:latest")
	if err != nil {
		t.Fatalf("ImageInspectWithRaw() error = %v", err)
	}
	if result.ID != "sha256:def456" {
		t.Errorf("ImageInspectWithRaw() ID = %s, want sha256:def456", result.ID)
	}
	if len(raw) == 0 {
		t.Error("ImageInspectWithRaw() raw bytes should not be empty")
	}
}

func TestAdapter_ImageInspectWithRaw_Error(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if _, err := fmt.Fprintln(w, `{"message":"no such image"}`); err != nil {
			t.Errorf("writing error response: %v", err)
		}
	}))

	ctx := context.Background()
	if _, _, err := adapter.ImageInspectWithRaw(ctx, "nonexistent"); err == nil {
		t.Fatal("ImageInspectWithRaw() expected error")
	}
}

func TestAdapter_DistributionInspect(t *testing.T) {
	distResp := dockerregistry.DistributionInspect{}
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/distribution/") {
			writeJSON(w, distResp)
			return
		}
		http.NotFound(w, r)
	}))

	ctx := context.Background()
	if _, err := adapter.DistributionInspect(ctx, "test:latest", "auth123"); err != nil {
		t.Fatalf("DistributionInspect() error = %v", err)
	}
}

func TestAdapter_DistributionInspect_Error(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if _, err := fmt.Fprintln(w, `{"message":"not found"}`); err != nil {
			t.Errorf("writing error response: %v", err)
		}
	}))

	ctx := context.Background()
	if _, err := adapter.DistributionInspect(ctx, "missing:latest", "auth"); err == nil {
		t.Fatal("DistributionInspect() expected error")
	}
}

func TestAdapter_ContainerList(t *testing.T) {
	containers := []dockercontainer.Summary{
		{ID: "c1", Names: []string{"/buildx_buildkit_default0"}, State: "running"},
		{ID: "c2", Names: []string{"/other"}, State: "exited"},
	}
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/containers/json") {
			writeJSON(w, containers)
			return
		}
		http.NotFound(w, r)
	}))

	ctx := context.Background()
	items, err := adapter.ContainerList(ctx, ContainerListOptions{All: true})
	if err != nil {
		t.Fatalf("ContainerList() error = %v", err)
	}
	if len(items) != 2 {
		t.Errorf("ContainerList() returned %d items, want 2", len(items))
	}
}

func TestAdapter_ContainerList_Error(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := fmt.Fprintln(w, `{"message":"server error"}`); err != nil {
			t.Errorf("writing error response: %v", err)
		}
	}))

	ctx := context.Background()
	if _, err := adapter.ContainerList(ctx, ContainerListOptions{All: true}); err == nil {
		t.Fatal("ContainerList() expected error")
	}
}

func TestAdapter_Ping(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/_ping") {
			w.Header().Set("Api-Version", "1.47")
			w.Header().Set("Ostype", "linux")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))

	ctx := context.Background()
	if _, err := adapter.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

func TestAdapter_Ping_Error(t *testing.T) {
	adapter := newTestAdapter(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := fmt.Fprintln(w, `{"message":"daemon not ready"}`); err != nil {
			t.Errorf("writing error response: %v", err)
		}
	}))

	ctx := context.Background()
	if _, err := adapter.Ping(ctx); err == nil {
		t.Fatal("Ping() expected error for unhealthy daemon")
	}
}

func TestNewDockerClientAdapter(t *testing.T) {
	cli, err := dockerclient.New(
		dockerclient.WithHost("tcp://localhost:0"),
		dockerclient.WithAPIVersion("v1.47"),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer func() { _ = cli.Close() }()

	adapter := newDockerClientAdapter(cli)
	if adapter == nil {
		t.Fatal("newDockerClientAdapter() returned nil")
	}

	// Verify it satisfies the DockerClient interface
	_ = DockerClient(adapter)
}
