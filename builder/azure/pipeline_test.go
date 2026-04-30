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

package azure

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	armresources "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/virtualmachineimagebuilder/armvirtualmachineimagebuilder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redirectingTransport intercepts all HTTP requests and rewrites the host to
// the test server URL so the Azure SDK's hard-coded management.azure.com calls
// are served by the fake server. The path and query are preserved.
type redirectingTransport struct {
	targetURL string
	inner     http.RoundTripper
}

func (t *redirectingTransport) Do(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http"
	cloned.URL.Host = strings.TrimPrefix(t.targetURL, "http://")
	cloned.Host = cloned.URL.Host
	return t.inner.RoundTrip(cloned)
}

// azTestServer holds the test HTTP server and a response registry.
// Each handler uses a simple path-to-response lookup; the catch-all returns
// 500 with the unmatched path so tests fail loudly.
type azTestServer struct {
	server    *httptest.Server
	responses map[string]azResponse
}

type azResponse struct {
	status int
	body   string
}

// newAzTestServer creates a test HTTP server. Routes are evaluated in order;
// prefix-based matching lets a single path handle polling too.
func newAzTestServer(t *testing.T) *azTestServer {
	t.Helper()
	s := &azTestServer{
		responses: make(map[string]azResponse),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		for prefix, resp := range s.responses {
			if strings.HasPrefix(r.URL.Path, prefix) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(resp.status)
				_, _ = fmt.Fprint(w, resp.body)
				return
			}
		}
		http.Error(w, fmt.Sprintf("unhandled path: %s %s", r.Method, r.URL.Path), http.StatusInternalServerError)
	})
	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

// armOpts returns arm.ClientOptions that route all requests through the test
// server by replacing the transport and disabling RP registration retries.
func (s *azTestServer) armOpts() *arm.ClientOptions {
	opts := &arm.ClientOptions{}
	opts.Transport = &redirectingTransport{
		targetURL: s.server.URL,
		inner:     http.DefaultTransport,
	}
	opts.DisableRPRegistration = true
	return opts
}

// aibClient creates a VirtualMachineImageTemplatesClient pointed at the test server.
func (s *azTestServer) aibClient(t *testing.T) *armvirtualmachineimagebuilder.VirtualMachineImageTemplatesClient {
	t.Helper()
	c, err := armvirtualmachineimagebuilder.NewVirtualMachineImageTemplatesClient("sub-1", &fakeTokenCredential{}, s.armOpts())
	require.NoError(t, err)
	return c
}

// galleryVersionsClient creates a GalleryImageVersionsClient pointed at the test server.
func (s *azTestServer) galleryVersionsClient(t *testing.T) *armcompute.GalleryImageVersionsClient {
	t.Helper()
	c, err := armcompute.NewGalleryImageVersionsClient("sub-1", &fakeTokenCredential{}, s.armOpts())
	require.NoError(t, err)
	return c
}

// galleriesClient creates a GalleriesClient pointed at the test server.
func (s *azTestServer) galleriesClient(t *testing.T) *armcompute.GalleriesClient {
	t.Helper()
	c, err := armcompute.NewGalleriesClient("sub-1", &fakeTokenCredential{}, s.armOpts())
	require.NoError(t, err)
	return c
}

// galleryImagesClient creates a GalleryImagesClient pointed at the test server.
func (s *azTestServer) galleryImagesClient(t *testing.T) *armcompute.GalleryImagesClient {
	t.Helper()
	c, err := armcompute.NewGalleryImagesClient("sub-1", &fakeTokenCredential{}, s.armOpts())
	require.NoError(t, err)
	return c
}

// resourcesClient creates an armresources.Client pointed at the test server.
func (s *azTestServer) resourcesClient(t *testing.T) *armresources.Client {
	t.Helper()
	c, err := armresources.NewClient("sub-1", &fakeTokenCredential{}, s.armOpts())
	require.NoError(t, err)
	return c
}

// register sets a prefix-matched response.
func (s *azTestServer) register(path string, status int, body string) {
	s.responses[path] = azResponse{status: status, body: body}
}

// TestPipelineRunner_NewPipelineRunner checks that newPipelineRunner returns a
// non-nil pipelineOps.
func TestPipelineRunner_NewPipelineRunner(t *testing.T) {
	clients := &AzureClients{SubscriptionID: "sub-1"}
	got := newPipelineRunner(clients, "rg-1", 0)
	assert.NotNil(t, got)
}

// TestPipelineRunner_NewPipelineRunner_WithPollFrequency exercises the
// pollFrequency>0 branch which sets pollOptions.
func TestPipelineRunner_NewPipelineRunner_WithPollFrequency(t *testing.T) {
	clients := &AzureClients{SubscriptionID: "sub-1"}
	got := newPipelineRunner(clients, "rg-1", 5*time.Second)
	require.NotNil(t, got)
}

