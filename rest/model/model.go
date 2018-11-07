package model

// Model defines how to transform to and from an API model for a given
// interface.
type Model interface {
	Export(interface{}) error
	Import(interface{}) error
}
