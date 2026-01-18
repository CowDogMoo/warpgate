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

package cli

// BuildCLIOptions defines command-line options for the build command.
//
// BuildCLIOptions captures options provided by the user via CLI flags
// and arguments. These are validated before being passed to the build logic.
type BuildCLIOptions struct {
	// ConfigFile specifies the path to the build configuration file.
	ConfigFile string

	// Template specifies the name of a template to use from the registry.
	Template string

	// FromGit specifies a git URL to load the template from.
	FromGit string

	// TargetType specifies the output target type (e.g., "container", "ami").
	TargetType string

	// Architectures specifies architectures to build (e.g., "amd64", "arm64").
	Architectures []string

	// Registry specifies the registry to push images to.
	Registry string

	// Tags specifies additional tags to apply to the built image.
	Tags []string

	// Region specifies AWS region for AMI builds.
	Region string

	// InstanceType specifies EC2 instance type for AMI builds.
	InstanceType string

	// Labels specifies image labels (unparsed key=value strings).
	Labels []string

	// BuildArgs specifies build arguments (unparsed key=value strings).
	BuildArgs []string

	// Variables specifies template variable overrides (unparsed key=value strings).
	Variables []string

	// VarFiles specifies paths to variable files.
	VarFiles []string

	// CacheFrom specifies sources to use for build cache.
	CacheFrom []string

	// CacheTo specifies destinations to store build cache.
	CacheTo []string

	// NoCache disables build cache usage if set to true.
	NoCache bool

	// Push indicates whether to push the image to the registry after build.
	Push bool

	// PushDigest indicates whether to push the image by digest without tagging.
	//
	// This option is mutually exclusive with Push.
	PushDigest bool

	// SaveDigests indicates whether to save image digests after push.
	SaveDigests bool

	// DigestDir specifies the directory to save image digest files.
	DigestDir string
}
