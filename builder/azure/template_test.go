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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/virtualmachineimagebuilder/armvirtualmachineimagebuilder"
	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fullTarget returns a ready-to-build azure target so tests can mutate just
// the field under exercise.
func fullTarget() *builder.Target {
	return &builder.Target{
		Type:                   "azure",
		SubscriptionID:         "sub-1",
		ResourceGroup:          "rg-1",
		Location:               "eastus",
		Gallery:                "myGallery",
		GalleryImageDefinition: "myDef",
		VMSize:                 "Standard_D2s_v3",
		OSType:                 "Linux",
		IdentityID:             "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.ManagedIdentity/userAssignedIdentities/uami",
		SourceImage: &builder.AzureSourceImage{
			Marketplace: &builder.AzureMarketplaceImage{
				Publisher: "Canonical",
				Offer:     "0001-com-ubuntu-server-jammy",
				SKU:       "22_04-lts-gen2",
				Version:   "latest",
			},
		},
	}
}

func TestBuildSource_Marketplace(t *testing.T) {
	target := fullTarget()
	src, err := buildSource(target)
	require.NoError(t, err)

	platform := mustBeType[*armvirtualmachineimagebuilder.ImageTemplatePlatformImageSource](t, src)
	assert.Equal(t, "PlatformImage", *platform.Type)
	assert.Equal(t, "Canonical", *platform.Publisher)
	assert.Equal(t, "0001-com-ubuntu-server-jammy", *platform.Offer)
	assert.Equal(t, "22_04-lts-gen2", *platform.SKU)
	assert.Equal(t, "latest", *platform.Version)
}

func TestBuildSource_MarketplaceDefaultsVersionToLatest(t *testing.T) {
	target := fullTarget()
	target.SourceImage.Marketplace.Version = ""

	src, err := buildSource(target)
	require.NoError(t, err)
	platform := mustBeType[*armvirtualmachineimagebuilder.ImageTemplatePlatformImageSource](t, src)
	assert.Equal(t, "latest", *platform.Version)
}

func TestBuildSource_GalleryVersion(t *testing.T) {
	target := fullTarget()
	target.SourceImage = &builder.AzureSourceImage{
		GalleryImageVersionID: "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/i/versions/1.0.0",
	}

	src, err := buildSource(target)
	require.NoError(t, err)
	gv := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateSharedImageVersionSource](t, src)
	assert.Equal(t, "SharedImageVersion", *gv.Type)
	assert.Equal(t, target.SourceImage.GalleryImageVersionID, *gv.ImageVersionID)
}

func TestBuildSource_MissingSource(t *testing.T) {
	target := fullTarget()
	target.SourceImage = nil

	_, err := buildSource(target)
	assert.Error(t, err)
}

func TestBuildSource_EmptySource(t *testing.T) {
	target := fullTarget()
	target.SourceImage = &builder.AzureSourceImage{}

	_, err := buildSource(target)
	assert.Error(t, err)
}

func TestBuildCustomizers_Shell(t *testing.T) {
	provs := []builder.Provisioner{
		{Type: "shell", Inline: []string{"echo hi", "uname -a"}},
	}
	cs, err := buildCustomizers(provs, "Linux", buildTemplateOpts{})
	require.NoError(t, err)
	require.Len(t, cs, 1)

	shell := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateShellCustomizer](t, cs[0])
	assert.Equal(t, "Shell", *shell.Type)
	assert.Equal(t, "shell-0", *shell.Name)
	require.Len(t, shell.Inline, 2)
	assert.Equal(t, "echo hi", *shell.Inline[0])
}

func TestBuildCustomizers_ShellIncludesEnvironment(t *testing.T) {
	provs := []builder.Provisioner{
		{
			Type:        "shell",
			Inline:      []string{"echo hi"},
			Environment: map[string]string{"B": "2", "A": "1"},
		},
	}

	cs, err := buildCustomizers(provs, "Linux", buildTemplateOpts{})
	require.NoError(t, err)

	shell := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateShellCustomizer](t, cs[0])
	require.Len(t, shell.Inline, 3)
	assert.Equal(t, "export A='1'", *shell.Inline[0])
	assert.Equal(t, "export B='2'", *shell.Inline[1])
	assert.Equal(t, "echo hi", *shell.Inline[2])
}

func TestBuildCustomizers_ShellRequiresInline(t *testing.T) {
	provs := []builder.Provisioner{{Type: "shell"}}
	_, err := buildCustomizers(provs, "Linux", buildTemplateOpts{})
	assert.Error(t, err)
}

