package main

import (
	"bytes"
	"os"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

func main() {

	// make a docker client
	dockerClient, err := docker.NewClientFromEnv()
	if err != nil {
		log.Errorln("failed to created new docker client", err)
		os.Exit(1)
	}

	// create a bytes buffer for our docker pull options output stream
	b := new(bytes.Buffer)

	// create channel that will read bytes from our buffer
	c := make(chan bytes.Buffer)

	// set options for docker image pull
	opts := docker.PullImageOptions{
		Repository: "nginx",
		Tag:        "1.21",
		// Platform: string,
		OutputStream: b,
	}
	auth := docker.AuthConfiguration{}

	// perform docker pull on target image
	err = dockerClient.PullImage(opts, auth)
	if err != nil {
		log.Errorln("failed to pull docker image with options ", opts)
		os.Exit(1)
	}

}
