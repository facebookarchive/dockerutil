package dockergoal

import (
	"errors"
	"regexp"
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

func TestContainerAfterCreate(t *testing.T) {
	givenErr := errors.New("")
	f := func(string) error { return givenErr }
	c, err := NewContainer(
		ContainerName("x"),
		ContainerAfterCreate(f),
	)
	ensure.Nil(t, err)
	ensure.True(t, c.afterCreate("") == givenErr)
}

func TestApplyMakesNew(t *testing.T) {
	const givenName = "x"
	const givenID = "y"
	givenContainerConfig := &dockerclient.ContainerConfig{Image: "foo"}
	givenHostConfig := &dockerclient.HostConfig{}
	var inspectCalls, createCalls, startCalls, afterCreateCalls int
	container, err := NewContainer(
		ContainerName(givenName),
		ContainerConfig(givenContainerConfig),
		ContainerHostConfig(givenHostConfig),
		ContainerAfterCreate(func(containerID string) error {
			afterCreateCalls++
			ensure.DeepEqual(t, containerID, givenID)
			return nil
		}),
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
	ensure.DeepEqual(t, afterCreateCalls, 1)
	ensure.DeepEqual(t, startCalls, 1)
}

func TestApplyCreateError(t *testing.T) {
	givenErr := errors.New("")
	container, err := NewContainer(
		ContainerName("x"),
		ContainerConfig(&dockerclient.ContainerConfig{Image: "foo"}),
	)
	ensure.Nil(t, err)
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			return nil, dockerclient.ErrNotFound
		},
		createContainer: func(config *dockerclient.ContainerConfig, name string) (string, error) {
			return "", givenErr
		},
	}
	err = container.Apply(client)
	ensure.True(t, stackerr.HasUnderlying(err, stackerr.Equals(givenErr)))
}

func TestApplyAfterCreateError(t *testing.T) {
	givenErr := errors.New("")
	const givenName = "x"
	const givenID = "y"
	var inspectCalls, removeCalls int
	container, err := NewContainer(
		ContainerName(givenName),
		ContainerConfig(&dockerclient.ContainerConfig{Image: "foo"}),
		ContainerAfterCreate(func(string) error { return givenErr }),
	)
	ensure.Nil(t, err)
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			inspectCalls++
			switch inspectCalls {
			case 1:
				return nil, dockerclient.ErrNotFound
			case 2:
				return &dockerclient.ContainerInfo{Id: givenID}, nil
			}
			panic("not reached")
		},
		createContainer: func(config *dockerclient.ContainerConfig, name string) (string, error) {
			return "", nil
		},
		startContainer: func(id string, config *dockerclient.HostConfig) error {
			return nil
		},
		removeContainer: func(id string, force, volumes bool) error {
			removeCalls++
			ensure.DeepEqual(t, id, givenID)
			return nil
		},
	}
	err = container.Apply(client)
	ensure.True(t, stackerr.HasUnderlying(err, stackerr.Equals(givenErr)))
	ensure.DeepEqual(t, removeCalls, 1)
}

func TestApplyInspectAfterCreateError(t *testing.T) {
	container, err := NewContainer(
		ContainerName("x"),
		ContainerConfig(&dockerclient.ContainerConfig{Image: "foo"}),
	)
	ensure.Nil(t, err)
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			return nil, dockerclient.ErrNotFound
		},
		createContainer: func(config *dockerclient.ContainerConfig, name string) (string, error) {
			return "baz", nil
		},
	}
	err = container.Apply(client)
	ensure.True(t, stackerr.HasUnderlying(err, stackerr.Equals(dockerclient.ErrNotFound)))
}

func TestApplyStartError(t *testing.T) {
	givenErr := errors.New("")
	const image = "x"
	const id = "y"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: image,
		},
	}
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			return &dockerclient.ContainerInfo{
				Id:     "a",
				Image:  id,
				Config: &dockerclient.ContainerConfig{},
			}, nil
		},
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{image},
					Id:       id,
				},
			}, nil
		},
		startContainer: func(id string, config *dockerclient.HostConfig) error {
			return givenErr
		},
	}
	err := container.Apply(client)
	ensure.True(t, stackerr.HasUnderlying(err, stackerr.Equals(givenErr)))
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

