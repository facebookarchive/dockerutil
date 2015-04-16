// Package dockergoal is a library to reach a set of container goals.
package dockergoal

import (
	"strings"

	"github.com/facebookgo/dockerutil"
	"github.com/facebookgo/errgroup"
	"github.com/facebookgo/stackerr"
	"github.com/samalba/dockerclient"
)

// A Container defines a "desired" container.
type Container struct {
	name                string
	containerConfig     *dockerclient.ContainerConfig
	hostConfig          *dockerclient.HostConfig
	removeExisting      bool
	forceRemoveExisting bool
	checkRunningImage   bool
	authConfig          *dockerclient.AuthConfig
}

type ContainerOption func(c *Container) error

func NewContainer(options ...ContainerOption) (*Container, error) {
	var c Container
	for _, o := range options {
		if err := o(&c); err != nil {
			return nil, err
		}
	}
	return &c, nil
}

func ContainerName(name string) ContainerOption {
	return func(c *Container) error {
		c.name = name
		return nil
	}
}

func ContainerRemoveExisting(c *Container) error {
	c.removeExisting = true
	return nil
}

func ContainerForceRemoveExisting(c *Container) error {
	c.forceRemoveExisting = true
	return nil
}

func ContainerConfig(config *dockerclient.ContainerConfig) ContainerOption {
	return func(c *Container) error {
		c.containerConfig = config
		return nil
	}
}

func ContainerHostConfig(config *dockerclient.HostConfig) ContainerOption {
	return func(c *Container) error {
		c.hostConfig = config
		return nil
	}
}

// ContainerCheckRunningImage will trigger checking of the running image ID
// with the goal image ID.
func ContainerCheckRunningImage(c *Container) error {
	c.checkRunningImage = true
	return nil
}

func ContainerAuthConfig(ac *dockerclient.AuthConfig) ContainerOption {
	return func(c *Container) error {
		c.authConfig = ac
		return nil
	}
}

func (c *Container) Apply(docker dockerclient.Client) error {
	ci, err := docker.InspectContainer(c.name)

	// force remove existing
	if c.forceRemoveExisting {
		if err := docker.RemoveContainer(ci.Id, true, false); err != nil {
			return stackerr.Wrap(err)
		}
		// otherwise we just removed the running container and want to start a new one
		err = dockerclient.ErrNotFound
	}

	// already running
	if err == nil && ci.State.Running {
		if ok, err := c.checkRunning(docker, ci); err != nil {
			return err
		} else if ok {
			return nil
		}

		// otherwise we just removed the running container and want to start a new one
		err = dockerclient.ErrNotFound
	}

	// unknown error, bail
	if err != nil && err != dockerclient.ErrNotFound {
		return stackerr.Wrap(err)
	}

	// container does not exist, create it
	if err == dockerclient.ErrNotFound {
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

	return nil
}

func (c *Container) checkRunning(docker dockerclient.Client, current *dockerclient.ContainerInfo) (bool, error) {
	// only do this check if configured to do so, otherwise consider the running container ok
	if !c.checkRunningImage {
		return true, nil
	}

	// image comparison is by ID, so we need to find the ID of our desired image
	desiredImageID, err := dockerutil.ImageID(docker, c.containerConfig.Image, nil)
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

		// otherwise remove it since it isn't want we want
		if err := docker.RemoveContainer(current.Id, true, false); err != nil {
			return false, stackerr.Wrap(err)
		}

		// trigger starting a new container
		return false, nil
	}

	// we're running the correct image
	return true, nil
}

func ApplyGraph(docker dockerclient.Client, containers []*Container) error {
	known := map[string]struct{}{}
	started := map[string]bool{}

	// we want to know all known names so we can detect malformed links
	for _, c := range containers {
		known[c.name] = struct{}{}
	}

	// TODO: parallel pull pass?

	// keep doing rounds of parallel starts until we're all done or error out
	pending := containers
	for len(pending) > 0 {
		var eg errgroup.Group
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
						return stackerr.Newf("%s expects unknown link %s", c.name, link)
					}

					// we need to wait for a dependency, schedule for the next round
					if !started[parts[0]] {
						nextRound = append(nextRound, c)
						continue pendingLoop
					}
				}
			}

			starting = append(starting, c.name)
			eg.Add(1)
			go func(c *Container) {
				defer eg.Done()
				if err := c.Apply(docker); err != nil {
					eg.Error(err)
				}
			}(c)
		}

		// now wait for all the containers we started in parallel
		if err := eg.Wait(); err != nil {
			return err
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
