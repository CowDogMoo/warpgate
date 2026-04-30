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
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validVersionID = "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/galleries/g/images/i/versions/1.2.3"

type fakeRoleAssignmentsClient struct {
	calls    []roleAssignCall
	stubErrs []error // returned in order; nil for success
}

type roleAssignCall struct {
	scope       string
	name        string
	principalID string
	roleDefID   string
}

func (f *fakeRoleAssignmentsClient) Create(_ context.Context, scope, name string, params armauthorization.RoleAssignmentCreateParameters, _ *armauthorization.RoleAssignmentsClientCreateOptions) (armauthorization.RoleAssignmentsClientCreateResponse, error) {
	c := roleAssignCall{scope: scope, name: name}
	if params.Properties != nil {
		if params.Properties.PrincipalID != nil {
			c.principalID = *params.Properties.PrincipalID
		}
		if params.Properties.RoleDefinitionID != nil {
			c.roleDefID = *params.Properties.RoleDefinitionID
		}
	}
	f.calls = append(f.calls, c)
	if len(f.stubErrs) == 0 {
		return armauthorization.RoleAssignmentsClientCreateResponse{}, nil
	}
	err := f.stubErrs[0]
	f.stubErrs = f.stubErrs[1:]
	return armauthorization.RoleAssignmentsClientCreateResponse{}, err
}

func TestShareGalleryImageVersion_GrantsReaderForEachPrincipal(t *testing.T) {
	fake := &fakeRoleAssignmentsClient{}
	err := shareGalleryImageVersion(context.Background(), fake, "sub-1", validVersionID, []string{"p1", "p2"})
	require.NoError(t, err)
	require.Len(t, fake.calls, 2)

	for i, c := range fake.calls {
		assert.Equal(t, validVersionID, c.scope, "call %d", i)
		assert.NotEmpty(t, c.name, "call %d should have a UUID name", i)
		assert.Equal(t, "/subscriptions/sub-1/providers/Microsoft.Authorization/roleDefinitions/"+readerRoleDefinitionID, c.roleDefID)
	}
	assert.Equal(t, "p1", fake.calls[0].principalID)
	assert.Equal(t, "p2", fake.calls[1].principalID)
}

func TestShareGalleryImageVersion_TolerantOfExisting(t *testing.T) {
	fake := &fakeRoleAssignmentsClient{
		stubErrs: []error{
			&azcore.ResponseError{ErrorCode: "RoleAssignmentExists", StatusCode: http.StatusConflict},
			nil,
		},
	}
	err := shareGalleryImageVersion(context.Background(), fake, "sub-1", validVersionID, []string{"p1", "p2"})
	require.NoError(t, err)
	require.Len(t, fake.calls, 2)
}

func TestShareGalleryImageVersion_TolerantOf409WithoutErrorCode(t *testing.T) {
	fake := &fakeRoleAssignmentsClient{
		stubErrs: []error{
			&azcore.ResponseError{StatusCode: http.StatusConflict},
		},
	}
	err := shareGalleryImageVersion(context.Background(), fake, "sub-1", validVersionID, []string{"p1"})
	require.NoError(t, err)
}

func TestShareGalleryImageVersion_PropagatesUnrelatedError(t *testing.T) {
	fake := &fakeRoleAssignmentsClient{
		stubErrs: []error{errors.New("forbidden")},
	}
	err := shareGalleryImageVersion(context.Background(), fake, "sub-1", validVersionID, []string{"p1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "p1")
	assert.Contains(t, err.Error(), "forbidden")
}

func TestShareGalleryImageVersion_SkipsBlankPrincipals(t *testing.T) {
	fake := &fakeRoleAssignmentsClient{}
	err := shareGalleryImageVersion(context.Background(), fake, "sub-1", validVersionID, []string{"", "  ", "p"})
	require.NoError(t, err)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, "p", fake.calls[0].principalID)
}

func TestShareGalleryImageVersion_NoPrincipalsIsNoop(t *testing.T) {
	fake := &fakeRoleAssignmentsClient{}
	err := shareGalleryImageVersion(context.Background(), fake, "sub-1", validVersionID, nil)
	require.NoError(t, err)
	assert.Empty(t, fake.calls)
}

func TestShareGalleryImageVersion_RejectsBadVersionID(t *testing.T) {
	fake := &fakeRoleAssignmentsClient{}
	err := shareGalleryImageVersion(context.Background(), fake, "sub-1", "/not/a/version", []string{"p"})
	require.Error(t, err)
	assert.Empty(t, fake.calls)
}

func TestShareGalleryImageVersion_RequiresVersionID(t *testing.T) {
	err := shareGalleryImageVersion(context.Background(), &fakeRoleAssignmentsClient{}, "sub-1", "", []string{"p"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "versionID")
}

func TestShareGalleryImageVersion_RequiresSubscriptionID(t *testing.T) {
	err := shareGalleryImageVersion(context.Background(), &fakeRoleAssignmentsClient{}, "", validVersionID, []string{"p"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscriptionID")
}

func TestIsRoleAssignmentExists(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("nope"), false},
		{"matching error code", &azcore.ResponseError{ErrorCode: "RoleAssignmentExists"}, true},
		{"409 without code", &azcore.ResponseError{StatusCode: http.StatusConflict}, true},
		{"403 forbidden", &azcore.ResponseError{StatusCode: http.StatusForbidden}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isRoleAssignmentExists(tt.err))
		})
	}
}
