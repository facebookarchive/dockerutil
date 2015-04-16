package dockergoal

import (
	"errors"
	"testing"

	"github.com/facebookgo/ensure"
	"github.com/facebookgo/stackerr"
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

func TestContainerNameMissing(t *testing.T) {
	c, err := NewContainer()
	ensure.DeepEqual(t, err, errNameMissing)
	ensure.True(t, c == nil)
}

func TestContainerRemoveExisting(t *testing.T) {
	c, err := NewContainer(
		ContainerName("x"),
		ContainerRemoveExisting(),
	)
	ensure.Nil(t, err)
	ensure.True(t, c.removeExisting)
}

func TestContainerForceRemoveExisting(t *testing.T) {
	c, err := NewContainer(
		ContainerName("x"),
		ContainerForceRemoveExisting(),
	)
	ensure.Nil(t, err)
	ensure.True(t, c.forceRemoveExisting)
}

func TestContainerConfig(t *testing.T) {
	config := &dockerclient.ContainerConfig{}
	c, err := NewContainer(
		ContainerName("x"),
		ContainerConfig(config),
	)
	ensure.Nil(t, err)
	ensure.True(t, c.containerConfig == config)
}

func TestContainerHostConfig(t *testing.T) {
	config := &dockerclient.HostConfig{}
	c, err := NewContainer(
		ContainerName("x"),
		ContainerHostConfig(config),
	)
	ensure.Nil(t, err)
	ensure.True(t, c.hostConfig == config)
}

func TestContainerAuthConfig(t *testing.T) {
	config := &dockerclient.AuthConfig{}
	c, err := NewContainer(
		ContainerName("x"),
		ContainerAuthConfig(config),
	)
	ensure.Nil(t, err)
	ensure.True(t, c.authConfig == config)
}

func TestApplyMakesNew(t *testing.T) {
	const givenName = "x"
	const givenID = "y"
	givenContainerConfig := &dockerclient.ContainerConfig{Image: "foo"}
	givenHostConfig := &dockerclient.HostConfig{}
	var inspectCalls, createCalls, startCalls int
	container, err := NewContainer(
		ContainerName(givenName),
		ContainerConfig(givenContainerConfig),
		ContainerHostConfig(givenHostConfig),
	)
	ensure.Nil(t, err)
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			inspectCalls++
			switch inspectCalls {
			case 1:
				ensure.DeepEqual(t, name, givenName)
				return nil, dockerclient.ErrNotFound
			case 2:
				return &dockerclient.ContainerInfo{Id: givenID}, nil
			}
			panic("not reached")
		},
		createContainer: func(config *dockerclient.ContainerConfig, name string) (string, error) {
			createCalls++
			ensure.True(t, config == givenContainerConfig)
			ensure.DeepEqual(t, name, givenName)
			return "", nil
		},
		startContainer: func(id string, config *dockerclient.HostConfig) error {
			startCalls++
			ensure.DeepEqual(t, id, givenID)
			ensure.True(t, config == givenHostConfig)
			return nil
		},
	}
	ensure.Nil(t, container.Apply(client))
	ensure.DeepEqual(t, inspectCalls, 2)
	ensure.DeepEqual(t, createCalls, 1)
	ensure.DeepEqual(t, startCalls, 1)
}

func TestApplyForceRemoveExisting(t *testing.T) {
	const removeID = "y"
	const newID = "z"
	givenContainerConfig := &dockerclient.ContainerConfig{Image: "foo"}
	var removeCalls, inspectCalls, createCalls, startCalls int
	container, err := NewContainer(
		ContainerName("x"),
		ContainerConfig(givenContainerConfig),
		ContainerForceRemoveExisting(),
	)
	ensure.Nil(t, err)
	client := &mockClient{
		removeContainer: func(id string, force, volumes bool) error {
			removeCalls++
			ensure.DeepEqual(t, id, removeID)
			ensure.True(t, force)
			ensure.False(t, volumes)
			return nil
		},
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			inspectCalls++
			switch inspectCalls {
			case 1:
				return &dockerclient.ContainerInfo{Id: removeID}, nil
			case 2:
				return &dockerclient.ContainerInfo{Id: newID}, nil
			}
			panic("not reached")
		},
		createContainer: func(config *dockerclient.ContainerConfig, name string) (string, error) {
			createCalls++
			return "", nil
		},
		startContainer: func(id string, config *dockerclient.HostConfig) error {
			startCalls++
			ensure.DeepEqual(t, id, newID)
			return nil
		},
	}
	ensure.Nil(t, container.Apply(client))
	ensure.DeepEqual(t, removeCalls, 1)
	ensure.DeepEqual(t, inspectCalls, 2)
	ensure.DeepEqual(t, createCalls, 1)
	ensure.DeepEqual(t, startCalls, 1)
}

func TestApplyForceRemoveExistingError(t *testing.T) {
	container, err := NewContainer(
		ContainerName("x"),
		ContainerConfig(&dockerclient.ContainerConfig{Image: "foo"}),
		ContainerForceRemoveExisting(),
	)
	ensure.Nil(t, err)
	givenErr := errors.New("")
	client := &mockClient{
		removeContainer: func(id string, force, volumes bool) error {
			return givenErr
		},
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			return &dockerclient.ContainerInfo{Id: "x"}, nil
		},
	}
	err = container.Apply(client)
	ensure.True(t, stackerr.HasUnderlying(err, stackerr.Equals(givenErr)))
}
