package extend

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/buildpacks/pack/internal/image"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

var (
	extendPathBase = filepath.Join(string(os.PathSeparator), "cnb", "image")
)

const (
	root = "0"
)

type Config struct {
	Certs []string `toml:certs,omitempty`
}

// Image runs a container from an image, calling the extend binary of that image, passing along extend.toml
// The method returns the name of the extended image
func Image(ctx context.Context, cli client.CommonAPIClient, baseImgName, kind string, tarToml io.ReadCloser) (newName string, err error) {
	kindPath := filepath.Join(extendPathBase, kind)
	extendBin := filepath.Join(kindPath, "extend")
	extendToml := filepath.Join(kindPath, "extend.toml")

	createResp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: baseImgName,
		User:  root,
		Cmd:   []string{extendBin, extendToml},
	}, nil, nil, image.RandomName(fmt.Sprintf("pack.local/extend/container/%s/%%s", kind), 10))
	if err != nil {
		return "", err
	}
	defer cli.ContainerRemove(ctx, createResp.ID, types.ContainerRemoveOptions{Force: true})

	// Its easier to just directly copy to toml vs copying the certs onto an ephemeral volume
	// Copying to a volume requires creating a container and copying anyway (https://github.com/moby/moby/issues/25245)
	if err := cli.CopyToContainer(ctx, createResp.ID, kindPath, tarToml, types.CopyToContainerOptions{}); err != nil {
		return "", err
	}

	// if we have an error here, we likely want to see the logs below
	// TODO figure out the api for passing logs
	if err := cli.ContainerStart(ctx, createResp.ID, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	statusCh, errCh := cli.ContainerWait(ctx, createResp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case <-statusCh:
	}

	//out, err := cli.ContainerLogs(ctx, createResp.ID, types.ContainerLogsOptions{ShowStdout: true})
	//if err != nil {
	//	return "", err
	//}
	//
	//stdcopy.StdCopy(os.Stdout, os.Stderr, out) // TODO debug

	extendedName := image.RandomName(fmt.Sprintf("pack.local/extend/commit/%s/%%s", kind), 10)
	_, err = cli.ContainerCommit(ctx, createResp.ID,
		types.ContainerCommitOptions{Reference: extendedName})
	if err != nil {
		return "", err
	}

	defer cli.ImageRemove(ctx, extendedName, types.ImageRemoveOptions{Force: true})

	//fmt.Println(commitResp.ID) //TODO debug

	newTag, err := shiftLastLayer(cli, extendedName, baseImgName)
	if err != nil {
		return "", err
	}

	//fmt.Println(logs) // TODO debug from shifting layers

	return newTag, nil
}

// we need to move only the layer created by the execution of the extension binary onto the initial image
// this is because running a container and committing also mutates the Config, which we don't want
func shiftLastLayer(cli client.CommonAPIClient, fromImg, toImg string) (string, error) {
	extended, err := name.ParseReference(fromImg)
	if err != nil {
		return "", err
	}
	extendImg, err := daemon.Image(extended, daemon.WithClient(cli))
	if err != nil {
		return "", err
	}

	base, err := name.ParseReference(toImg)
	if err != nil {
		return "", err
	}
	buildImage, err := daemon.Image(base, daemon.WithClient(cli), daemon.WithBufferedOpener())
	if err != nil {
		return "", err
	}

	extensionLayers, err := extendImg.Layers()
	if err != nil {
		return "", err
	}
	topLayer := extensionLayers[len(extensionLayers)-1] // Retrieve the last layer of the image
	buildImage, err = mutate.AppendLayers(buildImage, topLayer)
	if err != nil {
		return "", err
	}
	extendedTag, err := name.NewTag(fmt.Sprintf("%s/extended", fromImg))
	if err != nil {
		return "", err
	}

	_, err = daemon.Write(extendedTag, buildImage)
	if err != nil {
		return "", err
	}

	return extendedTag.Name(), nil
}
