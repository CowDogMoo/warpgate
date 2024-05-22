package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/docker/buildx/util/progress"

	"strings"
	"time"

	"github.com/cowdogmoo/warpgate/pkg/blueprint"
	bp "github.com/cowdogmoo/warpgate/pkg/blueprint"
	"github.com/cowdogmoo/warpgate/pkg/packer"
	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	dockerClient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"google.golang.org/grpc"
)

// DockerClientInterface represents an interface for Docker client
// operations.
//
// **Methods:**
//
// DockerLogin: Authenticates with a Docker registry.
// DockerPush: Pushes a Docker image to a registry.
// DockerTag: Tags a Docker image with a new name.
type DockerClientInterface interface {
	DockerLogin() error
	DockerPush(image string) error
	DockerTag(sourceImage, targetImage string) error
	DockerBuild(contextDir, dockerfile string, platforms []string, tags []string) error
}

// DockerClient represents a Docker client.
//
// **Attributes:**
//
// AuthStr: The base64 encoded auth string for the Docker registry.
// CLI: API client for Docker operations.
// Container: A packer.Container instance.
type DockerClient struct {
	AuthStr   string
	CLI       dockerClient.APIClient
	Container packer.Container
}

// NewDockerClient creates a new Docker client.
//
// **Returns:**
//
// *DockerClient: A DockerClient instance.
// error: An error if any issue occurs while creating the client.
func NewDockerClient() (*DockerClient, error) {
	cli, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv, dockerClient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerClient{
		CLI: cli,
		Container: packer.Container{
			ImageRegistry: packer.ContainerImageRegistry{},
			ImageHashes:   make(map[string]string),
		},
	}, nil
}

// DockerLogin authenticates with a Docker registry using the provided
// username, password, and server. It constructs an auth string for
// the registry.
//
// **Parameters:**
//
// username: The username for the Docker registry.
// password: The password for the Docker registry.
// server: The server address of the Docker registry.
//
// **Returns:**
//
// string: The base64 encoded auth string.
// error: An error if any issue occurs during the login process.
func (d *DockerClient) DockerLogin() error {
	if d.Container.ImageRegistry.Username == "" || d.Container.ImageRegistry.Credential == "" || d.Container.ImageRegistry.Server == "" {
		return errors.New("username, password, and server must not be empty")
	}

	authConfig := registry.AuthConfig{
		Username:      d.Container.ImageRegistry.Username,
		Password:      d.Container.ImageRegistry.Credential,
		ServerAddress: d.Container.ImageRegistry.Server,
	}

	authBytes, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}
	d.AuthStr = base64.URLEncoding.EncodeToString(authBytes)

	ctx := context.Background()
	_, err = d.CLI.RegistryLogin(ctx, authConfig)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully logged in to %s as %s\n", d.Container.ImageRegistry.Server, d.Container.ImageRegistry.Username)
	fmt.Printf("CREDENTIAL:%s", authConfig)

	return nil
}

// DockerTag tags a Docker image with a new name.
//
// **Parameters:**
//
// sourceImage: The current name of the image.
// targetImage: The new name to assign to the image.
//
// **Returns:**
//
// error: An error if the tagging operation fails.
func (d *DockerClient) DockerTag(sourceImage, targetImage string) error {
	if sourceImage == "" || targetImage == "" {
		return errors.New("sourceImage and targetImage must not be empty")
	}

	ctx := context.Background()
	return d.CLI.ImageTag(ctx, sourceImage, targetImage)
}

// DockerPush pushes a Docker image to a registry using the provided
// auth string.
//
// **Parameters:**
//
// containerImage: The name of the image to push.
// authStr: The auth string for the Docker registry.
//
// **Returns:**
//
// error: An error if the push operation fails.
func (d *DockerClient) PushImage(containerImage string) error {
	if d.AuthStr == "" {
		return errors.New("error: docker client is not authenticated with a registry")
	}

	if containerImage == "" {
		return errors.New("containerImage must not be empty")
	}

	ctx := context.Background()
	resp, err := d.CLI.ImagePush(ctx, containerImage, image.PushOptions{
		RegistryAuth: d.AuthStr,
	})
	if err != nil {
		return err
	}
	defer resp.Close()

	_, err = io.Copy(os.Stdout, resp)
	return err
}

