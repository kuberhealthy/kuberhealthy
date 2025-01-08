package main

import (
	"context"
	"dagger/build/internal/dagger"
	"fmt"
)

type Build struct{}

func (m *Build) Build(ctx context.Context, dir *dagger.Directory, repo string, version string, function string) (string, error) {
	var imageDigest, err = m.buildContainer(ctx, dir, repo, version, function)
	if err != nil {
		return "", err
	}
	return imageDigest, nil
}

// TODO: Had to timebox, the below just needs the ability to increment version.
// func (m *Build) Build_All(ctx context.Context, dir *dagger.Directory, repo string, version string) ([]string, error) {
// 	var cmds = []string{"ami-check", "cronjob-checker", "daemonset-check", "deployment-check", "dns-resolution-check", "http-check", "http-content-check", "image-download-check", "kiam-check", "kuberhealthy", "namespace-pod-check", "network-connection-check", "pod-restarts-check", "pod-status-check", "resource-quota-check", "ssl-expiry-check", "ssl-handshake-check", "test-chec"}
// 	var digests = []string{""}
// 	for _, cmd := range cmds {
// 		var imageDigest, err = m.buildContainer(ctx, dir, repo, version, cmd)
// 		if err != nil {
// 			return []string{""}, err
// 		}
// 		digests = append(digests, imageDigest)
// 	}
// 	return digests, nil
// }

func (m *Build) buildContainer(ctx context.Context, dir *dagger.Directory, repo string, version string, function string) (string, error) {
	var platforms = []dagger.Platform{
		"linux/amd64", // a.k.a. x86_64
		"linux/arm64", // a.k.a. aarch64
	}

	platformVariants := make([]*dagger.Container, 0, len(platforms))
	for _, platform := range platforms {
		// build app
		var builder = dag.Container(dagger.ContainerOpts{Platform: platform}).
			From("golang:1.23").
			WithDirectory("/src", dir).
			WithWorkdir("/src").
			WithExec([]string{"go", "mod", "download"}).
			WithWorkdir(fmt.Sprintf("/src/cmd/%s", function)).
			WithEnvVariable("CGO_ENABLED", "0").
			WithExec([]string{"go", "build", "-v", "-o", fmt.Sprintf("/src/%s", function)})

		// Create empty image and copy binary
		var prodImage = dag.Container(dagger.ContainerOpts{Platform: platform}).
			WithFile(fmt.Sprintf("/bin/%s", function), builder.File(fmt.Sprintf("/src/%s", function))).
			WithEntrypoint([]string{fmt.Sprintf("/bin/%s", function)})

		// Append image to list of variants produced.
		platformVariants = append(platformVariants, prodImage)
	}

	return dag.Container().
		Publish(ctx, fmt.Sprintf("%s/%s:%s", repo, function, version), dagger.ContainerPublishOpts{
			PlatformVariants: platformVariants,
		})
}