func TestApplyForceRemoveExistingWhenNotFound(t *testing.T) {
	var inspectCalls int
	container, err := NewContainer(
		ContainerName("x"),
		ContainerConfig(&dockerclient.ContainerConfig{Image: "foo"}),
		ContainerForceRemoveExisting(),
	)
	ensure.Nil(t, err)
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			inspectCalls++
			switch inspectCalls {
			case 1:
				return nil, dockerclient.ErrNotFound
			case 2:
				return &dockerclient.ContainerInfo{Id: "y"}, nil
			}
			panic("not reached")
		},
		createContainer: func(config *dockerclient.ContainerConfig, name string) (string, error) {
			return "", nil
		},
		startContainer: func(id string, config *dockerclient.HostConfig) error {
			return nil
		},
	}
	ensure.Nil(t, container.Apply(client))
}

func TestApplyRemovesExistingWithoutDesiredImage(t *testing.T) {
	const image = "x"
	const removeID = "y"
	var inspectCalls, removeCalls int
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: image,
		},
		removeExisting: true,
	}
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			inspectCalls++
			switch inspectCalls {
			case 1:
				return &dockerclient.ContainerInfo{Id: removeID, Image: "a"}, nil
			case 2:
				return &dockerclient.ContainerInfo{Id: "y"}, nil
			}
			panic("not reached")
		},
		createContainer: func(config *dockerclient.ContainerConfig, name string) (string, error) {
			return "", nil
		},
		startContainer: func(id string, config *dockerclient.HostConfig) error {
			return nil
		},
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{image},
					Id:       "y",
				},
			}, nil
		},
		removeContainer: func(id string, force, volumes bool) error {
			removeCalls++
			ensure.DeepEqual(t, id, removeID)
			return nil
		},
	}
	ensure.Nil(t, container.Apply(client))
	ensure.DeepEqual(t, removeCalls, 1)
}

func TestApplyRemovesExistingWithoutDesiredImageError(t *testing.T) {
	givenErr := errors.New("")
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: "x",
		},
	}
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			return &dockerclient.ContainerInfo{Id: "y", Image: "a"}, nil
		},
		listImages: func() ([]*dockerclient.Image, error) {
			return nil, givenErr
		},
	}
	err := container.Apply(client)
	ensure.True(t, stackerr.HasUnderlying(err, stackerr.Equals(givenErr)))
}

func TestApplyWithWithDesiredImageAndRunning(t *testing.T) {
	const image = "x"
	const id = "y"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: image,
		},
	}
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			ci := &dockerclient.ContainerInfo{
				Id:     "a",
				Image:  id,
				Config: &dockerclient.ContainerConfig{},
			}
			ci.State.Running = true
			return ci, nil
		},
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{image},
					Id:       id,
				},
			}, nil
		},
	}
	ensure.Nil(t, container.Apply(client))
}

func TestApplyWithWithDesiredImageAndNotRunning(t *testing.T) {
	const image = "x"
	const id = "y"
	var startCalls int
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: image,
		},
	}
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			return &dockerclient.ContainerInfo{
				Id:     "a",
				Image:  id,
				Config: &dockerclient.ContainerConfig{},
			}, nil
		},
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{image},
					Id:       id,
				},
			}, nil
		},
		startContainer: func(id string, config *dockerclient.HostConfig) error {
			startCalls++
			return nil
		},
	}
	ensure.Nil(t, container.Apply(client))
	ensure.DeepEqual(t, startCalls, 1)
}

func TestApplyInitialInspectError(t *testing.T) {
	givenErr := errors.New("")
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: "x",
		},
	}
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			return nil, givenErr
		},
	}
	err := container.Apply(client)
	ensure.True(t, stackerr.HasUnderlying(err, stackerr.Equals(givenErr)))
}

