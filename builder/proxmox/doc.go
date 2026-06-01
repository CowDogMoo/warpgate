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

// Package proxmox provides functionality for building Proxmox VE VM templates
// from warpgate template configurations.
//
// This package implements the [github.com/cowdogmoo/warpgate/v3/builder.ProxmoxImageBuilder]
// interface to create custom VM templates on a Proxmox VE cluster. It manages
// the full lifecycle of a build: clone a source template, configure cloud-init,
// boot the VM, wait for the guest agent, run provisioners over SSH, stop the
// VM, and convert it to a Proxmox template.
//
// # Architecture Overview
//
// The package is organized into several key components:
//
//   - [ImageBuilder]: Main entry point implementing the build workflow
//   - [PipelineRunner]: Orchestrates clone → configure → provision → template
//   - [ProxmoxClients]: Proxmox SDK client wrapper
//   - Validation, naming, and error helpers
//
// # Usage
//
// Create a new [ImageBuilder] and execute a build:
//
//	ctx := context.Background()
//	clientConfig := proxmox.ClientConfig{
//	    Endpoint: "https://pve.example.com:8006/api2/json",
//	    APITokenID: "user@pve!warpgate",
//	    APIToken:   os.Getenv("PROXMOX_API_TOKEN"),
//	    Node:       "pve1",
//	}
//
//	builder, err := proxmox.NewImageBuilder(ctx, clientConfig)
//	if err != nil {
//	    return err
//	}
//	defer builder.Close()
//
//	result, err := builder.Build(ctx, config)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Built template VMID: %d on node %s\n", result.TemplateVMID, result.Node)
//
// # Build Flow
//
// The pipeline executes the following steps:
//
//  1. Resolve the source template VMID on the configured node.
//  2. Clone the source into a new VMID with a build-stamped name.
//  3. Apply cloud-init (user/password, SSH key, network) and resize the disk.
//  4. Start the cloned VM and wait for the QEMU guest agent.
//  5. Resolve the VM's IP address via the guest agent.
//  6. Run warpgate provisioners (shell, ansible, file) over SSH.
//  7. Stop the VM, detach the cloud-init drive, and convert it to a template.
//
// # Resource Naming
//
// Cloned VMs and final templates follow the pattern:
//
//	{build-name}-{timestamp}
//
// For example: "kali-build-20260601-120000".
//
// # Error Handling
//
// The package provides enhanced error handling through the [WrapWithRemediation]
// function, which adds contextual information and remediation suggestions to
// common Proxmox API errors (auth failures, missing nodes, VMID collisions).
//
// Errors returned by this package follow Go error wrapping conventions:
//
//	if err != nil {
//	    return fmt.Errorf("clone template: %w", err)
//	}
//
// # Concurrency
//
// [ImageBuilder.Build] is safe to call concurrently for distinct configurations.
// VMID allocation is serialized through Proxmox's cluster-wide
// /cluster/nextid endpoint.
package proxmox
