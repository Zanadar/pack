package container

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	dcontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pkg/errors"
)

func Run(ctx context.Context, docker client.CommonAPIClient, ctrID string, out, errOut io.Writer, tty bool) error {
	bodyChan, errChan := docker.ContainerWait(ctx, ctrID, dcontainer.WaitConditionNextExit)

	if err := docker.ContainerStart(ctx, ctrID, types.ContainerStartOptions{}); err != nil {
		return errors.Wrap(err, "container start")
	}
	logs, err := docker.ContainerLogs(ctx, ctrID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return errors.Wrap(err, "container logs stdout")
	}
	if tty {
		w := io.MultiWriter(out, errOut)
		if _, err := io.Copy(w, logs); err != nil {
			return err
		}
	} else {
		out = stdcopy.NewStdWriter(out, stdcopy.Stdout)
		errOut = stdcopy.NewStdWriter(errOut, stdcopy.Stderr)
		copyErr := make(chan error)
		go func() {
			_, err := stdcopy.StdCopy(out, errOut, logs)
			copyErr <- err
		}()

		select {
		case body := <-bodyChan:
			if body.StatusCode != 0 {
				return fmt.Errorf("failed with status code: %d", body.StatusCode)
			}
		case err := <-errChan:
			return err
		}
		return <-copyErr
	}
	return nil
}