// TestPipelineRunner_Submit_Success tests the submit path through the test server.
func TestPipelineRunner_Submit_Success(t *testing.T) {
	s := newAzTestServer(t)

	// Return a terminal 200 for any AIB imageTemplates path.
	// The SDK performs CreateOrUpdate and then polls the provisioning state.
	// Returning 200 with provisioningState=Succeeded avoids a polling loop.
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates", 200,
		`{"id":"/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1",
		 "name":"tpl1","location":"eastus",
		 "properties":{"provisioningState":"Succeeded"}}`)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	loc := "eastus"
	tpl := &armvirtualmachineimagebuilder.ImageTemplate{Location: &loc}
	err := runner.submit(context.Background(), "tpl1", tpl)
	require.NoError(t, err)
}

// TestPipelineRunner_Submit_Error confirms submit propagates a 403 error.
func TestPipelineRunner_Submit_Error(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates", 403,
		`{"error":{"code":"Forbidden","message":"access denied"}}`)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	loc := "eastus"
	tpl := &armvirtualmachineimagebuilder.ImageTemplate{Location: &loc}
	err := runner.submit(context.Background(), "tpl1", tpl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tpl1")
}

// TestPipelineRunner_Run_Success tests the run (BeginRun) path.
func TestPipelineRunner_Run_Success(t *testing.T) {
	s := newAzTestServer(t)
	// The BeginRun endpoint is a POST that starts an LRO.
	// Returning 200 means "synchronous success" for the SDK.
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1/run", 200,
		`{}`)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	err := runner.run(context.Background(), "tpl1")
	require.NoError(t, err)
}

// TestPipelineRunner_Run_Error confirms run errors propagate.
func TestPipelineRunner_Run_Error(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1/run", 403,
		`{"error":{"code":"Forbidden","message":"access denied"}}`)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	err := runner.run(context.Background(), "tpl1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tpl1")
}

// TestPipelineRunner_ReadArtifact_Success tests the GetRunOutput happy path.
func TestPipelineRunner_ReadArtifact_Success(t *testing.T) {
	s := newAzTestServer(t)
	artifactID := "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/def/versions/2026.0429.1"
	body := fmt.Sprintf(`{"id":"ro1","name":"%s","properties":{"artifactId":"%s","provisioningState":"Succeeded"}}`,
		runOutputName, artifactID)
	s.register(
		fmt.Sprintf("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1/runOutputs/%s", runOutputName),
		200, body)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	got, err := runner.readArtifact(context.Background(), "tpl1")
	require.NoError(t, err)
	assert.Equal(t, artifactID, got)
}

// TestPipelineRunner_ReadArtifact_NoArtifactID checks the missing artifact case.
func TestPipelineRunner_ReadArtifact_NoArtifactID(t *testing.T) {
	s := newAzTestServer(t)
	// Return a run output with no artifactId.
	body := fmt.Sprintf(`{"id":"ro1","name":"%s","properties":{"provisioningState":"Succeeded"}}`, runOutputName)
	s.register(
		fmt.Sprintf("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1/runOutputs/%s", runOutputName),
		200, body)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	_, err := runner.readArtifact(context.Background(), "tpl1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no artifact ID")
}

// TestPipelineRunner_ReadArtifact_Error confirms error propagation on 403.
func TestPipelineRunner_ReadArtifact_Error(t *testing.T) {
	s := newAzTestServer(t)
	s.register(
		fmt.Sprintf("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1/runOutputs/%s", runOutputName),
		403, `{"error":{"code":"Forbidden","message":"access denied"}}`)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	_, err := runner.readArtifact(context.Background(), "tpl1")
	require.Error(t, err)
}

// TestPipelineRunner_DescribeLastRun_Success exercises the Get + status parse.
func TestPipelineRunner_DescribeLastRun_Success(t *testing.T) {
	s := newAzTestServer(t)
	runState := string(armvirtualmachineimagebuilder.RunStateFailed)
	msg := "something went wrong"
	body := fmt.Sprintf(`{
		"id":"/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1",
		"name":"tpl1",
		"location":"eastus",
		"properties":{
			"provisioningState":"Succeeded",
			"lastRunStatus":{"runState":"%s","message":"%s"}
		}
	}`, runState, msg)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1", 200, body)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	desc, err := runner.describeLastRun(context.Background(), "tpl1")
	require.NoError(t, err)
	assert.Contains(t, desc, runState)
	assert.Contains(t, desc, msg)
}

