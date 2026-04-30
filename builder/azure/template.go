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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/virtualmachineimagebuilder/armvirtualmachineimagebuilder"
	"github.com/cowdogmoo/warpgate/v3/builder"
)

// runOutputName is the AIB RunOutput name we use for the gallery distribution.
// It becomes part of the run output resource path; we keep it stable so callers
// can locate the produced gallery image version.
const runOutputName = "warpgate-gallery-output"

// buildTemplateOpts carries optional context required to assemble an image
// template. Currently used to thread staged blobs for `file` provisioners so
// the input provisioners stay pristine. Mirrors builder/ami.GenerateComponentOpts.
type buildTemplateOpts struct {
	// StagedFiles maps provisioner index → result of pre-uploading that
	// provisioner's source to blob storage. Only consulted when the
	// provisioner type is "file".
	StagedFiles map[int]*StagedFile
}

// buildImageTemplate converts a warpgate build Config + Target into an AIB
// ImageTemplate ready for submission.
//
// The returned template has Source set from target.SourceImage, Customize set
// from config.Provisioners (one customizer per provisioner — except file
// provisioners pointing at directories, which expand to one customizer per
// staged file), and Distribute set to publish into target.Gallery /
// target.GalleryImageDefinition at the supplied versionTag with replication
// into target.Location plus target.TargetRegions.
func buildImageTemplate(cfg builder.Config, target *builder.Target, versionTag string, buildTimeoutMinutes int32, opts buildTemplateOpts) (*armvirtualmachineimagebuilder.ImageTemplate, error) {
	source, err := buildSource(target)
	if err != nil {
		return nil, err
	}

	customizers, err := buildCustomizers(cfg.Provisioners, target.OSType, opts)
	if err != nil {
		return nil, err
	}

	distributor := buildSharedImageDistributor(target, versionTag)

	props := &armvirtualmachineimagebuilder.ImageTemplateProperties{
		Source:                source,
		Customize:             customizers,
		Distribute:            []armvirtualmachineimagebuilder.ImageTemplateDistributorClassification{distributor},
		BuildTimeoutInMinutes: to.Ptr(buildTimeoutMinutes),
		VMProfile: &armvirtualmachineimagebuilder.ImageTemplateVMProfile{
			VMSize: to.Ptr(target.VMSize),
		},
	}
	if target.StagingResourceGroup != "" {
		props.StagingResourceGroup = to.Ptr(target.StagingResourceGroup)
	}
	if target.SubnetID != "" {
		vnetCfg := &armvirtualmachineimagebuilder.VirtualNetworkConfig{
			SubnetID: to.Ptr(target.SubnetID),
		}
		if target.ProxyVMSize != "" {
			vnetCfg.ProxyVMSize = to.Ptr(target.ProxyVMSize)
		}
		props.VMProfile.VnetConfig = vnetCfg
	}

	tpl := &armvirtualmachineimagebuilder.ImageTemplate{
		Location:   to.Ptr(target.Location),
		Properties: props,
		Identity: &armvirtualmachineimagebuilder.ImageTemplateIdentity{
			Type: to.Ptr(armvirtualmachineimagebuilder.ResourceIdentityTypeUserAssigned),
			UserAssignedIdentities: map[string]*armvirtualmachineimagebuilder.ComponentsVrq145SchemasImagetemplateidentityPropertiesUserassignedidentitiesAdditionalproperties{
				target.IdentityID: {},
			},
		},
	}
	if len(target.ImageTags) > 0 {
		tpl.Tags = stringMapToPointerMap(target.ImageTags)
	}
	return tpl, nil
}