// ProcessPackerTemplates tags and pushes images specified in packer templates.
//
// **Parameters:**
//
// packerTemplates: A slice of PackerTemplate containing the images to tag
// and push.
//
// **Returns:**
//
// error: An error if any operation fails during tagging or pushing.
func (d *DockerClient) ProcessPackerTemplates(pTmpl []packer.PackerTemplate, blueprint bp.Blueprint) error {
	if len(pTmpl) == 0 {
		return errors.New("packer templates must be provided for the blueprint")
	}

	for _, p := range pTmpl {
		if err := d.ProcessTemplate(p, blueprint); err != nil {
			return err
		}
	}

	return nil
}

// ProcessTemplate processes a Packer template by tagging and pushing images
// to a registry.
//
// **Parameters:**
//
// pTmpl: A PackerTemplate containing the image to process.
// blueprint: The blueprint containing tag information.
//
// **Returns:**
//
// error: An error if any operation fails during tagging or pushing.
func (d *DockerClient) ProcessTemplate(pTmpl packer.PackerTemplate, blueprint blueprint.Blueprint) error {
	if blueprint.Name == "" {
		return errors.New("blueprint name must not be empty")
	}

	if blueprint.Tag.Name == "" || blueprint.Tag.Version == "" {
		return errors.New("blueprint tag name and version must not be empty")
	}

	if pTmpl.Container.ImageRegistry.Server == "" || pTmpl.Container.ImageRegistry.Username == "" || pTmpl.Container.ImageRegistry.Credential == "" {
		return fmt.Errorf("registry server '%s', username '%s', and credential must not be empty", pTmpl.Container.ImageRegistry.Server, pTmpl.Container.ImageRegistry.Username)
	}

	// Set the image registry for the Docker client
	d.Container.ImageRegistry = pTmpl.Container.ImageRegistry

	if d.AuthStr == "" {
		if err := d.DockerLogin(); err != nil {
			return fmt.Errorf("failed to login to %s: %v", pTmpl.Container.ImageRegistry.Server, err)
		}
	}

	fmt.Printf("Processing %s image...\n", blueprint.Name)

	platforms := []string{"linux/amd64", "linux/arm64"}
	tags := []string{fmt.Sprintf("%s/%s:%s", pTmpl.Container.ImageRegistry.Server, blueprint.Tag.Name, blueprint.Tag.Version)}

	// Use BuildKit to build and push multi-architecture images
	if err := d.PushMultiArchImage(blueprint.BuildDir, platforms, tags); err != nil {
		return err
	}

	return nil
}

// TagAndPushImages tags and pushes images to a registry.
//
// **Parameters:**
//
// blueprint: The blueprint containing image tag information.
//
// **Returns:**
//
// error: An error if any operation fails during tagging or pushing.
func (d *DockerClient) TagAndPushImages(blueprint blueprint.Blueprint) error {
	var imageTags []string

	logger := func(status *client.SolveStatus) {}

	for arch, hash := range d.Container.ImageHashes {
		if arch == "" || hash == "" {
			return errors.New("arch and hash must not be empty")
		}

		localTag := fmt.Sprintf("sha256:%s", hash)
		remoteTag := fmt.Sprintf("%s/%s:%s",
			strings.TrimPrefix(d.Container.ImageRegistry.Server, "https://"),
			blueprint.Tag.Name, arch)
		fmt.Printf("Tagging image: %s as %s\n", localTag, remoteTag)

		if err := d.DockerTag(localTag, remoteTag); err != nil {
			return err
		}

		fmt.Printf("Pushing image: %s\n", remoteTag)

		if err := progress.Wrap("Pushing image", logger, func(l progress.SubLogger) error {
			return d.pushWithMoby(context.Background(), remoteTag, l)
		}); err != nil {
			return err
		}

		imageTags = append(imageTags, remoteTag)
	}

	latestTag := fmt.Sprintf("%s/%s:latest",
		strings.TrimPrefix(d.Container.ImageRegistry.Server, "https://"),
		blueprint.Tag.Name)

	for _, remoteTag := range imageTags {
		fmt.Printf("Tagging image: %s as %s\n", remoteTag, latestTag)

		if err := d.DockerTag(remoteTag, latestTag); err != nil {
			return err
		}

		fmt.Printf("Pushing image: %s\n", latestTag)

		if err := progress.Wrap("Pushing image", logger, func(l progress.SubLogger) error {
			return d.pushWithMoby(context.Background(), latestTag, l)
		}); err != nil {
			return err
		}
	}

	return nil
}

