/*
Package cedar holds a a number of application level constants and
shared resources for the cedar application.

Environment

The Environment interface provides a collection of application level
state that holds database sessions, a configuration object, and
loggers. There is global instance, but using the Environment interface
allows cedar components to be tested without depending on global state.
*/
package cedar

// Sink defines a number of application level constants used
// throughout the application.
const (
	QueueName       = "queue"
	ShortDateFormat = "2006-01-02T15:04"
)

// BuildRevision stores the commit in the git repository at build time
// and is specified with -ldflags at build time
var BuildRevision = ""
