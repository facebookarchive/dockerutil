package dockerutil

import (
	"github.com/facebookgo/stackerr"
	"github.com/samalba/dockerclient"
)

// CreateWithPull is the same as CreateContainer but will pull the image if it
// isn't found and retry creating the container.
func CreateWithPull(
	d dockerclient.Client,
	c *dockerclient.ContainerConfig,
	name string,
	ac *dockerclient.AuthConfig,
) (string, error) {

	id, err := d.CreateContainer(c, name)
	if err == nil {
		return id, nil
	}

	// unknown error, bail
	if err != dockerclient.ErrNotFound {
		return "", stackerr.Wrap(err)
	}

	// need to pull the image
	if err := d.PullImage(c.Image, ac); err != nil {
		return "", stackerr.Wrap(err)
	}

	// try again with the pulled image
	id, err = d.CreateContainer(c, name)
	if err != nil {
		return "", stackerr.Wrap(err)
	}

	return id, nil
}