func (d *DockerClient) PushMultiArchImage(contextDir string, platforms []string, tags []string) error {
	ctx := context.Background()

	// Load Docker configuration
	dockerConfig := config.LoadDefaultConfigFile(os.Stderr)
	tlsConfigs := map[string]*authprovider.AuthTLSConfig{}
	// Set up session for authentication
	attachable := []session.Attachable{authprovider.NewDockerAuthProvider(dockerConfig, tlsConfigs)}

	// Set up progress writer
	pw, err := progresswriter.NewPrinter(ctx, os.Stdout, "plain")
	if err != nil {
		return fmt.Errorf("failed to create progress writer: %v", err)
	}

	// Define the solve options
	solveOpt := client.SolveOpt{
		Exports: []client.ExportEntry{
			{
				Type: client.ExporterImage,
				Attrs: map[string]string{
					"name": strings.Join(tags, ","),
					"push": "true",
				},
			},
		},
		LocalDirs: map[string]string{
			"context": contextDir,
		},
		Session: attachable,
	}

	// Create BuildKit client with custom gRPC options to handle large payloads
	bkClient, err := client.New(ctx, "unix:///var/run/docker.sock", client.WithGRPCDialOption(grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(1024*1024*100), // 100MB
		grpc.MaxCallSendMsgSize(1024*1024*100), // 100MB
	)))
	if err != nil {
		return fmt.Errorf("failed to create BuildKit client: %v", err)
	}

	// Marshal the state (using busybox as an example here)
	st := llb.Image("busybox").Run(llb.Shlex("echo 'hello world'"))
	def, err := st.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("failed to marshal LLB: %v", err)
	}

	// Solve the build and push the image
	_, err = bkClient.Solve(ctx, def, solveOpt, pw.Status())
	if err != nil {
		return fmt.Errorf("failed to solve build: %v", err)
	}

	return nil
}

// RegistryAuth is a helper struct to implement the imagetools.Auth interface.
type RegistryAuth struct {
	authConfig registry.AuthConfig
}

// GetAuthConfig returns the auth configuration.
func (ra *RegistryAuth) GetAuthConfig(_ string) (registry.AuthConfig, error) {
	return ra.authConfig, nil
}

// pushWithMoby pushes the image using the Docker API and manages progress logging.
func (d *DockerClient) pushWithMoby(ctx context.Context, name string, l progress.SubLogger) error {
	api := d.CLI
	if api == nil {
		return errors.New("invalid empty Docker API reference")
	}

	creds := &RegistryAuth{
		authConfig: registry.AuthConfig{
			Username:      d.Container.ImageRegistry.Username,
			Password:      d.Container.ImageRegistry.Credential,
			ServerAddress: d.Container.ImageRegistry.Server,
		},
	}

	authStr, err := json.Marshal(creds.authConfig)
	if err != nil {
		return err
	}

	rc, err := api.ImagePush(ctx, name, image.PushOptions{
		RegistryAuth: base64.URLEncoding.EncodeToString(authStr),
	})
	if err != nil {
		return err
	}

	started := map[string]*client.VertexStatus{}

	defer func() {
		for _, st := range started {
			if st.Completed == nil {
				now := time.Now()
				st.Completed = &now
				l.SetStatus(st)
			}
		}
	}()

	dec := json.NewDecoder(rc)
	var parsedError error
	for {
		var jm jsonmessage.JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if parsedError != nil {
				return parsedError
			}
			if err == io.EOF {
				break
			}
			return err
		}
		if jm.ID != "" {
			id := "pushing layer " + jm.ID
			st, ok := started[id]
			if !ok {
				if jm.Progress != nil || jm.Status == "Pushed" {
					now := time.Now()
					st = &client.VertexStatus{
						ID:      id,
						Started: &now,
					}
					started[id] = st
				} else {
					continue
				}
			}
			st.Timestamp = time.Now()
			if jm.Progress != nil {
				st.Current = jm.Progress.Current
				st.Total = jm.Progress.Total
			}
			if jm.Error != nil {
				now := time.Now()
				st.Completed = &now
			}
			if jm.Status == "Pushed" {
				now := time.Now()
				st.Completed = &now
				st.Current = st.Total
			}
			l.SetStatus(st)
		}
		if jm.Error != nil {
			parsedError = jm.Error
		}
	}
	return nil

}
