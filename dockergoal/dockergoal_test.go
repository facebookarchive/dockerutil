package dockergoal

import (
	"errors"
	"testing"

	"github.com/facebookgo/ensure"
)

func TestNewContainerError(t *testing.T) {
	givenErr := errors.New("")
	c, err := NewContainer(func(*Container) error {
		return givenErr
	})
	ensure.True(t, err == givenErr)
	ensure.True(t, c == nil)
}
