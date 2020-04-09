package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/docker/pkg/archive"
	"github.com/google/go-containerregistry/pkg/v1/daemon"

	"github.com/docker/docker/pkg/stdcopy"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func DoMain() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	kind := os.Args[1]
	extendPathBase := "/cnb/image"
	kindPath := filepath.Join(extendPathBase, kind)
	extendBin := filepath.Join(kindPath, "extend")
	extendToml := filepath.Join(kindPath, "extend.toml")

	createResp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: "cnbs/sample-stack-build:bionic",
		User:  "0",
		Cmd:   []string{extendBin, extendToml},
	}, nil, nil, "build-extend")
	if err != nil {
		log.Fatal(err)
	}

	defer cli.ContainerRemove(ctx, createResp.ID, types.ContainerRemoveOptions{Force: true})

	tomlPath := os.Args[2]
	tarToml, err := archive.Tar(tomlPath, archive.Uncompressed)
	if err != nil {
		log.Fatal(err)
	}

	if err := cli.CopyToContainer(ctx, createResp.ID, kindPath, tarToml, types.CopyToContainerOptions{}); err != nil {
		log.Fatal(err)
	}

	if err := cli.ContainerStart(ctx, createResp.ID, types.ContainerStartOptions{}); err != nil {
		log.Fatal(err)
	}

	statusCh, errCh := cli.ContainerWait(ctx, createResp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			log.Fatal(err)
		}
	case <-statusCh:
	}

	commitResp, err := cli.ContainerCommit(ctx, createResp.ID, types.ContainerCommitOptions{Reference: "cnbs/sample-stack-build:bionic-extended-temp"})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(commitResp.ID)

	out, err := cli.ContainerLogs(ctx, createResp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		log.Fatal(err)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)

	extended, err := name.ParseReference("cnbs/sample-stack-build:bionic-extended-temp")
	if err != nil {
		log.Fatal(err)
	}
	img, err := daemon.Image(extended, daemon.WithClient(cli))
	if err != nil {
		log.Fatal(err)
	}

	base, err := name.ParseReference("cnbs/sample-stack-build:bionic")
	if err != nil {
		log.Fatal(err)
	}
	buildImage, err := daemon.Image(base, daemon.WithClient(cli), daemon.WithBufferedOpener())
	if err != nil {
		log.Fatal(err)
	}

	extensionLayers, err := img.Layers()
	if err != nil {
		log.Fatal(err)
	}

	top := extensionLayers[len(extensionLayers)-1]

	buildImage, err = mutate.AppendLayers(buildImage, top)
	if err != nil {
		log.Fatal(err)
	}

	buildTag, err := name.NewTag("cnbs/sample-stack-build:bionic-extended")
	if err != nil {
		log.Fatal(err)
	}

	done, err := daemon.Write(buildTag, buildImage)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(done)
}

func main() {
	DoMain()
}