func TestCheckExistingWithDesiredImage(t *testing.T) {
	const image = "x"
	const id = "y"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: image,
		},
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{image},
					Id:       id,
				},
			}, nil
		},
	}
	ci := &dockerclient.ContainerInfo{
		Image:  id,
		Config: &dockerclient.ContainerConfig{},
	}
	ok, err := container.checkExisting(client, ci)
	ensure.Nil(t, err)
	ensure.True(t, ok)
}

func TestCheckExistingWithoutDesiredImageAndNoRemoveExisting(t *testing.T) {
	const image = "x"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: image,
		},
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{image},
					Id:       "y",
				},
			}, nil
		},
	}
	ok, err := container.checkExisting(client, &dockerclient.ContainerInfo{Image: "z"})
	ensure.Err(t, err, regexp.MustCompile("but desired image is"))
	ensure.False(t, ok)
}

func TestCheckExistingImageIdentifyError(t *testing.T) {
	givenErr := errors.New("")
	const image = "x"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: image,
		},
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return nil, givenErr
		},
	}
	ok, err := container.checkExisting(client, &dockerclient.ContainerInfo{Image: "z"})
	ensure.True(t, stackerr.HasUnderlying(err, stackerr.Equals(givenErr)))
	ensure.False(t, ok)
}

func TestCheckExistingWithoutDesiredImageWithRemoveExisting(t *testing.T) {
	const image = "x"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: image,
		},
		removeExisting: true,
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{image},
					Id:       "y",
				},
			}, nil
		},
	}
	ci := &dockerclient.ContainerInfo{
		Image: "z",
		Id:    "y",
	}
	ok, err := container.checkExisting(client, ci)
	ensure.Nil(t, err)
	ensure.False(t, ok)
}

func TestCheckExistingWithoutDesiredDNSWithoutRemoveExisting(t *testing.T) {
	const imageID = "ii1"
	const imageName = "in1"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: imageName,
		},
		hostConfig: &dockerclient.HostConfig{
			Dns: []string{"a"},
		},
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{imageName},
					Id:       imageID,
				},
			}, nil
		},
	}
	ci := &dockerclient.ContainerInfo{
		Image: imageID,
		Id:    "y",
	}
	ok, err := container.checkExisting(client, ci)
	ensure.Err(t, err, regexp.MustCompile("but desired DNS is"))
	ensure.False(t, ok)
}

func TestCheckExistingWithoutDesiredDNSWithRemoveExisting(t *testing.T) {
	const imageID = "ii1"
	const imageName = "in1"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: imageName,
		},
		hostConfig: &dockerclient.HostConfig{
			Dns: []string{"a"},
		},
		removeExisting: true,
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{imageName},
					Id:       imageID,
				},
			}, nil
		},
	}
	ci := &dockerclient.ContainerInfo{
		Image: imageID,
		Id:    "y",
		HostConfig: &dockerclient.HostConfig{
			Dns: []string{"b"},
		},
	}
	ok, err := container.checkExisting(client, ci)
	ensure.Nil(t, err)
	ensure.False(t, ok)
}

func TestCheckExistingWithoutDesiredEnv(t *testing.T) {
	const imageID = "ii1"
	const imageName = "in1"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: imageName,
			Env: []string{
				"a",
			},
		},
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{imageName},
					Id:       imageID,
				},
			}, nil
		},
	}
	ci := &dockerclient.ContainerInfo{
		Image:  imageID,
		Id:     "y",
		Config: &dockerclient.ContainerConfig{},
	}
	ok, err := container.checkExisting(client, ci)
	ensure.Err(t, err, regexp.MustCompile("but desired env is"))
	ensure.False(t, ok)
}

func TestCheckExistingWithoutDesiredEnvWithRemoveExisting(t *testing.T) {
	const imageID = "ii1"
	const imageName = "in1"
	container := &Container{
		removeExisting: true,
		containerConfig: &dockerclient.ContainerConfig{
			Image: imageName,
			Env: []string{
				"a",
			},
		},
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{imageName},
					Id:       imageID,
				},
			}, nil
		},
	}
	ci := &dockerclient.ContainerInfo{
		Image:  imageID,
		Id:     "y",
		Config: &dockerclient.ContainerConfig{},
	}
	ok, err := container.checkExisting(client, ci)
	ensure.Nil(t, err)
	ensure.False(t, ok)
}

