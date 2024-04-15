package registry

import (
	"fmt"
	"io"
	"net/http"
)

// ValidateToken checks the validity of a GitHub access token by making
// a GET request to the GitHub API. It sets the Authorization header with
// the token and examines the response status code.
//
// **Parameters:**
//
// token: The GitHub access token to validate.
//
// **Returns:**
//
// error: An error if the token is invalid, or if any issue occurs during
// the request or reading the response.
func ValidateToken(token string) error {
	// Define the GitHub API URL for user authentication
	url := "https://api.github.com/user"

	// Create a new request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set the Authorization header with the token
	req.Header.Set("Authorization", "token "+token)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	// Check if the status code is not 200
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token validation failed with status code: %d", resp.StatusCode)
	}

	return nil
}
