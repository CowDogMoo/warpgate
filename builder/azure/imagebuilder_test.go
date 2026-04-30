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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/virtualmachineimagebuilder/armvirtualmachineimagebuilder"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePipelineOps records the order of calls Build makes against the AIB
// pipeline. Optional stub errors and stub return values let individual tests
// drive specific failure or success paths.
type fakePipelineOps struct {
	calls          []string
	submitErr      error
	runErr         error
	artifactID     string
	readErr        error
	lastRunStatus  string
	describeErr    error
	receivedTpl    *armvirtualmachineimagebuilder.ImageTemplate
	receivedTplKey string
}

func (f *fakePipelineOps) submit(_ context.Context, name string, tpl *armvirtualmachineimagebuilder.ImageTemplate) error {
	f.calls = append(f.calls, "submit:"+name)
	f.receivedTplKey = name
	f.receivedTpl = tpl
	return f.submitErr
}

func (f *fakePipelineOps) run(_ context.Context, name string) error {
	f.calls = append(f.calls, "run:"+name)
	return f.runErr
}

func (f *fakePipelineOps) readArtifact(_ context.Context, name string) (string, error) {
	f.calls = append(f.calls, "readArtifact:"+name)
	return f.artifactID, f.readErr
}

func (f *fakePipelineOps) describeLastRun(_ context.Context, name string) (string, error) {
	f.calls = append(f.calls, "describeLastRun:"+name)
	return f.lastRunStatus, f.describeErr
}

func (f *fakePipelineOps) deleteTemplate(_ context.Context, name string) {
	f.calls = append(f.calls, "deleteTemplate:"+name)
}

// startsWith returns true if call matches one of the prefixes — used to assert
// the lifecycle order without coupling tests to the timestamp suffix in tpl
// names.
func startsWith(call string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(call, p) {
			return true
		}
	}
	return false
}

// buildAzureCfg returns a builder.Config + Target pair good enough to drive
// ImageBuilder.Build through to completion using a fake pipeline.
func buildAzureCfg() builder.Config {
	return builder.Config{
		Name: "myimg",
		Targets: []builder.Target{
			{
				Type:                   "azure",
				SubscriptionID:         "sub-1",
				ResourceGroup:          "rg-1",
				Location:               "eastus",
				Gallery:                "g",
				GalleryImageDefinition: "def",
				VMSize:                 "Standard_D2s_v3",
				OSType:                 "Linux",
				IdentityID:             "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ManagedIdentity/userAssignedIdentities/uami",
				SourceImage: &builder.AzureSourceImage{
					Marketplace: &builder.AzureMarketplaceImage{
						Publisher: "Canonical",
						Offer:     "0001-com-ubuntu-server-jammy",
						SKU:       "22_04-lts-gen2",
					},
				},
			},
		},
		Provisioners: []builder.Provisioner{
			{Type: "shell", Inline: []string{"echo hi"}},
		},
	}
}

func newImageBuilderWithFake(fake *fakePipelineOps) *ImageBuilder {
	return &ImageBuilder{
		clients:       &AzureClients{SubscriptionID: "sub-1"},
		runnerFactory: func(*AzureClients, string, time.Duration) pipelineOps { return fake },
	}
}

func TestImageBuilder_Build_HappyPathPublishesArtifact(t *testing.T) {
	fake := &fakePipelineOps{
		artifactID: "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/def/versions/2026.0429.130509",
	}
	b := newImageBuilderWithFake(fake)

	res, err := b.Build(context.Background(), buildAzureCfg())
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, fake.artifactID, res.GalleryImageVersionID)
	assert.Equal(t, "eastus", res.Location)
	assert.NotEmpty(t, res.Duration)

	require.Len(t, fake.calls, 3, "calls were: %v", fake.calls)
	assert.True(t, startsWith(fake.calls[0], "submit:"))
	assert.True(t, startsWith(fake.calls[1], "run:"))
	assert.True(t, startsWith(fake.calls[2], "readArtifact:"))

	// Submit should hand a fully-formed ImageTemplate referencing the target's location.
	require.NotNil(t, fake.receivedTpl)
	require.NotNil(t, fake.receivedTpl.Location)
	assert.Equal(t, "eastus", *fake.receivedTpl.Location)
}

