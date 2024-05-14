/*
Copyright © 2024-present, Jayson Grace <jayson.e.grace@gmail.com>

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
package registry

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// HttpClient is an interface that defines the Do method for making HTTP requests.
type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var Client HttpClient = &http.Client{}

// ValidateToken checks the validity of a GitHub access token by making
// a GET request to the GitHub API. If no token is provided as an argument,
// it checks for a GITHUB_TOKEN environment variable and uses that.
// It sets the Authorization header with the token and examines the response status code.
//
// **Parameters:**
//
// token: The GitHub access token to validate. If empty, the function will
// check for a GITHUB_TOKEN environment variable.
//
// **Returns:**
//
// error: An error if the token is invalid, or if any issue occurs during
// the request or reading the response, or if no token is provided and
// the GITHUB_TOKEN environment variable is not set.
func ValidateToken(token string) error {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
		if token == "" {
			return fmt.Errorf("no token provided and GITHUB_TOKEN env var is not set")
		}
	}

	url := "https://api.github.com/user"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "token "+token)

	resp, err := Client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token validation failed with status code: %d", resp.StatusCode)
	}

	return nil
}
