package dockergoal

import (
	"io"

	"github.com/samalba/dockerclient"
)

type mockClient struct {
	info                 func() (*dockerclient.Info, error)
	listContainers       func(all, size bool, filters string) ([]dockerclient.Container, error)
	inspectContainer     func(id string) (*dockerclient.ContainerInfo, error)
	createContainer      func(config *dockerclient.ContainerConfig, name string) (string, error)
	containerLogs        func(id string, options *dockerclient.LogOptions) (io.ReadCloser, error)
	containerChanges     func(id string) ([]*dockerclient.ContainerChanges, error)
	exec                 func(config *dockerclient.ExecConfig) (string, error)
	startContainer       func(id string, config *dockerclient.HostConfig) error
	stopContainer        func(id string, timeout int) error
	restartContainer     func(id string, timeout int) error
	killContainer        func(id, signal string) error
	startMonitorEvents   func(cb dockerclient.Callback, ec chan error, args ...interface{})
	stopAllMonitorEvents func()
	startMonitorStats    func(id string, cb dockerclient.StatCallback, ec chan error, args ...interface{})
	stopAllMonitorStats  func()
	version              func() (*dockerclient.Version, error)
	pullImage            func(name string, auth *dockerclient.AuthConfig) error
	loadImage            func(reader io.Reader) error
	removeContainer      func(id string, force, volumes bool) error
	listImages           func() ([]*dockerclient.Image, error)
	removeImage          func(name string) ([]*dockerclient.ImageDelete, error)
	pauseContainer       func(name string) error
	unpauseContainer     func(name string) error
}

func (m *mockClient) Info() (*dockerclient.Info, error) {
	return m.info()
}

func (m *mockClient) ListContainers(all, size bool, filters string) ([]dockerclient.Container, error) {
	return m.listContainers(all, size, filters)
}

func (m *mockClient) InspectContainer(id string) (*dockerclient.ContainerInfo, error) {
	return m.inspectContainer(id)
}

func (m *mockClient) CreateContainer(config *dockerclient.ContainerConfig, name string) (string, error) {
	return m.createContainer(config, name)
}

func (m *mockClient) ContainerLogs(id string, options *dockerclient.LogOptions) (io.ReadCloser, error) {
	return m.containerLogs(id, options)
}

func (m *mockClient) ContainerChanges(id string) ([]*dockerclient.ContainerChanges, error) {
	return m.containerChanges(id)
}

func (m *mockClient) Exec(config *dockerclient.ExecConfig) (string, error) {
	return m.exec(config)
}

func (m *mockClient) StartContainer(id string, config *dockerclient.HostConfig) error {
	return m.startContainer(id, config)
}

func (m *mockClient) StopContainer(id string, timeout int) error {
	return m.stopContainer(id, timeout)
}

func (m *mockClient) RestartContainer(id string, timeout int) error {
	return m.restartContainer(id, timeout)
}

func (m *mockClient) KillContainer(id, signal string) error {
	return m.killContainer(id, signal)
}

func (m *mockClient) StartMonitorEvents(cb dockerclient.Callback, ec chan error, args ...interface{}) {
	m.startMonitorEvents(cb, ec, args...)
}

func (m *mockClient) StopAllMonitorEvents() {
	m.stopAllMonitorEvents()
}

func (m *mockClient) StartMonitorStats(id string, cb dockerclient.StatCallback, ec chan error, args ...interface{}) {
	m.startMonitorStats(id, cb, ec, args...)
}

func (m *mockClient) StopAllMonitorStats() {
	m.stopAllMonitorStats()
}

func (m *mockClient) Version() (*dockerclient.Version, error) {
	return m.version()
}

func (m *mockClient) PullImage(name string, auth *dockerclient.AuthConfig) error {
	return m.pullImage(name, auth)
}

func (m *mockClient) LoadImage(reader io.Reader) error {
	return m.loadImage(reader)
}

func (m *mockClient) RemoveContainer(id string, force, volumes bool) error {
	return m.removeContainer(id, force, volumes)
}

func (m *mockClient) ListImages() ([]*dockerclient.Image, error) {
	return m.listImages()
}

func (m *mockClient) RemoveImage(name string) ([]*dockerclient.ImageDelete, error) {
	return m.removeImage(name)
}

func (m *mockClient) PauseContainer(name string) error {
	return m.pauseContainer(name)
}

func (m *mockClient) UnpauseContainer(name string) error {
	return m.unpauseContainer(name)
}
