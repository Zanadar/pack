package extend_test

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"testing"

	"github.com/buildpacks/pack/internal/logging"

	"github.com/docker/docker/api/types"

	"github.com/buildpacks/pack/extend"
	h "github.com/buildpacks/pack/testhelpers"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// Test that we called the extend binary
// test that we have the results of calling the extend binary
// test the error case of calling extend on a builder without extend

type fakeClient struct {
	client.Client
	ContainerCreateCalledWithConfig container.Config
	ContainerCreateCalledWithName   string

	ContainerRemoveCalledWithId     string
	ContainerRemoveCalledWithConfig types.ContainerRemoveOptions

	ContainerCopyCalledWithReaderString string
	ContainerCopyCalledWithId           string
	ContainerCopyCalledWithDst          string
	ContainerCopyCalledWithConfig       types.CopyToContainerOptions

	ContainerStartCalledWithConfig types.ContainerStartOptions
	ContainerStartCalledWithID     string

	ContainerWaitCalledWithID        string
	ContainerWaitCalledWithCondition container.WaitCondition

	ContainerCommitCalledWithReference string
	ContainerCommitCalledWithID        string

	ContainerLogsCalledWithId     string
	ContainerLogsCalledWithConfig types.ContainerLogsOptions

	ImageDeleteCalledWithName string
}

// Should this be a subset of commonAPICLient?
func (f *fakeClient) ContainerCreate(_ context.Context,
	config *container.Config,
	hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig,
	containerName string) (container.ContainerCreateCreatedBody, error) {

	f.ContainerCreateCalledWithConfig = *config
	f.ContainerCreateCalledWithName = containerName

	return container.ContainerCreateCreatedBody{ID: "fake-container-id"}, nil
}

func (f *fakeClient) CopyToContainer(_ context.Context, id, path string, r io.Reader, c types.CopyToContainerOptions) error {
	f.ContainerCopyCalledWithConfig = c
	b, _ := ioutil.ReadAll(r)
	f.ContainerCopyCalledWithReaderString = string(b)
	f.ContainerCopyCalledWithId = id
	f.ContainerCopyCalledWithDst = path
	return nil
}

func (f *fakeClient) ContainerRemove(_ context.Context, id string, config types.ContainerRemoveOptions) error {
	f.ContainerRemoveCalledWithConfig = config
	f.ContainerRemoveCalledWithId = id
	return nil
}

func (f *fakeClient) ContainerStart(_ context.Context, id string, config types.ContainerStartOptions) error {
	f.ContainerStartCalledWithConfig = config
	f.ContainerStartCalledWithID = id
	return nil
}

func (f *fakeClient) ContainerWait(_ context.Context, id string, wait container.WaitCondition) (<-chan container.ContainerWaitOKBody, <-chan error) {
	ok := make(chan container.ContainerWaitOKBody)
	close(ok)

	f.ContainerWaitCalledWithID = id
	f.ContainerWaitCalledWithCondition = wait

	return ok, make(chan error)
}

func (f *fakeClient) ContainerCommit(_ context.Context, id string, config types.ContainerCommitOptions) (types.IDResponse, error) {
	f.ContainerCommitCalledWithID = id
	f.ContainerCommitCalledWithReference = config.Reference
	return types.IDResponse{ID: "fake-commit-id"}, nil
}

func (f *fakeClient) ContainerLogs(_ context.Context, id string, config types.ContainerLogsOptions) (io.ReadCloser, error) {
	f.ContainerLogsCalledWithConfig = config
	f.ContainerLogsCalledWithId = id
	buff := bytes.NewBufferString("fake logs")
	return ioutil.NopCloser(buff), nil
}

func (f *fakeClient) ImageRemove(_ context.Context, imgName string, _ types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
	f.ImageDeleteCalledWithName = imgName
	return make([]types.ImageDeleteResponseItem, 0), nil
}

func TestImageExtenderSucceeds(t *testing.T) {
	client := fakeClient{}
	buf := bytes.NewBufferString("toml-tar-file-contents")
	logErr := &bytes.Buffer{}
	logOut := &bytes.Buffer{}
	extender := extend.ImageExtender{
		Kind:          "fake-kind",
		ExtendToml:    buf,
		Client:        &client,
		BaseImageName: "fake-image",
		Logger:        logging.NewLogWithWriters(logOut, logErr),
		LogCopy: func(destOut, destErr io.Writer, src io.Reader) (n int64, err error) { // In docker, logs are multiplexed. we're ignoring that here
			return io.Copy(io.MultiWriter(destOut, destErr), src)
		},
	}

	extender.Extend(context.Background())

	h.AssertEq(t, client.ContainerCreateCalledWithConfig.Image, "fake-image")
	h.AssertEq(t, client.ContainerCreateCalledWithConfig.User, "0")
	h.AssertIncludeAllExpectedPatterns(
		t,
		client.ContainerCreateCalledWithConfig.Cmd,
		[]string{"/cnb/image/fake-kind/extend", "/cnb/image/fake-kind/extend.toml"},
	)
	h.AssertContains(t, client.ContainerCreateCalledWithName, "pack.local/extend/container")

	h.AssertEq(t, client.ContainerRemoveCalledWithId, "fake-container-id")
	h.AssertEq(t, client.ContainerRemoveCalledWithConfig.Force, true)

	h.AssertEq(t, client.ContainerCopyCalledWithId, "fake-container-id")
	h.AssertEq(t, client.ContainerCopyCalledWithDst, "/cnb/image/fake-kind")
	h.AssertEq(t, client.ContainerCopyCalledWithReaderString, "toml-tar-file-contents")
	h.AssertEq(t, client.ContainerCopyCalledWithConfig, types.CopyToContainerOptions{})

	h.AssertEq(t, client.ContainerStartCalledWithID, "fake-container-id")
	h.AssertEq(t, client.ContainerStartCalledWithConfig, types.ContainerStartOptions{})

	h.AssertEq(t, client.ContainerWaitCalledWithID, "fake-container-id")
	h.AssertEq(t, client.ContainerWaitCalledWithCondition, container.WaitConditionNotRunning)

	h.AssertEq(t, client.ContainerCommitCalledWithID, "fake-container-id")
	h.AssertContains(t, client.ContainerCommitCalledWithReference, "pack.local/extend/commit/")

	h.AssertEq(t, client.ContainerLogsCalledWithId, "fake-container-id")
	h.AssertEq(t, client.ContainerLogsCalledWithConfig, types.ContainerLogsOptions{
		ShowStderr: true,
		ShowStdout: true,
	})

	h.AssertMatch(t, logOut.String(), "fake logs")
	h.AssertMatch(t, logErr.String(), "fake logs")

	h.AssertContains(t, client.ImageDeleteCalledWithName, "pack.local/extend/commit")
}