func TestCheckExistingWithoutDesiredCmd(t *testing.T) {
	const imageID = "ii1"
	const imageName = "in1"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: imageName,
			Cmd:   []string{"a"},
		},
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{imageName},
					Id:       imageID,
				},
			}, nil
		},
	}
	ci := &dockerclient.ContainerInfo{
		Image:  imageID,
		Id:     "y",
		Config: &dockerclient.ContainerConfig{},
	}
	ok, err := container.checkExisting(client, ci)
	ensure.Err(t, err, regexp.MustCompile("but desired command is"))
	ensure.False(t, ok)
}

func TestCheckExistingWithoutDesiredCmdWithRemoveExisting(t *testing.T) {
	const imageID = "ii1"
	const imageName = "in1"
	container := &Container{
		removeExisting: true,
		containerConfig: &dockerclient.ContainerConfig{
			Image: imageName,
			Cmd:   []string{"a"},
		},
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{imageName},
					Id:       imageID,
				},
			}, nil
		},
	}
	ci := &dockerclient.ContainerInfo{
		Image:  imageID,
		Id:     "y",
		Config: &dockerclient.ContainerConfig{},
	}
	ok, err := container.checkExisting(client, ci)
	ensure.Nil(t, err)
	ensure.False(t, ok)
}

func TestCheckExistingWithoutDesiredBinds(t *testing.T) {
	const imageID = "ii1"
	const imageName = "in1"
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: imageName,
		},
		hostConfig: &dockerclient.HostConfig{
			Binds: []string{"a:b"},
		},
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{imageName},
					Id:       imageID,
				},
			}, nil
		},
	}
	ci := &dockerclient.ContainerInfo{
		Image:  imageID,
		Id:     "y",
		Config: &dockerclient.ContainerConfig{},
	}
	ok, err := container.checkExisting(client, ci)
	ensure.Err(t, err, regexp.MustCompile("but desired volumes are"))
	ensure.False(t, ok)
}

func TestCheckExistingWithoutDesiredBindsWithRemoveExisting(t *testing.T) {
	const imageID = "ii1"
	const imageName = "in1"
	container := &Container{
		removeExisting: true,
		containerConfig: &dockerclient.ContainerConfig{
			Image: imageName,
		},
		hostConfig: &dockerclient.HostConfig{
			Binds: []string{"a:b"},
		},
	}
	client := &mockClient{
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{imageName},
					Id:       imageID,
				},
			}, nil
		},
	}
	ci := &dockerclient.ContainerInfo{
		Image:   imageID,
		Id:      "y",
		Config:  &dockerclient.ContainerConfig{},
		Volumes: map[string]string{"a": "d"},
	}
	ok, err := container.checkExisting(client, ci)
	ensure.Nil(t, err)
	ensure.False(t, ok)
}

func TestApplyWithExistingRemoveError(t *testing.T) {
	const image = "x"
	givenErr := errors.New("")
	container := &Container{
		containerConfig: &dockerclient.ContainerConfig{
			Image: image,
		},
		removeExisting: true,
	}
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			return &dockerclient.ContainerInfo{
				Id:    "y",
				Image: "z",
			}, nil
		},
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{image},
					Id:       "y",
				},
			}, nil
		},
		removeContainer: func(id string, force, volumes bool) error {
			return givenErr
		},
	}
	err := container.Apply(client)
	ensure.True(t, stackerr.HasUnderlying(err, stackerr.Equals(givenErr)))
}

func TestApplyGraphSingle(t *testing.T) {
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
	ensure.Nil(t, ApplyGraph(client, []*Container{container}))
	ensure.DeepEqual(t, inspectCalls, 2)
	ensure.DeepEqual(t, createCalls, 1)
	ensure.DeepEqual(t, startCalls, 1)
}