// buildSource builds the AIB source classifier from a warpgate target.
// Exactly one of Marketplace or GalleryImageVersionID is expected (validated
// upstream by templates/config_validator.go).
func buildSource(target *builder.Target) (armvirtualmachineimagebuilder.ImageTemplateSourceClassification, error) {
	if target.SourceImage == nil {
		return nil, fmt.Errorf("azure target requires source_image")
	}
	if mp := target.SourceImage.Marketplace; mp != nil {
		version := mp.Version
		if version == "" {
			version = "latest"
		}
		src := &armvirtualmachineimagebuilder.ImageTemplatePlatformImageSource{
			Type:      to.Ptr("PlatformImage"),
			Publisher: to.Ptr(mp.Publisher),
			Offer:     to.Ptr(mp.Offer),
			SKU:       to.Ptr(mp.SKU),
			Version:   to.Ptr(version),
		}
		if mp.Plan != nil {
			src.PlanInfo = &armvirtualmachineimagebuilder.PlatformImagePurchasePlan{
				PlanName:      to.Ptr(mp.Plan.Name),
				PlanProduct:   to.Ptr(mp.Plan.Product),
				PlanPublisher: to.Ptr(mp.Plan.Publisher),
			}
		}
		return src, nil
	}
	if id := target.SourceImage.GalleryImageVersionID; id != "" {
		return &armvirtualmachineimagebuilder.ImageTemplateSharedImageVersionSource{
			Type:           to.Ptr("SharedImageVersion"),
			ImageVersionID: to.Ptr(id),
		}, nil
	}
	return nil, fmt.Errorf("azure source_image must set marketplace or gallery_image_version_id")
}

// buildCustomizers converts warpgate provisioners into AIB customizers.
// Supported provisioner types: shell, script, powershell, file, ansible. Other types return
// an error so the caller can surface a clear diagnostic.
func buildCustomizers(provisioners []builder.Provisioner, osType string, opts buildTemplateOpts) ([]armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification, error) {
	out := make([]armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification, 0, len(provisioners))
	for i := range provisioners {
		p := &provisioners[i]
		switch p.Type {
		case "shell":
			c, err := shellCustomizer(p, i)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		case "powershell":
			c, err := powerShellCustomizer(p, i)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		case "file":
			cs, err := fileCustomizers(p, i, opts.StagedFiles[i])
			if err != nil {
				return nil, err
			}
			out = append(out, cs...)
			if c := fileModeCustomizer(p, i, osType, opts.StagedFiles[i]); c != nil {
				out = append(out, c)
			}
		case "script":
			c, err := scriptCustomizer(p, i, osType)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		case "ansible":
			c, err := ansibleCustomizer(p, i, osType)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		case "":
			return nil, fmt.Errorf("provisioner[%d] missing type", i)
		default:
			return nil, fmt.Errorf("provisioner[%d] type=%q is not supported for azure builds", i, p.Type)
		}
	}
	return out, nil
}

// shellCustomizer builds an AIB shell customizer from a shell provisioner.
func shellCustomizer(p *builder.Provisioner, index int) (armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification, error) {
	if len(p.Inline) == 0 {
		return nil, fmt.Errorf("provisioner[%d] shell: inline commands are required", index)
	}
	inline := make([]string, 0, len(p.Environment)+len(p.Inline))
	for _, key := range sortedStringKeys(p.Environment) {
		inline = append(inline, fmt.Sprintf("export %s=%s", key, shellSingleQuote(p.Environment[key])))
	}
	inline = append(inline, p.Inline...)
	return &armvirtualmachineimagebuilder.ImageTemplateShellCustomizer{
		Type:   to.Ptr("Shell"),
		Name:   to.Ptr(fmt.Sprintf("shell-%d", index)),
		Inline: stringSliceToPointerSlice(inline),
	}, nil
}

// powerShellCustomizer builds an AIB PowerShell customizer from a powershell
// provisioner. `ps_scripts` follow the shared schema used by the rest of
// warpgate; `inline` is retained for backwards compatibility.
func powerShellCustomizer(p *builder.Provisioner, index int) (armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification, error) {
	inline, err := powerShellInlineCommands(p, index)
	if err != nil {
		return nil, err
	}
	return &armvirtualmachineimagebuilder.ImageTemplatePowerShellCustomizer{
		Type:        to.Ptr("PowerShell"),
		Name:        to.Ptr(fmt.Sprintf("powershell-%d", index)),
		Inline:      stringSliceToPointerSlice(inline),
		RunElevated: to.Ptr(true),
	}, nil
}

