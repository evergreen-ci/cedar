/*
Package sink holds a a number of application level constants and
shared resources for the sink application.

Services Cache

The sink package maintains a public interface to a shared cache of
interfaces and services for use in building tools within sink. The
sink package has no dependencies to any sub-packages, and all methods
in the public interface are thread safe.

In practice these values are set in the operations package. See
sink/operations/setup.go for details.
*/
package sink

// Sink defines a number of application level constants used
// throughout the application.
const (
	QueueName = "queue"
)

// BuildRevision stores the commit in the git repository at build time
// and is specified with -ldflags at build time
var BuildRevision = ""
