package model

import "github.com/evergreen-ci/sink"

type DataModeler interface {
	Setup(sink.Environment) error
	IsNil() bool
	// Insert() error
}
