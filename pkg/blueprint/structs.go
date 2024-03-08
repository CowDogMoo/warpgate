package blueprint

import "github.com/cowdogmoo/warpgate/pkg/packer"

// Blueprint represents the contents of a Blueprint.
type Blueprint struct {
	// Name of the Blueprint
	Name string `yaml:"name"`
	// Path to the provisioning repo
	ProvisioningRepo string
}

// Data holds a blueprint and its associated packer templates.
type Data struct {
	Blueprint       Blueprint
	PackerTemplates []packer.BlueprintPacker
	Container       packer.BlueprintContainer
}
