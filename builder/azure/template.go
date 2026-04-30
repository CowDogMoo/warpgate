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
	"fmt"

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
		return &armvirtualmachineimagebuilder.ImageTemplatePlatformImageSource{
			Type:      to.Ptr("PlatformImage"),
			Publisher: to.Ptr(mp.Publisher),
			Offer:     to.Ptr(mp.Offer),
			SKU:       to.Ptr(mp.SKU),
			Version:   to.Ptr(version),
		}, nil
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
// Supported provisioner types: shell, powershell, file. Other types return
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
		case "script":
			// `script` provisioners run local script files; today we surface a
			// clear error directing users to switch to shell/powershell. A
			// future iteration can stage scripts to blob storage and emit a
			// shell or powershell customizer with ScriptURI.
			return nil, fmt.Errorf("provisioner[%d] type=script is not yet supported for azure builds; use shell or powershell with inline commands", i)
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
// Only inline commands are supported in this iteration.
func shellCustomizer(p *builder.Provisioner, index int) (armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification, error) {
	if len(p.Inline) == 0 {
		return nil, fmt.Errorf("provisioner[%d] shell: inline commands are required", index)
	}
	return &armvirtualmachineimagebuilder.ImageTemplateShellCustomizer{
		Type:   to.Ptr("Shell"),
		Name:   to.Ptr(fmt.Sprintf("shell-%d", index)),
		Inline: stringSliceToPointerSlice(p.Inline),
	}, nil
}

// powerShellCustomizer builds an AIB PowerShell customizer from a powershell
// provisioner. Only inline commands are supported in this iteration.
func powerShellCustomizer(p *builder.Provisioner, index int) (armvirtualmachineimagebuilder.ImageTemplateCustomizerClassification, error) {
	if len(p.Inline) == 0 {
		return nil, fmt.Errorf("provisioner[%d] powershell: inline commands are required", index)
	}
	return &armvirtualmachineimagebuilder.ImageTemplatePowerShellCustomizer{
		Type:        to.Ptr("PowerShell"),
		Name:        to.Ptr(fmt.Sprintf("powershell-%d", index)),
		Inline:      stringSliceToPointerSlice(p.Inline),
		RunElevated: to.Ptr(true),
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