func TestBuildCustomizers_PowerShellInline(t *testing.T) {
	provs := []builder.Provisioner{
		{Type: "powershell", Inline: []string{"Get-Service"}},
	}
	cs, err := buildCustomizers(provs, "Windows", buildTemplateOpts{})
	require.NoError(t, err)
	require.Len(t, cs, 1)

	ps := mustBeType[*armvirtualmachineimagebuilder.ImageTemplatePowerShellCustomizer](t, cs[0])
	assert.Equal(t, "PowerShell", *ps.Type)
	assert.Equal(t, "powershell-0", *ps.Name)
	assert.True(t, *ps.RunElevated)
}

func TestBuildCustomizers_PowerShellScripts(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "setup.ps1")
	require.NoError(t, os.WriteFile(scriptPath, []byte("Write-Host 'hello'\n"), 0o644))

	provs := []builder.Provisioner{
		{Type: "powershell", PSScripts: []string{scriptPath}, ExecutionPolicy: "RemoteSigned"},
	}
	cs, err := buildCustomizers(provs, "Windows", buildTemplateOpts{})
	require.NoError(t, err)

	ps := mustBeType[*armvirtualmachineimagebuilder.ImageTemplatePowerShellCustomizer](t, cs[0])
	require.GreaterOrEqual(t, len(ps.Inline), 6)
	assert.Equal(t, "$ErrorActionPreference = 'Stop'", *ps.Inline[0])
	assert.Equal(t, "Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope Process -Force", *ps.Inline[1])
	assert.Contains(t, *ps.Inline[4], "WriteAllBytes('C:\\warpgate-powershell\\00-setup.ps1'")
	assert.Equal(t, "& 'C:\\warpgate-powershell\\00-setup.ps1'", *ps.Inline[5])
}

func TestBuildCustomizers_File(t *testing.T) {
	provs := []builder.Provisioner{
		{Type: "file", Source: "https://example.com/script.sh", Destination: "/tmp/script.sh"},
	}
	cs, err := buildCustomizers(provs, "Linux", buildTemplateOpts{})
	require.NoError(t, err)
	require.Len(t, cs, 1)

	f := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateFileCustomizer](t, cs[0])
	assert.Equal(t, "File", *f.Type)
	assert.Equal(t, "file-0", *f.Name)
	assert.Equal(t, "https://example.com/script.sh", *f.SourceURI)
	assert.Equal(t, "/tmp/script.sh", *f.Destination)
}

func TestBuildCustomizers_FileRequiresSourceAndDestination(t *testing.T) {
	cases := []builder.Provisioner{
		{Type: "file", Destination: "/tmp/x"},
		{Type: "file", Source: "https://example.com/x"},
	}
	for _, p := range cases {
		_, err := buildCustomizers([]builder.Provisioner{p}, "Linux", buildTemplateOpts{})
		assert.Error(t, err, "provisioner %+v should error", p)
	}
}

func TestBuildCustomizers_Script(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "setup.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi\n"), 0o644))

	provs := []builder.Provisioner{
		{Type: "script", Scripts: []string{scriptPath}},
	}
	cs, err := buildCustomizers(provs, "Linux", buildTemplateOpts{})
	require.NoError(t, err)
	require.Len(t, cs, 1)

	shell := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateShellCustomizer](t, cs[0])
	assert.Equal(t, "script-0", *shell.Name)
	require.GreaterOrEqual(t, len(shell.Inline), 5)
	assert.Equal(t, "set -eu", *shell.Inline[0])
	assert.Equal(t, "mkdir -p /tmp/warpgate-scripts", *shell.Inline[1])
	assert.Contains(t, *shell.Inline[2], "base64 -d > '/tmp/warpgate-scripts/00-setup.sh'")
	assert.Equal(t, "chmod +x '/tmp/warpgate-scripts/00-setup.sh'", *shell.Inline[3])
	assert.Equal(t, "'/tmp/warpgate-scripts/00-setup.sh'", *shell.Inline[4])
}

