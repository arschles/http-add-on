package build

import (
	"fmt"
	"strings"

	"github.com/kedacore/http-add-on/pkg/env"
	"github.com/magefile/mage/sh"
)

func getGitSHA() (string, error) {
	return sh.Output("git", "rev-parse", "--short", "HEAD")
}

// DockerBuild calls the following and returns the resulting error, if any:
//
//	docker build -t <image> -f <dockerfileLocation> <context>
func DockerBuild(image, dockerfileLocation, context string) error {
	return sh.RunV(
		"docker",
		"build",
		"-t",
		image,
		"-f",
		dockerfileLocation,
		".",
	)
}

// DockerPush calls the following and returns the resulting error, if any:
//
//	docker push <image>
func DockerPush(image string) error {
	return sh.RunV("docker", "push", image)
}

func GetImageName(envName string) (string, error) {
	img, err := env.Get(envName)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(img, "${GIT_SHA}") {
		sha, err := getGitSHA()
		if err != nil {
			return "", err
		}
		trimmed := strings.TrimRight(img, "${GIT_SHA}")
		return fmt.Sprintf("%s:sha-%s", trimmed, sha), nil
	}
	return img, nil
}
