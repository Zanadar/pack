package pack

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types"
)

type CertConfig struct {
	Build []string
	Run   []string
}

var (
	buildPrefix = "build:"
	runPrefix   = "run:"
)

func NewCertConfig(paths []string) CertConfig {
	cfg := CertConfig{}

	for _, path := range paths {
		if strings.HasPrefix(path, buildPrefix) {
			cfg.Build = append(cfg.Build, strings.TrimLeft(path, buildPrefix))
		} else if strings.HasPrefix(path, runPrefix) {
			cfg.Run = append(cfg.Run, strings.TrimLeft(path, runPrefix))
		} else {
			cfg.Build = append(cfg.Build, path)
			cfg.Run = append(cfg.Run, path)
		}
	}

	return cfg

}

func AddCerts(c Client, opts BuildOptions, bl, rn ) error {

	c.docker.ContainerCreate()
	c.docker.ContainerExecCreate()
	c.docker.ContainerCommit()
	build := opts.Builder.Image()
	imageRef, err := c.parseTagReference(opts.Image)
	run := c.resolveRunImage()
	// Add certs to build image
	// find build image -> this is builder base-image
	// mounts certs in -> param
	// execute hook for every cert
	// clean up temp certs path
	// save new certs layer + add to image -> return
	// add certs to run

	ephemeralBuilder, err := c.createEphemeralBuilder(rawBuilderImage, opts.Env, order, fetchedBPs)
	if err != nil {
		return err
	}
	defer c.docker.ImageRemove(context.Background(), ephemeralBuilder.Name(), types.ImageRemoveOptions{Force: true})
}