func TestBuildCustomizers_ScriptRejectsWindows(t *testing.T) {
	provs := []builder.Provisioner{
		{Type: "script", Scripts: []string{"setup.sh"}},
	}
	_, err := buildCustomizers(provs, "Windows", buildTemplateOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use powershell")
}

func TestBuildCustomizers_UnsupportedTypes(t *testing.T) {
	for _, ty := range []string{"", "rocket"} {
		_, err := buildCustomizers([]builder.Provisioner{{Type: ty, Inline: []string{"x"}}}, "Linux", buildTemplateOpts{})
		assert.Errorf(t, err, "type=%q must error", ty)
	}
}

func TestBuildCustomizers_FileWithStagedSingleFile(t *testing.T) {
	provs := []builder.Provisioner{
		{Type: "file", Source: "scripts/install.sh", Destination: "/opt/install.sh"},
	}
	opts := buildTemplateOpts{
		StagedFiles: map[int]*StagedFile{
			0: {
				Account:     "acct",
				Container:   "ctr",
				IsDirectory: false,
				KeyPrefix:   "warpgate-staging/build-1/0",
				Entries:     []StagedEntry{{BlobName: "warpgate-staging/build-1/0/install.sh"}},
			},
		},
	}
	cs, err := buildCustomizers(provs, "Linux", opts)
	require.NoError(t, err)
	require.Len(t, cs, 1)

	f := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateFileCustomizer](t, cs[0])
	assert.Equal(t, "file-0", *f.Name)
	assert.Equal(t, "https://acct.blob.core.windows.net/ctr/warpgate-staging/build-1/0/install.sh", *f.SourceURI)
	assert.Equal(t, "/opt/install.sh", *f.Destination)
}

func TestBuildCustomizers_FileWithStagedDirectoryEmitsMultiple(t *testing.T) {
	provs := []builder.Provisioner{
		{Type: "file", Source: "scripts/", Destination: "/opt/scripts"},
	}
	opts := buildTemplateOpts{
		StagedFiles: map[int]*StagedFile{
			0: {
				Account:     "acct",
				Container:   "ctr",
				IsDirectory: true,
				KeyPrefix:   "warpgate-staging/build-1/0",
				Entries: []StagedEntry{
					{BlobName: "warpgate-staging/build-1/0/install.sh", RelPath: "install.sh"},
					{BlobName: "warpgate-staging/build-1/0/lib/util.sh", RelPath: "lib/util.sh"},
				},
			},
		},
	}
	cs, err := buildCustomizers(provs, "Linux", opts)
	require.NoError(t, err)
	require.Len(t, cs, 2, "directory uploads must emit one customizer per file")

	f0 := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateFileCustomizer](t, cs[0])
	assert.Equal(t, "file-0-0", *f0.Name)
	assert.Equal(t, "https://acct.blob.core.windows.net/ctr/warpgate-staging/build-1/0/install.sh", *f0.SourceURI)
	assert.Equal(t, "/opt/scripts/install.sh", *f0.Destination)

	f1 := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateFileCustomizer](t, cs[1])
	assert.Equal(t, "file-0-1", *f1.Name)
	assert.Equal(t, "https://acct.blob.core.windows.net/ctr/warpgate-staging/build-1/0/lib/util.sh", *f1.SourceURI)
	assert.Equal(t, "/opt/scripts/lib/util.sh", *f1.Destination)
}

func TestBuildCustomizers_PreservesOrder(t *testing.T) {
	provs := []builder.Provisioner{
		{Type: "shell", Inline: []string{"a"}},
		{Type: "powershell", Inline: []string{"b"}},
		{Type: "file", Source: "https://x/y", Destination: "/y"},
	}
	cs, err := buildCustomizers(provs, "Linux", buildTemplateOpts{})
	require.NoError(t, err)
	require.Len(t, cs, 3)
	mustBeType[*armvirtualmachineimagebuilder.ImageTemplateShellCustomizer](t, cs[0])
	mustBeType[*armvirtualmachineimagebuilder.ImageTemplatePowerShellCustomizer](t, cs[1])
	mustBeType[*armvirtualmachineimagebuilder.ImageTemplateFileCustomizer](t, cs[2])
}

func TestBuildCustomizers_FileModeAddsLinuxChmodStep(t *testing.T) {
	provs := []builder.Provisioner{
		{Type: "file", Source: "https://example.com/tool", Destination: "/usr/local/bin/tool", Mode: "0755"},
	}

	cs, err := buildCustomizers(provs, "Linux", buildTemplateOpts{})
	require.NoError(t, err)
	require.Len(t, cs, 2)

	file := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateFileCustomizer](t, cs[0])
	assert.Equal(t, "/usr/local/bin/tool", *file.Destination)

	mode := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateShellCustomizer](t, cs[1])
	require.Len(t, mode.Inline, 1)
	assert.Equal(t, "chmod 0755 '/usr/local/bin/tool'", *mode.Inline[0])
}