// TestPipelineRunner_DescribeLastRun_NilProperties covers the nil-check path.
func TestPipelineRunner_DescribeLastRun_NilProperties(t *testing.T) {
	s := newAzTestServer(t)
	// Return a template with no properties.
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1", 200,
		`{"id":"tpl1","name":"tpl1","location":"eastus"}`)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	desc, err := runner.describeLastRun(context.Background(), "tpl1")
	require.NoError(t, err)
	assert.Empty(t, desc)
}

// TestPipelineRunner_DescribeLastRun_Error tests error propagation from Get.
func TestPipelineRunner_DescribeLastRun_Error(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1", 403,
		`{"error":{"code":"Forbidden","message":"access denied"}}`)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	_, err := runner.describeLastRun(context.Background(), "tpl1")
	require.Error(t, err)
}

// TestPipelineRunner_DeleteTemplate_Success confirms delete does not panic on 200.
func TestPipelineRunner_DeleteTemplate_Success(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1", 200,
		`{}`)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	// deleteTemplate logs errors but does not return them — must not panic.
	runner.deleteTemplate(context.Background(), "tpl1")
}

// TestPipelineRunner_DeleteTemplate_Error confirms delete swallows the error.
func TestPipelineRunner_DeleteTemplate_Error(t *testing.T) {
	s := newAzTestServer(t)
	// Use 403 Forbidden (not retried by the SDK) to keep the test fast.
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.VirtualMachineImages/imageTemplates/tpl1", 403,
		`{"error":{"code":"Forbidden","message":"access denied"}}`)

	aib := s.aibClient(t)
	runner := &pipelineRunner{
		clients:       &AzureClients{ImageTemplates: aib},
		resourceGroup: "rg-1",
	}

	// Must not panic or propagate an error.
	runner.deleteTemplate(context.Background(), "tpl1")
}

// ----- operations.go tests using the test server -----

const testVersionID = "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/img/versions/1.0.0"

// TestDeleteGalleryVersion_InvalidID checks the parse-error guard.
func TestDeleteGalleryVersion_InvalidID(t *testing.T) {
	clients := &AzureClients{SubscriptionID: "sub-1"}
	err := deleteGalleryVersion(context.Background(), clients, "/not/a/version")
	require.Error(t, err)
}

// TestDeleteGalleryVersion_Success drives BeginDelete through the test server.
func TestDeleteGalleryVersion_Success(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/img/versions/1.0.0", 200,
		`{}`)

	vc := s.galleryVersionsClient(t)
	clients := &AzureClients{GalleryImageVersions: vc}

	err := deleteGalleryVersion(context.Background(), clients, testVersionID)
	require.NoError(t, err)
}

// TestDeleteGalleryVersion_Error propagates SDK errors.
func TestDeleteGalleryVersion_Error(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/img/versions/1.0.0", 403,
		`{"error":{"code":"Forbidden","message":"access denied"}}`)

	vc := s.galleryVersionsClient(t)
	clients := &AzureClients{GalleryImageVersions: vc}

	err := deleteGalleryVersion(context.Background(), clients, testVersionID)
	require.Error(t, err)
}

// TestUpdateGalleryVersionRegions_InvalidID ensures parse failure is returned early.
func TestUpdateGalleryVersionRegions_InvalidID(t *testing.T) {
	clients := &AzureClients{SubscriptionID: "sub-1"}
	err := updateGalleryVersionRegions(context.Background(), clients, "/not/a/version", []string{"westus"})
	require.Error(t, err)
}

// TestUpdateGalleryVersionRegions_Success drives the Get→BeginUpdate flow.
func TestUpdateGalleryVersionRegions_Success(t *testing.T) {
	s := newAzTestServer(t)
	versionPath := "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/img/versions/1.0.0"

	// The SDK will GET then PATCH/PUT (BeginUpdate). Returning 200 for both
	// via a single prefix registration works because both share the path.
	s.register(versionPath, 200, `{
		"id":"`+versionPath+`",
		"name":"1.0.0",
		"location":"eastus",
		"properties":{
			"provisioningState":"Succeeded",
			"publishingProfile":{"targetRegions":[{"name":"eastus"}]},
			"storageProfile":{}
		}
	}`)

	vc := s.galleryVersionsClient(t)
	clients := &AzureClients{GalleryImageVersions: vc}

	err := updateGalleryVersionRegions(context.Background(), clients, testVersionID, []string{"westus"})
	require.NoError(t, err)
}

// TestUpdateGalleryVersionRegions_GetError propagates a Get failure.
func TestUpdateGalleryVersionRegions_GetError(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/img/versions/1.0.0", 403,
		`{"error":{"code":"Forbidden","message":"access denied"}}`)

	vc := s.galleryVersionsClient(t)
	clients := &AzureClients{GalleryImageVersions: vc}

	err := updateGalleryVersionRegions(context.Background(), clients, testVersionID, []string{"westus"})
	require.Error(t, err)
}

