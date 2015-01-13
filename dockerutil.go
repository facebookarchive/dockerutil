package dockerutil

import (
	"github.com/facebookgo/stackerr"
	"github.com/samalba/dockerclient"
)

// ImageID returns the image ID for the given image name. If the imageName is
// not known, it will also attempt to pull the image as well.
func ImageID(d *dockerclient.DockerClient, imageName string, auth *dockerclient.AuthConfig) (string, error) {
	id, err := imageIDFromList(d, imageName)
	if err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}

	if err := d.PullImage(imageName, auth); err != nil {
		return "", stackerr.Wrap(err)
	}

	id, err = imageIDFromList(d, imageName)
	if err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}

	return "", stackerr.Newf("image named %q could not be identified", imageName)
}

func imageIDFromList(d *dockerclient.DockerClient, imageName string) (string, error) {
	images, err := d.ListImages()
	if err != nil {
		return "", stackerr.Wrap(err)
	}

	for _, i := range images {
		for _, t := range i.RepoTags {
			if t == imageName {
				return i.Id, nil
			}
		}
	}

	return "", nil
}