func TestBuildCustomizers_FileModeUsesRecursiveChmodForDirectories(t *testing.T) {
	provs := []builder.Provisioner{
		{Type: "file", Source: "scripts", Destination: "/opt/scripts", Mode: "0750"},
	}
	opts := buildTemplateOpts{
		StagedFiles: map[int]*StagedFile{
			0: {
				Account:     "acct",
				Container:   "ctr",
				IsDirectory: true,
				KeyPrefix:   "warpgate-staging/build-1/0",
				Entries:     []StagedEntry{{BlobName: "warpgate-staging/build-1/0/install.sh", RelPath: "install.sh"}},
			},
		},
	}

	cs, err := buildCustomizers(provs, "Linux", opts)
	require.NoError(t, err)
	require.Len(t, cs, 2)

	mode := mustBeType[*armvirtualmachineimagebuilder.ImageTemplateShellCustomizer](t, cs[1])
	assert.Equal(t, "chmod -R 0750 '/opt/scripts'", *mode.Inline[0])
}

func TestBuildCustomizers_FileModeIgnoredForWindows(t *testing.T) {
	provs := []builder.Provisioner{
		{Type: "file", Source: "https://example.com/tool", Destination: "C:\\tool.exe", Mode: "0755"},
	}

	cs, err := buildCustomizers(provs, "Windows", buildTemplateOpts{})
	require.NoError(t, err)
	require.Len(t, cs, 1)
	mustBeType[*armvirtualmachineimagebuilder.ImageTemplateFileCustomizer](t, cs[0])
}

func TestBuildSharedImageDistributor_BasicLayout(t *testing.T) {
	target := fullTarget()
	dist := buildSharedImageDistributor(target, "2026.0429.130509")

	assert.Equal(t, "SharedImage", *dist.Type)
	assert.Equal(t, runOutputName, *dist.RunOutputName)

	wantID := "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/myGallery/images/myDef/versions/2026.0429.130509"
	assert.Equal(t, wantID, *dist.GalleryImageID)

	require.Len(t, dist.ReplicationRegions, 1)
	assert.Equal(t, "eastus", *dist.ReplicationRegions[0])
	assert.Empty(t, dist.ArtifactTags)
}

func TestBuildSharedImageDistributor_AddsTargetRegions(t *testing.T) {
	target := fullTarget()
	target.TargetRegions = []string{"westus2", "eastus"} // eastus dup'd, should drop
	dist := buildSharedImageDistributor(target, "v1")

	got := []string{}
	for _, r := range dist.ReplicationRegions {
		got = append(got, *r)
	}
	assert.Equal(t, []string{"eastus", "westus2"}, got)
}

func TestBuildSharedImageDistributor_TagsPropagate(t *testing.T) {
	target := fullTarget()
	target.ImageTags = map[string]string{"env": "prod", "owner": "redteam"}
	dist := buildSharedImageDistributor(target, "v1")

	require.Len(t, dist.ArtifactTags, 2)
	assert.Equal(t, "prod", *dist.ArtifactTags["env"])
	assert.Equal(t, "redteam", *dist.ArtifactTags["owner"])
}

func TestBuildImageTemplate_FullShape(t *testing.T) {
	cfg := builder.Config{
		Name: "Attack-Box",
		Provisioners: []builder.Provisioner{
			{Type: "shell", Inline: []string{"apt-get update"}},
		},
	}
	target := fullTarget()
	target.StagingResourceGroup = "rg-staging"
	target.ImageTags = map[string]string{"k": "v"}

	tpl, err := buildImageTemplate(cfg, target, "2026.0429.130509", 90, buildTemplateOpts{})
	require.NoError(t, err)

	assert.Equal(t, "eastus", *tpl.Location)
	require.NotNil(t, tpl.Properties)
	assert.Equal(t, int32(90), *tpl.Properties.BuildTimeoutInMinutes)
	assert.Equal(t, "Standard_D2s_v3", *tpl.Properties.VMProfile.VMSize)
	assert.Equal(t, "rg-staging", *tpl.Properties.StagingResourceGroup)

	require.NotNil(t, tpl.Properties.Source)
	require.Len(t, tpl.Properties.Customize, 1)
	require.Len(t, tpl.Properties.Distribute, 1)

	require.NotNil(t, tpl.Identity)
	assert.Equal(t, armvirtualmachineimagebuilder.ResourceIdentityTypeUserAssigned, *tpl.Identity.Type)
	_, hasUAMI := tpl.Identity.UserAssignedIdentities[target.IdentityID]
	assert.True(t, hasUAMI, "expected UAMI key %q in UserAssignedIdentities", target.IdentityID)

	require.Len(t, tpl.Tags, 1)
	assert.Equal(t, "v", *tpl.Tags["k"])
}

