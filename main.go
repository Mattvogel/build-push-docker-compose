package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/types"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/moby/moby/client"
	"github.com/moby/moby/pkg/archive"
	"gopkg.in/yaml.v2"
)

var (
	composeFile,
	tags,
	DockerRegistry string

	dockerClient *client.Client
	spec         ComposeSpec
)

type ComposeSpec struct {
	Services map[string]types.ServiceConfig `yaml:"services"`
}

func init() {
	var err error
	composeFile = os.Getenv("COMPOSE_FILE")
	tags = os.Getenv("COMPOSE_TAGS")
	DockerRegistry = os.Getenv("COMPOSE_REGISTRY")

	dockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	compose, err := os.ReadFile(composeFile)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Compose file: ", string(compose))

	err = yaml.Unmarshal(compose, &spec)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Compose Spec: ", spec)

	for _, service := range spec.Services {
		log.Print("test")
		if service.Build.Dockerfile == "" {
			log.Println("No Dockerfile found for service: ")
			continue
		}

		log.Println("Building: ", service.Image)
		buildImage(dockerClient, service)
		pushImage(dockerClient, service)
	}
}

func buildImage(dockerClient *client.Client, service types.ServiceConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()
	buildContext, err := archive.TarWithOptions(service.Build.Context, &archive.TarOptions{})
	if err != nil {
		log.Fatal(err)
	}
	tags := strings.ToLower(service.Image)

	buildOpts := dockerTypes.ImageBuildOptions{
		Tags:   []string{tags},
		Remove: true,
	}
	resp, err := dockerClient.ImageBuild(ctx, buildContext, buildOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		log.Fatal(err)
	}
}

func pushImage(dockerClient *client.Client, service types.ServiceConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	authConfig, _ := json.Marshal(registry.AuthConfig{
		Username: os.Getenv("DOCKER_USERNAME"),
		Password: os.Getenv("DOCKER_PASSWORD"),
	})
	authConfigEncoded := base64.URLEncoding.EncodeToString(authConfig)

	tag := DockerRegistry + "/" + service.Image + ":" + tags
	tag = strings.ToLower(tag)
	pushOpts := dockerTypes.ImagePushOptions{
		RegistryAuth: authConfigEncoded,
	}
	resp, err := dockerClient.ImagePush(ctx, tag, pushOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Close()
	_, err = io.Copy(os.Stdout, resp)
	if err != nil {
		log.Fatal(err)
	}
}
