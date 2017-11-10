package sink

// Configuration defines
type Configuration struct {
	BucketName    string
	DatabaseName  string
	MongoDBURI    string
	UseLocalQueue bool
	NumWorkers    int
}
