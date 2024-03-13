package main

import (
	"context"
	"io"
	"log"
	"os"
	"regexp"
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
	composeContext,
	composeFile,
	tags,
	username,
	password,
	DockerRegistry string

	dockerClient *client.Client
	spec         ComposeSpec
)

type ComposeSpec struct {
	Services map[string]Service `yaml:"services"`
}

type Service struct {
	Image string `yaml:"image"`
	Build struct {
		Context    string `yaml:"context"`
		Dockerfile string `yaml:"dockerfile"`
	} `yaml:"build"`
}

func init() {
	var err error
	composeContext = os.Getenv("COMPOSE_CONTEXT")
	composeFile = os.Getenv("COMPOSE_FILE")
	tags = os.Getenv("COMPOSE_TAGS")
	DockerRegistry = os.Getenv("COMPOSE_REGISTRY")
	username = os.Getenv("COMPOSE_USERNAME")
	password = os.Getenv("COMPOSE_PASSWORD")

	dockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	compose, err := os.ReadFile(composeContext + composeFile)
	if err != nil {
		log.Fatal(err)
	}
	err = yaml.Unmarshal(compose, &spec)
	if err != nil {
		log.Fatal("Unmarshalling Error: ", err)
	}
	for key, service := range spec.Services {
		if service.Build.Dockerfile == "" {
			continue
		}
		log.Println("Service ID: ", key)
		log.Println("Service Image Name: ", service.Image)
		st := types.ServiceConfig{
			Image: key,
			Build: &types.BuildConfig{
				Context:    service.Build.Context,
				Dockerfile: service.Build.Dockerfile,
			},
		}

		log.Println("Building: ", key)
		buildImage(dockerClient, st)
		pushImage(dockerClient, st)
	}
}

func buildImage(dockerClient *client.Client, service types.ServiceConfig) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()
	buildContext, err := archive.TarWithOptions(service.Build.Context, &archive.TarOptions{})
	if err != nil {
		log.Fatal("Build Context :", err)
	}
	imageName := strings.ToLower(DockerRegistry + "/" + service.Image + ":" + tags)

	buildOpts := dockerTypes.ImageBuildOptions{
		Dockerfile: service.Build.Dockerfile,
		Context:    buildContext,
		Tags:       []string{imageName},
		Remove:     true,
	}
	resp, err := dockerClient.ImageBuild(ctx, buildContext, buildOpts)
	if err != nil {
		log.Fatal("Image Build: ", err)
	}
	defer resp.Body.Close()
	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		log.Fatal("IO Close Error: ", err)
	}
	return imageName
}

func pushImage(dockerClient *client.Client, service types.ServiceConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()
	var authEncoded string

	if isECRRepositoryURL(DockerRegistry) {
		// AWS ECR
		// https://docs.aws.amazon.com/AmazonECR/latest/userguide/registry_auth.html
		//
		authConfig := registry.AuthConfig{
			Username: "AWS",
			Password: password,
		}
		authEncoded, _ = registry.EncodeAuthConfig(authConfig)
	} else {
		// Docker Hub
		authConfig := registry.AuthConfig{
			Username: username,
			Password: password,
		}
		authEncoded, _ = registry.EncodeAuthConfig(authConfig)
	}

	tag := DockerRegistry + "/" + service.Image + ":" + tags
	tag = strings.ToLower(tag)
	pushOpts := dockerTypes.ImagePushOptions{
		RegistryAuth: authEncoded,
	}
	resp, err := dockerClient.ImagePush(ctx, tag, pushOpts)
	if err != nil {
		log.Fatal("Image Push Error: ", err)
	}
	defer resp.Close()
	_, err = io.Copy(os.Stdout, resp)
	if err != nil {
		log.Fatal("Error closing push reader: ", err)
	}
}

func isECRRepositoryURL(url string) bool {
	if url == "public.ecr.aws" {
		return true
	}
	// Regexp is based on the ecr urls shown in https://docs.aws.amazon.com/AmazonECR/latest/userguide/registry_auth.html
	var ecrRexp = regexp.MustCompile(`^.*?dkr\.ecr\..*?\.amazonaws\.com$`)
	return ecrRexp.MatchString(url)
}
