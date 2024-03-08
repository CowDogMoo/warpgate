package blueprint

import "github.com/cowdogmoo/warpgate/pkg/packer"

// Blueprint represents the configuration of a blueprint for image building.
//
// **Attributes:**
//
// Name: Name of the blueprint.
// ProvisioningRepo: Path to the repository containing provisioning logic.
type Blueprint struct {
	Name             string `yaml:"name"`
	ProvisioningRepo string
}

// Data holds a blueprint and its associated Packer templates and container
// configuration.
//
// **Attributes:**
//
// Blueprint: The blueprint configuration.
// PackerTemplates: A slice of Packer templates associated with the blueprint.
// Container: The container configuration for the blueprint.
type Data struct {
	Blueprint       Blueprint
	PackerTemplates []packer.BlueprintPacker
	Container       packer.BlueprintContainer
}
