package model

// Model defines how to transform to and from an API model for a given
// interface.
type Model interface {
	Import(interface{}) error
	Export() (interface{}, error)
}