func TestBuildImageTemplate_VnetConfigUnsetByDefault(t *testing.T) {
	cfg := builder.Config{Name: "x", Provisioners: []builder.Provisioner{{Type: "shell", Inline: []string{"true"}}}}
	tpl, err := buildImageTemplate(cfg, fullTarget(), "v1", 60, buildTemplateOpts{})
	require.NoError(t, err)
	assert.Nil(t, tpl.Properties.VMProfile.VnetConfig, "VnetConfig must be nil when SubnetID is empty")
}

func TestBuildImageTemplate_VnetConfigPopulatedFromSubnet(t *testing.T) {
	cfg := builder.Config{Name: "x", Provisioners: []builder.Provisioner{{Type: "shell", Inline: []string{"true"}}}}
	target := fullTarget()
	target.SubnetID = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/build"
	tpl, err := buildImageTemplate(cfg, target, "v1", 60, buildTemplateOpts{})
	require.NoError(t, err)
	require.NotNil(t, tpl.Properties.VMProfile.VnetConfig)
	assert.Equal(t, target.SubnetID, *tpl.Properties.VMProfile.VnetConfig.SubnetID)
	assert.Nil(t, tpl.Properties.VMProfile.VnetConfig.ProxyVMSize, "ProxyVMSize must stay nil when not set")
}

func TestBuildImageTemplate_VnetConfigProxySize(t *testing.T) {
	cfg := builder.Config{Name: "x", Provisioners: []builder.Provisioner{{Type: "shell", Inline: []string{"true"}}}}
	target := fullTarget()
	target.SubnetID = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/build"
	target.ProxyVMSize = "Standard_A2_v2"
	tpl, err := buildImageTemplate(cfg, target, "v1", 60, buildTemplateOpts{})
	require.NoError(t, err)
	require.NotNil(t, tpl.Properties.VMProfile.VnetConfig.ProxyVMSize)
	assert.Equal(t, "Standard_A2_v2", *tpl.Properties.VMProfile.VnetConfig.ProxyVMSize)
}

func TestBuildImageTemplate_PropagatesProvisionerError(t *testing.T) {
	cfg := builder.Config{
		Name:         "x",
		Provisioners: []builder.Provisioner{{Type: "ansible"}},
	}
	target := fullTarget()
	_, err := buildImageTemplate(cfg, target, "v1", 60, buildTemplateOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ansible")
}

func TestBuildImageTemplate_PropagatesSourceError(t *testing.T) {
	cfg := builder.Config{Name: "x"}
	target := fullTarget()
	target.SourceImage = nil
	_, err := buildImageTemplate(cfg, target, "v1", 60, buildTemplateOpts{})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "source_image")
}

func TestPowerShellCustomizer_RequiresScriptsOrInline(t *testing.T) {
	_, err := powerShellCustomizer(&builder.Provisioner{Type: "powershell"}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ps_scripts or inline commands are required")
}

func TestScriptCustomizer_RequiresScripts(t *testing.T) {
	_, err := scriptCustomizer(&builder.Provisioner{Type: "script"}, 0, "Linux")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scripts are required")
}

func TestScriptCustomizer_ReadError(t *testing.T) {
	_, err := scriptCustomizer(&builder.Provisioner{
		Type:    "script",
		Scripts: []string{"/nonexistent/script.sh"},
	}, 0, "Linux")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read script")
}

func TestPowerShellCommandsFromScripts_ReadError(t *testing.T) {
	_, err := powerShellCommandsFromScripts(&builder.Provisioner{
		Type:      "powershell",
		PSScripts: []string{"/nonexistent/script.ps1"},
	}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read script")
}

func TestNormalizePowerShellScript_StripsBOM(t *testing.T) {
	// UTF-8 BOM (EF BB BF) followed by content should be stripped.
	withBOM := []byte{0xEF, 0xBB, 0xBF, 'W', 'r', 'i', 't', 'e'}
	got := normalizePowerShellScript(withBOM)
	assert.Equal(t, []byte("Write"), got)

	// Content without a BOM passes through unchanged.
	noBOM := []byte("Get-Service")
	assert.Equal(t, noBOM, normalizePowerShellScript(noBOM))

	// Short content stays intact even if it might look BOM-like.
	short := []byte{0xEF, 0xBB}
	assert.Equal(t, short, normalizePowerShellScript(short))
}
