// Package dockergoal is a library to reach a set of container goals. It
// provides the ability to apply "deltas" in the form of recreating containers
// as necessary as the goal configuration changes.
package dockergoal

import (
	"errors"
	"strings"

	"github.com/facebookgo/dockerutil"
	"github.com/facebookgo/stackerr"
	"github.com/samalba/dockerclient"
)

var (
	errNameMissing = errors.New("dockergoal: ContainerName is required")
)

// A Container defines a "desired" container state.
type Container struct {
	name                string
	containerConfig     *dockerclient.ContainerConfig
	hostConfig          *dockerclient.HostConfig
	removeExisting      bool
	forceRemoveExisting bool
	authConfig          *dockerclient.AuthConfig
	afterCreate         func(string) error
}

// ContainerOption configure options for a container.
type ContainerOption func(c *Container) error

// NewContainer creates a new desired container state. This is only the desired
// state, it isn't applied until Apply is called.
func NewContainer(options ...ContainerOption) (*Container, error) {
	var c Container
	for _, o := range options {
		if err := o(&c); err != nil {
			return nil, err
		}
	}
	if c.name == "" {
		return nil, errNameMissing
	}
	return &c, nil
}

// ContainerName configures the name of the container.
func ContainerName(name string) ContainerOption {
	return func(c *Container) error {
		c.name = name
		return nil
	}
}

// ContainerRemoveExisting removes an existing container if necessary. If this
// is option is not specified, and an existing container with a different
// configuration exists, it will not be removed and an error will be returned.
// If this option is specified, the existing container will be removed and a
// new container will be created.
func ContainerRemoveExisting() ContainerOption {
	return func(c *Container) error {
		c.removeExisting = true
		return nil
	}
}

// ContainerForceRemoveExisting forces removing the existing container if one
// is found. It does so even if the existing container matches the desired
// state.
func ContainerForceRemoveExisting() ContainerOption {
	return func(c *Container) error {
		c.forceRemoveExisting = true
		return nil
	}
}

// ContainerConfig specifies the container configuration.
func ContainerConfig(config *dockerclient.ContainerConfig) ContainerOption {
	return func(c *Container) error {
		c.containerConfig = config
		return nil
	}
}

// ContainerHostConfig specifies the container host configuration.
func ContainerHostConfig(config *dockerclient.HostConfig) ContainerOption {
	return func(c *Container) error {
		c.hostConfig = config
		return nil
	}
}

// ContainerAuthConfig specifies the auth credentials used when pulling an
// image.
func ContainerAuthConfig(ac *dockerclient.AuthConfig) ContainerOption {
	return func(c *Container) error {
		c.authConfig = ac
		return nil
	}
}

// ContainerAfterCreate specifies a function which is invoked when a new
// container is created. It is not called if an existing running container with
// the desired state was found.
func ContainerAfterCreate(f func(containerID string) error) ContainerOption {
	return func(c *Container) error {
		c.afterCreate = f
		return nil
	}
}

// Apply creates the container, possibly removing it as necessary based on the
// container options that were set.
func (c *Container) Apply(docker dockerclient.Client) error {
	ci, err := docker.InspectContainer(c.name)
	createIt := false

	if err != nil {
		// unknown error, bail
		if err != dockerclient.ErrNotFound {
			return stackerr.Wrap(err)
		}

		// container does not exist, create it
		createIt = true
	} else {
		// force remove existing
		if c.forceRemoveExisting {
			if err := docker.RemoveContainer(ci.Id, true, false); err != nil {
				return stackerr.Wrap(err)
			}
			createIt = true
		} else {
			// already exists, check it
			ok, err := c.checkExisting(docker, ci)
			if err != nil {
				return err
			}
			if ok {
				// existing container is good
				if ci.State.Running {
					// and it's already running, so we do nothing
					return nil
				}
				// keep going, we need to start it
			} else {
				// existing container is not acceptable, remove it and create a new one
				if err := docker.RemoveContainer(ci.Id, true, false); err != nil {
					return stackerr.Wrap(err)
				}
				createIt = true
			}
		}
	}

	// container needs to be created
	if createIt {
		_, err := dockerutil.CreateWithPull(docker, c.containerConfig, c.name, c.authConfig)
		if err != nil {
			return err
		}

		ci, err = docker.InspectContainer(c.name)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}

	// start the container
	err = docker.StartContainer(ci.Id, c.hostConfig)
	if err != nil {
		return stackerr.Wrap(err)
	}

	if createIt && c.afterCreate != nil {
		if err := c.afterCreate(ci.Id); err != nil {
			docker.RemoveContainer(ci.Id, true, false)
			return stackerr.Wrap(err)
		}
	}

	return nil
}

func (c *Container) checkExisting(docker dockerclient.Client, current *dockerclient.ContainerInfo) (bool, error) {
	if equal, err := c.checkExistingImage(docker, current); !equal || err != nil {
		return false, err
	}
	if equal, err := c.checkExistingDNS(current); !equal || err != nil {
		return false, err
	}
	if equal, err := c.checkExistingCmd(current); !equal || err != nil {
		return false, err
	}
	if equal, err := c.checkExistingEnv(current); !equal || err != nil {
		return false, err
	}
	if equal, err := c.checkExistingBinds(current); !equal || err != nil {
		return false, err
	}
	// we're running with the desired configuration
	return true, nil
}

