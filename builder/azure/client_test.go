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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTokenCredential satisfies azcore.TokenCredential without contacting Azure.
// The Azure SDK ARM client constructors accept any credential and do not call
// GetToken during construction, so this lets us exercise NewAzureClients without
// real Azure credentials.
type fakeTokenCredential struct{}

func (f *fakeTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "fake-token"}, nil
}

// installFakeCred replaces newCredential with one that returns fakeTokenCredential.
// The returned function restores the original, suitable for defer.
func installFakeCred(t *testing.T) func() {
	t.Helper()
	orig := newCredential
	newCredential = func(_ *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return &fakeTokenCredential{}, nil
	}
	return func() { newCredential = orig }
}

func TestNewAzureClients_RequiresSubscriptionID(t *testing.T) {
	defer installFakeCred(t)()

	_, err := NewAzureClients(context.Background(), ClientConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SubscriptionID is required")
}

func TestNewAzureClients_CredentialError(t *testing.T) {
	orig := newCredential
	newCredential = func(_ *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return nil, errors.New("credential boom")
	}
	defer func() { newCredential = orig }()

	_, err := NewAzureClients(context.Background(), ClientConfig{SubscriptionID: "sub-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credential boom")
}

func TestNewAzureClients_HappyPath(t *testing.T) {
	defer installFakeCred(t)()

	cfg := ClientConfig{
		SubscriptionID: "sub-1",
		TenantID:       "tenant-1",
		Location:       "eastus",
		IdentityID:     "/subscriptions/sub-1/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/uami",
	}

	clients, err := NewAzureClients(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, clients)

	assert.Equal(t, "sub-1", clients.SubscriptionID)
	assert.Equal(t, "tenant-1", clients.TenantID)
	assert.Equal(t, "eastus", clients.Location)
	assert.Equal(t, cfg.IdentityID, clients.IdentityID)
	assert.NotNil(t, clients.ImageTemplates)
	assert.NotNil(t, clients.GalleryImageVersions)
	assert.NotNil(t, clients.RoleAssignments)
	assert.Nil(t, clients.BlobStaging, "BlobStaging should be nil when no staging config")
}

func TestNewAzureClients_WithFileStagingAccount(t *testing.T) {
	defer installFakeCred(t)()

	cfg := ClientConfig{
		SubscriptionID:       "sub-1",
		FileStagingAccount:   "mystorageacct",
		FileStagingContainer: "staging",
	}

	clients, err := NewAzureClients(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, clients)
	assert.NotNil(t, clients.BlobStaging, "BlobStaging should be set when account and container are provided")
	assert.Equal(t, "mystorageacct", clients.FileStagingAccount)
	assert.Equal(t, "staging", clients.FileStagingContainer)
}

func TestNewAzureClients_StagingRequiresBothAccountAndContainer(t *testing.T) {
	defer installFakeCred(t)()

	tests := []struct {
		name      string
		account   string
		container string
		wantBlob  bool
	}{
		{"both empty", "", "", false},
		{"account only", "acct", "", false},
		{"container only", "", "ctr", false},
		{"both set", "acct", "ctr", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ClientConfig{
				SubscriptionID:       "sub-1",
				FileStagingAccount:   tt.account,
				FileStagingContainer: tt.container,
			}
			clients, err := NewAzureClients(context.Background(), cfg)
			require.NoError(t, err)
			if tt.wantBlob {
				assert.NotNil(t, clients.BlobStaging)
			} else {
				assert.Nil(t, clients.BlobStaging)
			}
		})
	}
}
