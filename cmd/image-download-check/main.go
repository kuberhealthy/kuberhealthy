package main

import (
	"context"

	"github.com/containerd/containerd"
	log "github.com/sirupsen/logrus"
)

func main() {

	var imageName = "nginx"

	err := pullImage(imageName)
	if err != nil {
		log.Fatal(err)
	}

}

// pullImage pulls an image from dockerhub
func pullImage(imageName string) error {

	// create a new containerd client
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		return err
	}
	defer client.Close()

	// ctx := namespaces.WithNamespace(context.Background(), "example")
	ctx := context.Background()

	image, err := client.Pull(ctx, imageName)
	if err != nil {
		return err
	}

	log.Printf("Successfully pulled %s image\n", image.Name())
	log.Println("Image: ", image)

	return nil
}

/*
func listImagesDocker() error {

	// create a context for docker
	docker := namespaces.WithNamespace(context.Background(), "docker")

	// set docker as the client's default namespace
	// client, err := containerd.New("unix:///var/run/docker.sock", containerd.WithDefaultNamespace("docker"))
	client, err := containerd.New("/var/run/docker.sock", containerd.WithDefaultNamespace("docker"))
	if err != nil {
		return err
	}

	// get a list of images from local docker
	images, err := client.ListImages(docker)
	if err != nil {
		return err
	}

	for _, image := range images {
		log.Println("Image::", image.Name())
	}
	return nil
}

func listImages() error {

	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		return err
	}
	defer client.Close()
	ctx := namespaces.WithNamespace(context.Background(), "example")
	images, err := client.ListImages(ctx)
	if err != nil {
		return err
	}

	for _, image := range images {
		log.Println("Image::", image.Name())
	}
	return nil
}
*/
