package model

import (
	"testing"

	"github.com/evergreen-ci/sink"
)

type commonModelInterFace interface {
	Setup(e sink.Environment)
	IsNil() bool
	Find() error
	Save() error
}

func TestModelInterface(t *testing.T) {

}