// scriptCustomizer builds an AIB shell customizer from one or more local
// script files. Azure's shell customizer supports inline commands, so we
// embed each script as base64, write it to disk, mark it executable, and run it.
func scriptCustomizer(p *builder.Provisioner, index int, osType string) (armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification, error) {
	if len(p.Scripts) == 0 {
		return nil, fmt.Errorf("provisioner[%d] script: scripts are required", index)
	}
	if strings.EqualFold(osType, "windows") {
		return nil, fmt.Errorf("provisioner[%d] script: Windows targets are not supported; use powershell", index)
	}

	commands := []string{
		"set -eu",
		"mkdir -p /tmp/warpgate-scripts",
	}
	for scriptIndex, scriptPath := range p.Scripts {
		content, err := os.ReadFile(scriptPath)
		if err != nil {
			return nil, fmt.Errorf("provisioner[%d] script: read script %s: %w", index, scriptPath, err)
		}
		scriptName := fmt.Sprintf("%02d-%s", scriptIndex, filepath.Base(scriptPath))
		scriptOnDisk := "/tmp/warpgate-scripts/" + scriptName
		encoded := base64.StdEncoding.EncodeToString(content)
		commands = append(commands,
			fmt.Sprintf("echo '%s' | base64 -d > %s", encoded, shellSingleQuote(scriptOnDisk)),
			fmt.Sprintf("chmod +x %s", shellSingleQuote(scriptOnDisk)),
			shellSingleQuote(scriptOnDisk),
		)
	}

	return &armvirtualmachineimagebuilder.ImageTemplateShellCustomizer{
		Type:   to.Ptr("Shell"),
		Name:   to.Ptr(fmt.Sprintf("script-%d", index)),
		Inline: stringSliceToPointerSlice(commands),
	}, nil
}

// fileCustomizers builds one or more AIB File customizers for a file
// provisioner. When the provisioner Source is a remote URL (https://, raw
// GitHub, pre-signed SAS), it returns a single customizer pointing at the URL
// directly. When Source is a local path, the upload must already have been
// staged (see ImageBuilder.stageFileProvisioners) — staged is consulted
// instead of the provisioner Source. For directory uploads, one customizer
// is emitted per staged entry so each file lands at its correct relative
// destination on the build VM.
func fileCustomizers(p *builder.Provisioner, index int, staged *StagedFile) ([]armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification, error) {
	if p.Source == "" {
		return nil, fmt.Errorf("provisioner[%d] file: 'source' is required (HTTPS, SAS URL, or local path)", index)
	}
	if p.Destination == "" {
		return nil, fmt.Errorf("provisioner[%d] file: 'destination' is required", index)
	}

	if staged == nil {
		// Remote URL — emit a single customizer pointing at the source.
		return []armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification{
			&armvirtualmachineimagebuilder.ImageTemplateFileCustomizer{
				Type:        to.Ptr("File"),
				Name:        to.Ptr(fmt.Sprintf("file-%d", index)),
				SourceURI:   to.Ptr(p.Source),
				Destination: to.Ptr(p.Destination),
			},
		}, nil
	}

	out := make([]armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification, 0, len(staged.Entries))
	for entryIdx, entry := range staged.Entries {
		name := fmt.Sprintf("file-%d", index)
		if staged.IsDirectory {
			name = fmt.Sprintf("file-%d-%d", index, entryIdx)
		}
		out = append(out, &armvirtualmachineimagebuilder.ImageTemplateFileCustomizer{
			Type:        to.Ptr("File"),
			Name:        to.Ptr(name),
			SourceURI:   to.Ptr(entry.URL(staged.Account, staged.Container)),
			Destination: to.Ptr(destinationForEntry(p.Destination, staged, entry)),
		})
	}
	return out, nil
}

// fileModeCustomizer applies a provisioner's Mode after its file upload(s)
// complete. Numeric chmod modes are only meaningful on Linux targets, so
// Windows builds ignore Mode to match the AWS implementation.
func fileModeCustomizer(p *builder.Provisioner, index int, osType string, staged *StagedFile) armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification {
	if p.Mode == "" || osType == "Windows" {
		return nil
	}

	chmodCmd := fmt.Sprintf("chmod %s %s", p.Mode, shellSingleQuote(p.Destination))
	if staged != nil && staged.IsDirectory {
		chmodCmd = fmt.Sprintf("chmod -R %s %s", p.Mode, shellSingleQuote(p.Destination))
	}

	return &armvirtualmachineimagebuilder.ImageTemplateShellCustomizer{
		Type:   to.Ptr("Shell"),
		Name:   to.Ptr(fmt.Sprintf("file-%d-mode", index)),
		Inline: stringSliceToPointerSlice([]string{chmodCmd}),
	}
}

