package extend

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/docker/docker/pkg/stdcopy"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpacks/pack/logging"

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
	root           = "0"
	extendBaseName = "pack.local_extend"
)

type Certs []string

// Struct used to store exend configuration on disk as "extend.toml"
type Config struct {
	Certs []string `toml:"certs,omitempty"`
}

type ImageExtender struct {
	Kind          string
	ExtendToml    io.Reader // Must be a reader of a tar
	Client        client.CommonAPIClient
	BaseImageName string
	Logger        logging.Logger
	LogCopy       func(dstout, dsterr io.Writer, src io.Reader) (written int64, err error) // Usually stdcopy.StdCopy
}

func DefaultImageExtender(kind string, extendToml io.Reader, client client.CommonAPIClient, baseImage string, logger logging.Logger) *ImageExtender {
	return &ImageExtender{
		Kind:          kind,
		ExtendToml:    extendToml,
		Client:        client,
		BaseImageName: baseImage,
		Logger:        logger,
		LogCopy:       stdcopy.StdCopy,
	}
}

// Image runs a container from an image, calling the extend binary of that image, passing along extend.toml
// The method returns the name of the extended image
// TODO this is quite similair to a phase...but we haven't figured out how to build a common abstraction
func (i *ImageExtender) Extend(ctx context.Context) (newName string, err error) {
	cli := i.Client
	kind := i.Kind

	kindPath := filepath.Join(extendPathBase, kind)
	extendBin := filepath.Join(kindPath, "extend")
	extendToml := filepath.Join(kindPath, "extend.toml")

	containerConfig := container.Config{
		Image: i.BaseImageName,
		User:  root,
		Cmd:   []string{extendBin, extendToml},
	}

	createResp, err := cli.ContainerCreate(
		ctx,
		&containerConfig,
		nil,
		nil,
		image.RandomName(fmt.Sprintf("%s_container_%s_%%s", extendBaseName, kind), 10))
	if err != nil {
		return "", err
	}

	defer cli.ContainerRemove(ctx, createResp.ID, types.ContainerRemoveOptions{Force: true})

	if err := cli.CopyToContainer(ctx, createResp.ID, kindPath, i.ExtendToml, types.CopyToContainerOptions{}); err != nil {
		return "", err
	}

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

	out, err := cli.ContainerLogs(ctx, createResp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", err
	}

	i.LogCopy(logging.GetWriterForLevel(i.Logger, logging.InfoLevel), logging.GetWriterForLevel(i.Logger, logging.ErrorLevel), out)

	extendedName := image.RandomName(fmt.Sprintf("%s/commit/%s/%%s", extendBaseName, kind), 10)
	_, err = cli.ContainerCommit(ctx, createResp.ID,
		types.ContainerCommitOptions{Reference: extendedName})
	if err != nil {
		return "", err
	}

	defer cli.ImageRemove(ctx, extendedName, types.ImageRemoveOptions{Force: true})

	extendImage, err := getImage(cli, extendedName)
	if err != nil {
		return "", err
	}

	baseImage, err := getImage(cli, i.BaseImageName)
	if err != nil {
		return "", err
	}

	newTag, err := shiftLastLayer(extendImage, baseImage, i.BaseImageName)
	if err != nil {
		return "", err
	}
	//
	////fmt.Println(logs) // TODO debug from shifting layers

	return newTag, nil
}

func getImage(cli client.CommonAPIClient, imgName string) (v1.Image, error) {
	ref, err := name.ParseReference(imgName)
	if err != nil {
		return nil, err
	}

	return daemon.Image(ref, daemon.WithClient(cli))
}

// we need to move only the layer created by the execution of the extension binary onto the initial image
// this is because running a container and committing also mutates the ContainerCreateCalledWithConfig, which we don't want
func shiftLastLayer(fromImg, toImg v1.Image, baseName string) (string, error) {
	extensionLayers, err := fromImg.Layers()
	if err != nil {
		return "", err
	}
	topLayer := extensionLayers[len(extensionLayers)-1] // Retrieve the last layer of the image
	toImg, err = mutate.AppendLayers(toImg, topLayer)
	if err != nil {
		return "", err
	}

	extendedTag, err := name.NewTag(fmt.Sprintf("%s-extended", baseName))
	if err != nil {
		return "", errors.Wrapf(err, "problem tagging %s", baseName)
	}

	_, err = daemon.Write(extendedTag, toImg)
	if err != nil {
		return "", err
	}

	return extendedTag.Name(), nil
}
