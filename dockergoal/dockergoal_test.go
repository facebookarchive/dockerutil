package dockergoal

import (
	"errors"
	"testing"

	"github.com/facebookgo/ensure"
	"github.com/samalba/dockerclient"
)

func TestNewContainerError(t *testing.T) {
	givenErr := errors.New("")
	c, err := NewContainer(func(*Container) error {
		return givenErr
	})
	ensure.True(t, err == givenErr)
	ensure.True(t, c == nil)
}

func TestContainerName(t *testing.T) {
	const name = "foo"
	c, err := NewContainer(ContainerName(name))
	ensure.Nil(t, err)
	ensure.DeepEqual(t, c.name, name)
}

func TestContainerRemoveExisting(t *testing.T) {
	c, err := NewContainer(ContainerRemoveExisting())
	ensure.Nil(t, err)
	ensure.True(t, c.removeExisting)
}

func TestContainerForceRemoveExisting(t *testing.T) {
	c, err := NewContainer(ContainerForceRemoveExisting())
	ensure.Nil(t, err)
	ensure.True(t, c.forceRemoveExisting)
}

func TestContainerConfig(t *testing.T) {
	config := &dockerclient.ContainerConfig{}
	c, err := NewContainer(ContainerConfig(config))
	ensure.Nil(t, err)
	ensure.True(t, c.containerConfig == config)
}

func TestContainerHostConfig(t *testing.T) {
	config := &dockerclient.HostConfig{}
	c, err := NewContainer(ContainerHostConfig(config))
	ensure.Nil(t, err)
	ensure.True(t, c.hostConfig == config)
}

func TestContainerAuthConfig(t *testing.T) {
	config := &dockerclient.AuthConfig{}
	c, err := NewContainer(ContainerAuthConfig(config))
	ensure.Nil(t, err)
	ensure.True(t, c.authConfig == config)
}
