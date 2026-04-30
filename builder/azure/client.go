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
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/virtualmachineimagebuilder/armvirtualmachineimagebuilder"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// AzureClients holds the Azure SDK service clients used by ImageBuilder.
type AzureClients struct {
	// SubscriptionID is the Azure subscription that owns the build resources.
	SubscriptionID string

	// TenantID is the Azure AD tenant for the credential.
	TenantID string

	// Location is the default Azure region for build resources.
	Location string

	// IdentityID is the user-assigned managed identity resource ID that AIB uses.
	IdentityID string

	// Credential is the token credential used by all clients.
	Credential azcore.TokenCredential

	// ImageTemplates is the AIB ImageTemplate client.
	ImageTemplates *armvirtualmachineimagebuilder.VirtualMachineImageTemplatesClient

	// GalleryImageVersions is the Compute Gallery image version client.
	GalleryImageVersions *armcompute.GalleryImageVersionsClient

	// RoleAssignments grants/revokes RBAC roles. Used by Share() to grant
	// Reader on a published gallery image version to additional principals.
	RoleAssignments *armauthorization.RoleAssignmentsClient

	// BlobStaging is the blob client for the configured FileStagingAccount.
	// nil when no staging account is configured. The client is account-scoped;
	// the container name is recorded on FileStager.
	BlobStaging *azblob.Client

	// FileStagingAccount mirrors ClientConfig.FileStagingAccount so the file
	// stager can construct blob URLs without re-threading config.
	FileStagingAccount string

	// FileStagingContainer mirrors ClientConfig.FileStagingContainer.
	FileStagingContainer string
}

// ClientConfig contains configuration for creating Azure clients.
type ClientConfig struct {
	// SubscriptionID is the Azure subscription to operate against.
	SubscriptionID string

	// TenantID is the Azure AD tenant. If empty, the default credential resolves it.
	TenantID string

	// Location is the default Azure region (e.g., "eastus").
	Location string

	// IdentityID is the resource ID of the user-assigned managed identity used by AIB.
	IdentityID string

	// FileStagingAccount is the Azure storage account that warpgate uploads
	// `file` provisioner sources to. When set together with FileStagingContainer,
	// local file paths in file provisioners are staged to blob storage before
	// the build and the AIB File customizer references the resulting URL.
	FileStagingAccount string

	// FileStagingContainer is the blob container within FileStagingAccount used
	// for staging file provisioner sources.
	FileStagingContainer string
}

// newCredential is the function used to construct the default Azure credential.
// Tests can override it to inject a fake credential without contacting Azure.
var newCredential = func(opts *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
	return azidentity.NewDefaultAzureCredential(opts)
}

// NewAzureClients creates a new set of Azure SDK clients with the given configuration.
// Credentials resolve through azidentity.DefaultAzureCredential, which honours
// AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE CLI login, and
// managed identity in that order.
func NewAzureClients(_ context.Context, cfg ClientConfig) (*AzureClients, error) {
	if cfg.SubscriptionID == "" {
		return nil, errors.New("azure: SubscriptionID is required")
	}

	credOpts := &azidentity.DefaultAzureCredentialOptions{}
	if cfg.TenantID != "" {
		credOpts.TenantID = cfg.TenantID
	}
	cred, err := newCredential(credOpts)
	if err != nil {
		return nil, fmt.Errorf("create azure credential: %w", err)
	}

	templates, err := armvirtualmachineimagebuilder.NewVirtualMachineImageTemplatesClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("create AIB client: %w", err)
	}

	versions, err := armcompute.NewGalleryImageVersionsClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("create gallery image version client: %w", err)
	}

	roleAssignments, err := armauthorization.NewRoleAssignmentsClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("create role assignments client: %w", err)
	}

	var blobStaging *azblob.Client
	if cfg.FileStagingAccount != "" && cfg.FileStagingContainer != "" {
		serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", cfg.FileStagingAccount)
		blobStaging, err = azblob.NewClient(serviceURL, cred, nil)
		if err != nil {
			return nil, fmt.Errorf("create blob client for %s: %w", cfg.FileStagingAccount, err)
		}
	}

	return &AzureClients{
		SubscriptionID:       cfg.SubscriptionID,
		TenantID:             cfg.TenantID,
		Location:             cfg.Location,
		IdentityID:           cfg.IdentityID,
		Credential:           cred,
		ImageTemplates:       templates,
		GalleryImageVersions: versions,
		RoleAssignments:      roleAssignments,
		BlobStaging:          blobStaging,
		FileStagingAccount:   cfg.FileStagingAccount,
		FileStagingContainer: cfg.FileStagingContainer,
	}, nil
}
