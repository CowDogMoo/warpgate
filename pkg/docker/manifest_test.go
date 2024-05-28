package docker_test

// func TestManifestCreate(t *testing.T) {
// 	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
// 	assert.NoError(t, err)

// 	dockerClient := &docker.DockerClient{
// 		CLI: cli,
// 		Container: packer.Container{
// 			ImageHashes: map[string]string{
// 				"amd64": "sha256:bb43e42b608500615a61d780a0484a52049b35e130f705773a3bf3ee59645d80",
// 				"arm64": "sha256:79d4c24ecd61d781c6cf36ae83811ade7ef0e9dc2e234791dd26949e00716810",
// 			},
// 		},
// 	}

// 	targetImage := "ghcr.io/l50/atomic-red-team:latest"
// 	imageTags := []string{"amd64", "arm64"}

// 	manifestList, err := dockerClient.ManifestCreate(context.Background(), targetImage, imageTags)
// 	assert.NoError(t, err)

// 	assert.Equal(t, 2, len(manifestList.Manifests))
// 	assert.Equal(t, "sha256:bb43e42b608500615a61d780a0484a52049b35e130f705773a3bf3ee59645d80", manifestList.Manifests[0].Digest.String())
// 	assert.Equal(t, "sha256:79d4c24ecd61d781c6cf36ae83811ade7ef0e9dc2e234791dd26949e00716810", manifestList.Manifests[1].Digest.String())
// }

// func TestPushManifest(t *testing.T) {
// 	manifestList := ocispec.Index{
// 		Versioned: specs.Versioned{
// 			SchemaVersion: 2,
// 		},
// 		MediaType: ocispec.MediaTypeImageIndex,
// 		Manifests: []ocispec.Descriptor{
// 			{
// 				MediaType: ocispec.MediaTypeImageManifest,
// 				Digest:    "sha256:bb43e42b608500615a61d780a0484a52049b35e130f705773a3bf3ee59645d80",
// 				Size:      420672803,
// 				Platform:  &ocispec.Platform{OS: "linux", Architecture: "amd64"},
// 			},
// 			{
// 				MediaType: ocispec.MediaTypeImageManifest,
// 				Digest:    "sha256:79d4c24ecd61d781c6cf36ae83811ade7ef0e9dc2e234791dd26949e00716810",
// 				Size:      554735173,
// 				Platform:  &ocispec.Platform{OS: "linux", Architecture: "arm64"},
// 			},
// 		},
// 	}

// 	// Set up a test HTTP server
// 	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method != http.MethodPut {
// 			t.Fatalf("Expected method PUT, got %s", r.Method)
// 		}
// 		if r.Header.Get("Authorization") != "Bearer test-auth-token" {
// 			t.Fatalf("Expected Authorization header to be Bearer test-auth-token, got %s", r.Header.Get("Authorization"))
// 		}
// 		w.WriteHeader(http.StatusCreated)
// 	}))
// 	defer ts.Close()

// 	// Create a DockerClient with the test server URL
// 	client := &docker.DockerClient{
// 		AuthStr: "test-auth-token",
// 		Container: packer.Container{
// 			ImageRegistry: packer.ContainerImageRegistry{
// 				Credential: "test-auth-token",
// 				Server:     ts.URL,
// 				Username:   "test-username",
// 			},
// 		},
// 	}

// 	repo := "ghcr.io/l50/atomic-red-team"
// 	tag := "latest"
// 	imageName := repo + ":" + tag

// 	err := client.PushManifest(imageName, manifestList)
// 	assert.NoError(t, err)
// }
