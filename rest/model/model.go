package model

// Model defines how to transform to and from an API model for a given
// interface.
type Model interface {
	// Import transforms to an API model.
	Import(interface{}) error
	// Export transforms from an API model.
	Export() (interface{}, error)
}