func TestApplyGraphWithLinks(t *testing.T) {
	const (
		container1Name = "n1"
		container2Name = "n2"
		imageName      = "in1"
		imageID        = "ii1"
	)
	inspectNames := make(chan string, 2)
	containers := []*Container{
		{
			name:            container1Name,
			containerConfig: &dockerclient.ContainerConfig{Image: imageName},
			hostConfig: &dockerclient.HostConfig{
				Links: []string{container2Name + ":foo"},
			},
		},
		{
			name:            container2Name,
			containerConfig: &dockerclient.ContainerConfig{Image: imageName},
		},
	}
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			inspectNames <- name
			ci := &dockerclient.ContainerInfo{
				Id:     "x",
				Image:  imageID,
				Config: &dockerclient.ContainerConfig{},
			}
			ci.State.Running = true
			return ci, nil
		},
		listImages: func() ([]*dockerclient.Image, error) {
			return []*dockerclient.Image{
				{
					RepoTags: []string{imageName},
					Id:       imageID,
				},
			}, nil
		},
	}
	ensure.Nil(t, ApplyGraph(client, containers))
	ensure.DeepEqual(t, <-inspectNames, container2Name)
	ensure.DeepEqual(t, <-inspectNames, container1Name)
}

func TestApplyGraphWithUnknownLinks(t *testing.T) {
	containers := []*Container{
		{
			name:            "n1",
			containerConfig: &dockerclient.ContainerConfig{Image: "in1"},
			hostConfig: &dockerclient.HostConfig{
				Links: []string{"baz:foo"},
			},
		},
	}
	client := &mockClient{}
	err := ApplyGraph(client, containers)
	ensure.Err(t, err, regexp.MustCompile(`expects unknown link "baz:foo"`))
}

func TestApplyGraphApplyError(t *testing.T) {
	givenErr := errors.New("")
	containers := []*Container{
		{
			name:            "n1",
			containerConfig: &dockerclient.ContainerConfig{Image: "in1"},
		},
	}
	client := &mockClient{
		inspectContainer: func(name string) (*dockerclient.ContainerInfo, error) {
			return nil, givenErr
		},
	}
	err := ApplyGraph(client, containers)
	ensure.True(t, stackerr.HasUnderlying(err, stackerr.Equals(givenErr)))
}

func TestEqualStrSlice(t *testing.T) {
	cases := []struct {
		A, B  []string
		Equal bool
	}{
		{
			Equal: true,
		},
		{
			A:     []string{},
			Equal: true,
		},
		{
			A: []string{"a"},
		},
		{
			B: []string{"a"},
		},
		{
			A: []string{"a"},
			B: []string{"b"},
		},
		{
			A: []string{"a", "b"},
			B: []string{"a"},
		},
		{
			A:     []string{"a", "b"},
			B:     []string{"a", "b"},
			Equal: true,
		},
	}

	for _, c := range cases {
		ensure.DeepEqual(t, equalStrSlice(c.A, c.B), c.Equal, c)
	}
}

func TestHasSuffixStrSlice(t *testing.T) {
	cases := []struct {
		S      []string
		Suffix []string
		Result bool
	}{
		{
			S:      []string{"a"},
			Suffix: []string{"b"},
			Result: false,
		},
		{
			S:      []string{},
			Suffix: []string{"b"},
			Result: false,
		},
		{
			S:      []string{"b"},
			Suffix: []string{},
			Result: true,
		},
		{
			Result: true,
		},
		{
			S:      []string{},
			Suffix: []string{},
			Result: true,
		},
	}

	for _, c := range cases {
		ensure.DeepEqual(t, hasSuffixStrSlice(c.S, c.Suffix), c.Result, c)
	}
}

func TestStrSliceSubset(t *testing.T) {
	cases := []struct {
		All    []string
		Subset []string
		Result bool
	}{
		{
			All:    []string{},
			Subset: []string{},
			Result: true,
		},
		{
			All:    []string{"a"},
			Subset: []string{"b"},
			Result: false,
		},
		{
			All:    []string{"a"},
			Subset: []string{"a"},
			Result: true,
		},
		{
			All:    []string{"a", "b"},
			Subset: []string{"a"},
			Result: true,
		},
	}

	for _, c := range cases {
		ensure.DeepEqual(t, strSliceSubset(c.All, c.Subset), c.Result, c)
	}
}
