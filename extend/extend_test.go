package extend_test

import (
	"context"
	"io"
	"testing"

	"github.com/docker/docker/api/types"

	"gotest.tools/assert"

	"github.com/buildpacks/pack/extend"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// Test that we called the extend binary
// test that we have the results of calling the extend binary
// test the error case of calling extend on a builder without extend

type fakeClient struct {
	client.CommonAPIClient
	config container.Config
}

func (f fakeClient) ClientVersion() string {
	return "fake"
}

func (f fakeClient) ContainerCreate(_ context.Context,
	config *container.Config,
	hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig,
	containerName string) (container.ContainerCreateCreatedBody, error) {

	f.config = *config

	return container.ContainerCreateCreatedBody{ID: "fake-container"}, nil
}

func (f fakeClient) CopyToContainer(_ context.Context, _, _ string, _ io.Reader, _ types.CopyToContainerOptions) error {
	return nil
}

func (f fakeClient) ContainerRemove(_ context.Context, _ string, _ types.ContainerRemoveOptions) error {
	return nil
}

func (f fakeClient) ContainerStart(_ context.Context, _ string, _ types.ContainerStartOptions) error {
	return nil
}

func (f fakeClient) ContainerWait(_ context.Context, _ string, _ container.WaitCondition) (<-chan container.ContainerWaitOKBody, <-chan error) {
	return make(chan container.ContainerWaitOKBody), make(chan error)
}

func TestImageExtender(t *testing.T) {
	client := fakeClient{}
	extender := extend.ImageExtender{
		Kind:       "fake-kind",
		ExtendToml: nil,
		Client:     client,
		BaseImage:  "fake-image",
		Logger:     nil,
	}

	extender.Extend(context.Background())

	assert.Assert(t, client.config.Image == "fake-image")

}