// ----- discover.go adapter tests -----

// TestGalleriesAdapter_ListGalleries_Success exercises the SDK pager adapter
// using a test server that returns a single page of galleries.
func TestGalleriesAdapter_ListGalleries_Success(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/providers/Microsoft.Compute/galleries", 200, `{
		"value":[
			{
				"id":"/subscriptions/sub-1/resourceGroups/rg-builds/providers/Microsoft.Compute/galleries/mygallery",
				"name":"mygallery",
				"location":"eastus",
				"tags":{"warpgate":"true"}
			}
		]
	}`)

	gc := s.galleriesClient(t)
	adapter := &galleriesAdapter{c: gc}

	galleries, err := adapter.ListGalleries(context.Background())
	require.NoError(t, err)
	require.Len(t, galleries, 1)
	assert.Equal(t, "mygallery", galleries[0].Name)
	assert.Equal(t, "rg-builds", galleries[0].ResourceGroup)
	assert.Equal(t, "eastus", galleries[0].Location)
	assert.Equal(t, "true", galleries[0].Tags["warpgate"])
}

// TestGalleriesAdapter_ListGalleries_Error propagates SDK errors.
func TestGalleriesAdapter_ListGalleries_Error(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/providers/Microsoft.Compute/galleries", 401,
		`{"error":{"code":"Unauthorized","message":"access denied"}}`)

	gc := s.galleriesClient(t)
	adapter := &galleriesAdapter{c: gc}

	_, err := adapter.ListGalleries(context.Background())
	require.Error(t, err)
}

// TestGalleryImagesAdapter_ListImageDefs_Success exercises the gallery images pager.
func TestGalleryImagesAdapter_ListImageDefs_Success(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/mygallery/images", 200, `{
		"value":[
			{
				"id":"/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/mygallery/images/ubuntu-22",
				"name":"ubuntu-22",
				"location":"eastus",
				"properties":{"osType":"Linux","provisioningState":"Succeeded"}
			}
		]
	}`)

	imgc := s.galleryImagesClient(t)
	adapter := &galleryImagesAdapter{c: imgc}

	images, err := adapter.ListImageDefs(context.Background(), "rg-1", "mygallery")
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, "ubuntu-22", images[0].Name)
	assert.Equal(t, "Linux", images[0].OSType)
}

// TestGalleryImagesAdapter_ListImageDefs_Error propagates errors.
func TestGalleryImagesAdapter_ListImageDefs_Error(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/mygallery/images", 403,
		`{"error":{"code":"Forbidden","message":"access denied"}}`)

	imgc := s.galleryImagesClient(t)
	adapter := &galleryImagesAdapter{c: imgc}

	_, err := adapter.ListImageDefs(context.Background(), "rg-1", "mygallery")
	require.Error(t, err)
}

// TestResourcesAdapter_ListByResourceGroup_Success exercises the ARM resources pager.
func TestResourcesAdapter_ListByResourceGroup_Success(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/resources", 200, `{
		"value":[
			{
				"id":"/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ManagedIdentity/userAssignedIdentities/my-uami",
				"name":"my-uami",
				"type":"Microsoft.ManagedIdentity/userAssignedIdentities",
				"tags":{"warpgate":"true"}
			}
		]
	}`)

	rc := s.resourcesClient(t)
	adapter := &resourcesAdapter{c: rc}

	resources, err := adapter.ListByResourceGroup(context.Background(), "rg-1", uamiResourceType)
	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "my-uami", resources[0].Name)
	assert.Contains(t, resources[0].ID, "my-uami")
	assert.Equal(t, "true", resources[0].Tags["warpgate"])
}

// TestResourcesAdapter_ListByResourceGroup_Error propagates SDK errors.
func TestResourcesAdapter_ListByResourceGroup_Error(t *testing.T) {
	s := newAzTestServer(t)
	s.register("/subscriptions/sub-1/resourceGroups/rg-1/resources", 401,
		`{"error":{"code":"Unauthorized","message":"access denied"}}`)

	rc := s.resourcesClient(t)
	adapter := &resourcesAdapter{c: rc}

	_, err := adapter.ListByResourceGroup(context.Background(), "rg-1", uamiResourceType)
	require.Error(t, err)
}

// ----- NewDiscoverer with fake transport -----

// TestNewDiscoverer_WithFakeCred ensures NewDiscoverer constructs successfully
// when given a valid credential and subscription. The clients are constructed
// without contacting Azure; we only verify no error is returned.
func TestNewDiscoverer_WithFakeCred(t *testing.T) {
	d, err := NewDiscoverer(&fakeTokenCredential{}, "sub-1")
	require.NoError(t, err)
	require.NotNil(t, d)
}
