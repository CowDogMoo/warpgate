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
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/google/uuid"
)

// readerRoleDefinitionID is the well-known Azure built-in Reader role.
// Granting Reader on a gallery image version scope lets a principal list and
// see the version (which is what consumers need to deploy from it).
const readerRoleDefinitionID = "acdd72a7-3385-48ef-bd42-f606fba81ae7"

// roleAssignmentsAPI is the subset of the Azure RoleAssignments client used
// by sharing. Tests can substitute a fake without contacting Azure.
type roleAssignmentsAPI interface {
	Create(ctx context.Context, scope, roleAssignmentName string, parameters armauthorization.RoleAssignmentCreateParameters, options *armauthorization.RoleAssignmentsClientCreateOptions) (armauthorization.RoleAssignmentsClientCreateResponse, error)
}

// shareGalleryImageVersion grants the Reader role on the version's resource
// scope to each principal ID. Existing assignments (RoleAssignmentExists /
// HTTP 409) are tolerated so re-running Share is idempotent.
func shareGalleryImageVersion(ctx context.Context, client roleAssignmentsAPI, subscriptionID, versionID string, principalIDs []string) error {
	if versionID == "" {
		return fmt.Errorf("versionID is required")
	}
	if subscriptionID == "" {
		return fmt.Errorf("subscriptionID is required")
	}
	if _, err := parseGalleryVersionID(versionID); err != nil {
		return err
	}
	if len(principalIDs) == 0 {
		return nil
	}

	roleDefID := fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", subscriptionID, readerRoleDefinitionID)

	for _, principalID := range principalIDs {
		principalID = strings.TrimSpace(principalID)
		if principalID == "" {
			continue
		}
		params := armauthorization.RoleAssignmentCreateParameters{
			Properties: &armauthorization.RoleAssignmentProperties{
				PrincipalID:      to.Ptr(principalID),
				RoleDefinitionID: to.Ptr(roleDefID),
			},
		}
		if _, err := client.Create(ctx, versionID, uuid.New().String(), params, nil); err != nil {
			if isRoleAssignmentExists(err) {
				continue
			}
			return fmt.Errorf("grant Reader on %s to %s: %w", versionID, principalID, err)
		}
	}
	return nil
}

// isRoleAssignmentExists returns true when err indicates that an equivalent
// role assignment is already in place. Azure surfaces this as HTTP 409 with
// the "RoleAssignmentExists" error code.
func isRoleAssignmentExists(err error) bool {
	var respErr *azcore.ResponseError
	if !errors.As(err, &respErr) {
		return false
	}
	if respErr.ErrorCode == "RoleAssignmentExists" {
		return true
	}
	return respErr.StatusCode == http.StatusConflict
}
