/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

// Package errors provides error wrapping utilities for consistent error handling.
package errors

import "fmt"

// Wrap wraps an error with a descriptive action and optional detail.
// It returns a formatted error in the form "failed to <action> [(<detail>)]: <error>".
//
// Example usage:
//
//	if err := doSomething(); err != nil {
//	    return errors.Wrap("create builder", "", err)
//	}
//
//	if err := parseFile(path); err != nil {
//	    return errors.Wrap("parse config", path, err)
//	}
func Wrap(action, detail string, err error) error {
	if err == nil {
		return nil
	}

	if detail != "" {
		return fmt.Errorf("failed to %s (%s): %w", action, detail, err)
	}
	return fmt.Errorf("failed to %s: %w", action, err)
}