func (c *Container) checkExistingImage(docker dockerclient.Client, current *dockerclient.ContainerInfo) (bool, error) {
	// image comparison is by ID, so we need to find the ID of our desired image
	desiredImageID, err := dockerutil.ImageID(docker, c.containerConfig.Image, c.authConfig)
	if err != nil {
		return false, err
	}
	if current.Image != desiredImageID {
		// if we aren't allowed to remove the existing container, consider this a failure
		if !c.removeExisting {
			return false, stackerr.Newf(
				"container %q running with image %q but desired image is %q with id %q",
				c.name,
				current.Image,
				c.containerConfig.Image,
				desiredImageID,
			)
		}

		// trigger removing the existing container and starting a new one
		return false, nil
	}
	return true, nil
}

func (c *Container) checkExistingDNS(current *dockerclient.ContainerInfo) (bool, error) {
	var currentDNS, desiredDNS []string
	if c.hostConfig != nil {
		desiredDNS = c.hostConfig.Dns
	}
	if current.HostConfig != nil {
		currentDNS = current.HostConfig.Dns
	}
	if !equalStrSlice(currentDNS, desiredDNS) {
		// if we aren't allowed to remove the existing container, consider this a failure
		if !c.removeExisting {
			return false, stackerr.Newf(
				"container %q running with DNS %v but desired DNS is %v",
				c.name,
				currentDNS,
				desiredDNS,
			)
		}

		// trigger removing the existing container and starting a new one
		return false, nil
	}
	return true, nil
}

func (c *Container) checkExistingCmd(current *dockerclient.ContainerInfo) (bool, error) {
	// we only check the tail of the current command matches because it includes
	// the entrypoint defined in the container dockerfile as well.
	if !hasSuffixStrSlice(current.Config.Cmd, c.containerConfig.Cmd) {
		// if we aren't allowed to remove the existing container, consider this a failure
		if !c.removeExisting {
			return false, stackerr.Newf(
				"container %q running with command %v but desired command is %v",
				c.name,
				current.Config.Cmd,
				c.containerConfig.Cmd,
			)
		}

		// trigger removing the existing container and starting a new one
		return false, nil
	}
	return true, nil
}

func (c *Container) checkExistingEnv(current *dockerclient.ContainerInfo) (bool, error) {
	// we only check for a subset because the current env includes the
	// environment variables defined the container dockerfile as well.
	if !strSliceSubset(current.Config.Env, c.containerConfig.Env) {
		// if we aren't allowed to remove the existing container, consider this a failure
		if !c.removeExisting {
			return false, stackerr.Newf(
				"container %q running with env %v but desired env is %v",
				c.name,
				current.Config.Env,
				c.containerConfig.Env,
			)
		}

		// trigger removing the existing container and starting a new one
		return false, nil
	}
	return true, nil
}

func (c *Container) checkExistingBinds(current *dockerclient.ContainerInfo) (bool, error) {
	// we only check for a subset because the current volumes includes the
	// ones defined the container dockerfile as well.
	if c.hostConfig != nil && !strSliceSubset(flattenVolumes(current.Volumes), c.hostConfig.Binds) {
		// if we aren't allowed to remove the existing container, consider this a failure
		if !c.removeExisting {
			return false, stackerr.Newf(
				"container %q running with volumes %v but desired volumes are %v",
				c.name,
				current.Volumes,
				c.hostConfig.Binds,
			)
		}

		// trigger removing the existing container and starting a new one
		return false, nil
	}
	return true, nil
}

// ApplyGraph creates all the specified containers. It handles links making
// sure the dependencies are created in the right order.
func ApplyGraph(docker dockerclient.Client, containers []*Container) error {
	known := map[string]struct{}{}
	started := map[string]bool{}

	// we want to know all known names so we can detect malformed links
	for _, c := range containers {
		known[c.name] = struct{}{}
	}

	// keep doing rounds of starts until we're all done or error out
	pending := containers
	for len(pending) > 0 {
		var starting []string
		var nextRound []*Container

	pendingLoop:
		for _, c := range pending {
			if c.hostConfig != nil {
				// TODO: also include c.hostConfig.VolumesFrom
				for _, link := range c.hostConfig.Links {
					// only care about the name, not the alias
					parts := strings.Split(link, ":")

					// make sure the link is known
					if _, ok := known[parts[0]]; !ok {
						return stackerr.Newf("%q expects unknown link %q", c.name, link)
					}

					// we need to wait for a dependency, schedule for the next round
					if !started[parts[0]] {
						nextRound = append(nextRound, c)
						continue pendingLoop
					}
				}
			}

			starting = append(starting, c.name)
			if err := c.Apply(docker); err != nil {
				return err
			}
		}

		// we successfully started them
		for _, n := range starting {
			started[n] = true
		}

		// move on to the next round
		pending = nextRound
	}

	return nil
}

func equalStrSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func hasSuffixStrSlice(s, suffix []string) bool {
	lenSuffix := len(suffix)
	lenS := len(s)
	if lenSuffix > lenS {
		return false
	}
	return equalStrSlice(s[lenS-lenSuffix:], suffix)
}

func strSliceSubset(big, subset []string) bool {
	for _, v := range subset {
		if !containsStr(big, v) {
			return false
		}
	}
	return true
}

func containsStr(all []string, target string) bool {
	for _, v := range all {
		if v == target {
			return true
		}
	}
	return false
}

func flattenVolumes(volumes map[string]string) []string {
	res := make([]string, len(volumes))
	for k, v := range volumes {
		res = append(res, v+":"+k)
	}
	return res
}