func powerShellInlineCommands(p *builder.Provisioner, index int) ([]string, error) {
	if len(p.PSScripts) > 0 {
		return powerShellCommandsFromScripts(p, index)
	}
	if len(p.Inline) == 0 {
		return nil, fmt.Errorf("provisioner[%d] powershell: ps_scripts or inline commands are required", index)
	}
	return append([]string{}, p.Inline...), nil
}

func powerShellCommandsFromScripts(p *builder.Provisioner, index int) ([]string, error) {
	executionPolicy := p.ExecutionPolicy
	if executionPolicy == "" {
		executionPolicy = "Bypass"
	}

	commands := []string{
		"$ErrorActionPreference = 'Stop'",
		fmt.Sprintf("Set-ExecutionPolicy -ExecutionPolicy %s -Scope Process -Force", powerShellEscape(executionPolicy)),
		"New-Item -ItemType Directory -Force -Path 'C:\\warpgate-powershell' | Out-Null",
	}

	for scriptIndex, scriptPath := range p.PSScripts {
		content, err := os.ReadFile(scriptPath)
		if err != nil {
			return nil, fmt.Errorf("provisioner[%d] powershell: read script %s: %w", index, scriptPath, err)
		}
		scriptName := fmt.Sprintf("%02d-%s", scriptIndex, filepath.Base(scriptPath))
		scriptOnDisk := "C:\\warpgate-powershell\\" + scriptName
		encoded := base64.StdEncoding.EncodeToString(normalizePowerShellScript(content))
		commands = append(commands,
			fmt.Sprintf("$scriptB64 = '%s'", encoded),
			fmt.Sprintf("[System.IO.File]::WriteAllBytes('%s', [System.Convert]::FromBase64String($scriptB64))", powerShellEscape(scriptOnDisk)),
			fmt.Sprintf("& '%s'", powerShellEscape(scriptOnDisk)),
		)
	}

	return commands, nil
}

func normalizePowerShellScript(content []byte) []byte {
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		content = content[3:]
	}
	return content
}

func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// buildSharedImageDistributor builds the AIB distribution target that publishes
// the produced image into a Compute Gallery image version.
func buildSharedImageDistributor(target *builder.Target, versionTag string) *armvirtualmachineimagebuilder.ImageTemplateSharedImageDistributor {
	galleryImageID := galleryImageDefinitionID(
		target.SubscriptionID,
		target.ResourceGroup,
		target.Gallery,
		target.GalleryImageDefinition,
	)

	regions := make([]string, 0, 1+len(target.TargetRegions))
	regions = append(regions, target.Location)
	for _, r := range target.TargetRegions {
		if r != "" && r != target.Location {
			regions = append(regions, r)
		}
	}

	dist := &armvirtualmachineimagebuilder.ImageTemplateSharedImageDistributor{
		Type:               to.Ptr("SharedImage"),
		RunOutputName:      to.Ptr(runOutputName),
		GalleryImageID:     to.Ptr(galleryImageID + "/versions/" + versionTag),
		ReplicationRegions: stringSliceToPointerSlice(regions),
	}
	if len(target.ImageTags) > 0 {
		dist.ArtifactTags = stringMapToPointerMap(target.ImageTags)
	}
	return dist
}

// stringSliceToPointerSlice converts []string to []*string (Azure SDK convention).
func stringSliceToPointerSlice(in []string) []*string {
	out := make([]*string, len(in))
	for i := range in {
		v := in[i]
		out[i] = &v
	}
	return out
}

// stringMapToPointerMap converts map[string]string to map[string]*string.
func stringMapToPointerMap(in map[string]string) map[string]*string {
	out := make(map[string]*string, len(in))
	for k, v := range in {
		v := v
		out[k] = &v
	}
	return out
}