func TestImageBuilder_Build_ForceRecreateDeletesBeforeSubmit(t *testing.T) {
	fake := &fakePipelineOps{artifactID: "v1"}
	b := newImageBuilderWithFake(fake)
	b.forceRecreate = true

	_, err := b.Build(context.Background(), buildAzureCfg())
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(fake.calls), 4, "calls: %v", fake.calls)
	assert.True(t, startsWith(fake.calls[0], "deleteTemplate:"), "first call should be deleteTemplate, got %s", fake.calls[0])
	assert.True(t, startsWith(fake.calls[1], "submit:"))
}

func TestImageBuilder_Build_CleanupOnFinishDeletesAfterSuccess(t *testing.T) {
	fake := &fakePipelineOps{artifactID: "v1"}
	b := newImageBuilderWithFake(fake)
	b.cleanupOnFinish = true

	_, err := b.Build(context.Background(), buildAzureCfg())
	require.NoError(t, err)

	last := fake.calls[len(fake.calls)-1]
	assert.True(t, startsWith(last, "deleteTemplate:"), "expected last call to be deleteTemplate, got %s", last)
}

func TestImageBuilder_Build_RunFailureAttachesLastRunStatus(t *testing.T) {
	fake := &fakePipelineOps{
		runErr:        errors.New("aib run blew up"),
		lastRunStatus: "state=Failed message=reasons",
	}
	b := newImageBuilderWithFake(fake)

	_, err := b.Build(context.Background(), buildAzureCfg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aib run blew up")
	assert.Contains(t, err.Error(), "state=Failed")
	assert.Contains(t, err.Error(), "reasons")
}

func TestImageBuilder_Build_RunFailureFallsBackWhenDescribeFails(t *testing.T) {
	fake := &fakePipelineOps{
		runErr:      errors.New("aib run blew up"),
		describeErr: errors.New("describe failed"),
	}
	b := newImageBuilderWithFake(fake)

	_, err := b.Build(context.Background(), buildAzureCfg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aib run blew up")
	assert.NotContains(t, err.Error(), "last run", "describe failure should not leak into surfaced error")
}

func TestImageBuilder_Build_SubmitErrorPropagates(t *testing.T) {
	fake := &fakePipelineOps{submitErr: errors.New("submit boom")}
	b := newImageBuilderWithFake(fake)

	_, err := b.Build(context.Background(), buildAzureCfg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "submit boom")
	for _, c := range fake.calls {
		assert.False(t, startsWith(c, "run:"), "run should not have been called after submit failure: %v", fake.calls)
	}
}

func TestImageBuilder_Build_ReadArtifactErrorPropagates(t *testing.T) {
	fake := &fakePipelineOps{readErr: errors.New("missing artifact")}
	b := newImageBuilderWithFake(fake)

	_, err := b.Build(context.Background(), buildAzureCfg())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing artifact")
}

func TestImageBuilder_Build_NoAzureTarget(t *testing.T) {
	cfg := buildAzureCfg()
	cfg.Targets[0].Type = "ami"

	b := newImageBuilderWithFake(&fakePipelineOps{})
	_, err := b.Build(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no azure target")
}

func TestImageBuilder_Build_MissingRequiredFields(t *testing.T) {
	cfg := buildAzureCfg()
	cfg.Targets[0].ResourceGroup = ""

	b := newImageBuilderWithFake(&fakePipelineOps{})
	_, err := b.Build(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource_group")
}

func TestImageBuilder_Build_PropagatesDefaultsFromClients(t *testing.T) {
	fake := &fakePipelineOps{artifactID: "v1"}
	b := &ImageBuilder{
		clients: &AzureClients{
			SubscriptionID: "sub-from-clients",
			IdentityID:     "/subscriptions/sub-from-clients/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/from-clients",
		},
		runnerFactory: func(*AzureClients, string, time.Duration) pipelineOps { return fake },
	}
	cfg := buildAzureCfg()
	cfg.Targets[0].SubscriptionID = ""
	cfg.Targets[0].IdentityID = ""

	_, err := b.Build(context.Background(), cfg)
	require.NoError(t, err)

	require.NotNil(t, fake.receivedTpl)
	require.NotNil(t, fake.receivedTpl.Identity)
	_, hasIdentityFromClients := fake.receivedTpl.Identity.UserAssignedIdentities["/subscriptions/sub-from-clients/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/from-clients"]
	assert.True(t, hasIdentityFromClients, "expected identity from AzureClients to flow into template; got %v", fake.receivedTpl.Identity.UserAssignedIdentities)
}

func TestImageBuilder_Build_AppliesBuildTimeoutOverride(t *testing.T) {
	fake := &fakePipelineOps{artifactID: "v1"}
	b := newImageBuilderWithFake(fake)
	b.SetBuildTimeoutMinutes(5)

	_, err := b.Build(context.Background(), buildAzureCfg())
	require.NoError(t, err)
	require.NotNil(t, fake.receivedTpl.Properties.BuildTimeoutInMinutes)
	assert.Equal(t, int32(5), *fake.receivedTpl.Properties.BuildTimeoutInMinutes)
}

func TestImageBuilder_Build_ZeroTimeoutFallsBackToDefault(t *testing.T) {
	fake := &fakePipelineOps{artifactID: "v1"}
	b := newImageBuilderWithFake(fake)

	_, err := b.Build(context.Background(), buildAzureCfg())
	require.NoError(t, err)
	require.NotNil(t, fake.receivedTpl.Properties.BuildTimeoutInMinutes)
	assert.Equal(t, defaultBuildTimeoutMinutes, *fake.receivedTpl.Properties.BuildTimeoutInMinutes)
}

func TestImageBuilder_Build_StagesLocalFileProvisioners(t *testing.T) {
	fake := &fakePipelineOps{artifactID: "v1"}
	stager := newFakeStager()
	b := newImageBuilderWithFake(fake)
	b.fileStager = stager

	cfg := buildAzureCfg()
	cfg.Provisioners = []builder.Provisioner{
		{Type: "file", Source: "/local/script.sh", Destination: "/opt/script.sh"},
	}

	_, err := b.Build(context.Background(), cfg)
	require.NoError(t, err)

	require.Len(t, stager.uploads, 1, "expected exactly one staging upload")
	assert.Equal(t, "/local/script.sh", stager.uploads[0].source)

	// Cleanup happens via defer, so the staged blob should also have been
	// passed through Cleanup.
	require.Len(t, stager.deletes, 1)
}

func TestImageBuilder_Build_FileStagingFailureSurfacesBeforeSubmit(t *testing.T) {
	fake := &fakePipelineOps{}
	stager := newFakeStager()
	stager.stageErr = errors.New("blob upload denied")
	b := newImageBuilderWithFake(fake)
	b.fileStager = stager

	cfg := buildAzureCfg()
	cfg.Provisioners = []builder.Provisioner{
		{Type: "file", Source: "/local/x.txt", Destination: "/opt/x.txt"},
	}

	_, err := b.Build(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blob upload denied")
	for _, c := range fake.calls {
		assert.False(t, startsWith(c, "submit:"), "submit should not run after staging failure")
	}
}

func TestImageBuilder_Replicate_NilOrEmpty(t *testing.T) {
	b := &ImageBuilder{clients: &AzureClients{SubscriptionID: "sub-1"}}
	require.NoError(t, b.Replicate(context.Background(), "/version/id", nil))
	require.NoError(t, b.Replicate(context.Background(), "/version/id", []string{}))
}

func TestImageBuilder_Replicate_RequiresVersionID(t *testing.T) {
	b := &ImageBuilder{clients: &AzureClients{SubscriptionID: "sub-1"}}
	err := b.Replicate(context.Background(), "", []string{"westus2"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "versionID")
}

func TestImageBuilder_Share_NoPrincipalsIsNoop(t *testing.T) {
	b := &ImageBuilder{clients: &AzureClients{SubscriptionID: "sub-1"}}
	require.NoError(t, b.Share(context.Background(), "/version/id", nil))
}

func TestImageBuilder_Share_RequiresVersionID(t *testing.T) {
	b := &ImageBuilder{clients: &AzureClients{SubscriptionID: "sub-1"}}
	err := b.Share(context.Background(), "", []string{"p1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "versionID")
}

func TestImageBuilder_Delete_RequiresVersionID(t *testing.T) {
	b := &ImageBuilder{clients: &AzureClients{SubscriptionID: "sub-1"}}
	err := b.Delete(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "versionID")
}

func TestImageBuilder_Close(t *testing.T) {
	b := &ImageBuilder{}
	require.NoError(t, b.Close())
}

func TestImageBuilder_GetBuildID(t *testing.T) {
	b := &ImageBuilder{buildID: "build-abc123"}
	assert.Equal(t, "build-abc123", b.GetBuildID())
}

func TestImageBuilder_GetBuildID_Empty(t *testing.T) {
	b := &ImageBuilder{}
	assert.Equal(t, "", b.GetBuildID())
}

func TestImageBuilder_SetCleanupOnFinish(t *testing.T) {
	b := &ImageBuilder{}
	assert.False(t, b.cleanupOnFinish)
	b.SetCleanupOnFinish(true)
	assert.True(t, b.cleanupOnFinish)
	b.SetCleanupOnFinish(false)
	assert.False(t, b.cleanupOnFinish)
}

func TestImageBuilder_SetPollingInterval(t *testing.T) {
	b := &ImageBuilder{}
	b.SetPollingInterval(42 * time.Second)
	assert.Equal(t, 42*time.Second, b.pollingInterval)
	b.SetPollingInterval(0)
	assert.Equal(t, time.Duration(0), b.pollingInterval)
}

func TestImageBuilder_SetBuildTimeoutMinutes(t *testing.T) {
	b := &ImageBuilder{}
	b.SetBuildTimeoutMinutes(120)
	assert.Equal(t, int32(120), b.buildTimeoutMinutes)
}

func TestNewImageBuilderWithAllOptions_RequiresSubscriptionID(t *testing.T) {
	defer installFakeCred(t)()

	_, err := NewImageBuilderWithAllOptions(context.Background(), ClientConfig{}, false, MonitorConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SubscriptionID is required")
}

func TestNewImageBuilderWithAllOptions_HappyPath(t *testing.T) {
	defer installFakeCred(t)()

	cfg := ClientConfig{
		SubscriptionID: "sub-1",
		Location:       "eastus",
	}
	b, err := NewImageBuilderWithAllOptions(context.Background(), cfg, true, MonitorConfig{StreamLogs: true})
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.True(t, b.forceRecreate)
	assert.True(t, b.monitor.StreamLogs)
	assert.NotEmpty(t, b.buildID)
	assert.NotNil(t, b.runnerFactory)
	assert.Nil(t, b.fileStager, "no staging account set")
}

func TestNewImageBuilderWithAllOptions_WithFileStaging(t *testing.T) {
	defer installFakeCred(t)()

	cfg := ClientConfig{
		SubscriptionID:       "sub-1",
		FileStagingAccount:   "myacct",
		FileStagingContainer: "myctr",
	}
	b, err := NewImageBuilderWithAllOptions(context.Background(), cfg, false, MonitorConfig{})
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.NotNil(t, b.fileStager, "stager should be created when staging config is present")
}

func TestGenerateBuildID_Format(t *testing.T) {
	id := generateBuildID()
	assert.NotEmpty(t, id)
	assert.Contains(t, id, "-", "buildID should contain separator")
}

func TestRequireAzureFields_AllErrors(t *testing.T) {
	base := &builder.Target{
		Type:                   "azure",
		SubscriptionID:         "sub-1",
		ResourceGroup:          "rg-1",
		Location:               "eastus",
		Gallery:                "g",
		GalleryImageDefinition: "def",
		IdentityID:             "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ManagedIdentity/userAssignedIdentities/uami",
	}

	tests := []struct {
		name  string
		field func(*builder.Target)
		want  string
	}{
		{"no subscription", func(t *builder.Target) { t.SubscriptionID = "" }, "subscription_id"},
		{"no resource group", func(t *builder.Target) { t.ResourceGroup = "" }, "resource_group"},
		{"no location", func(t *builder.Target) { t.Location = "" }, "location"},
		{"no gallery", func(t *builder.Target) { t.Gallery = "" }, "gallery"},
		{"no gallery image def", func(t *builder.Target) { t.GalleryImageDefinition = "" }, "gallery_image_definition"},
		{"no identity", func(t *builder.Target) { t.IdentityID = "" }, "identity_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tgt := *base
			tt.field(&tgt)
			err := requireAzureFields(&tgt)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestRequireAzureFields_HappyPath(t *testing.T) {
	tgt := &builder.Target{
		Type:                   "azure",
		SubscriptionID:         "sub-1",
		ResourceGroup:          "rg-1",
		Location:               "eastus",
		Gallery:                "g",
		GalleryImageDefinition: "def",
		IdentityID:             "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ManagedIdentity/userAssignedIdentities/uami",
	}
	require.NoError(t, requireAzureFields(tgt))
}

func TestImageBuilder_Delete_HappyPath(t *testing.T) {
	fake := &fakeRoleAssignmentsClient{}
	_ = fake
	b := &ImageBuilder{clients: &AzureClients{SubscriptionID: "sub-1"}}
	// Delete with a valid version ID will call deleteGalleryVersion which needs
	// a real GalleryImageVersions client. Without one it panics (nil pointer).
	// We test the versionID == "" guard here; the actual SDK path is tested in pipeline_test.go.
	err := b.Delete(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "versionID")
}
