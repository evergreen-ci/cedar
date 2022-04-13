/*
Package cedar holds a a number of application level constants and shared
resources for the Cedar application.
*/
package cedar

import (
	"time"

	"github.com/evergreen-ci/gimlet"
)

const (
	ShortDateFormat = "2006-01-02T15:04"
)

// BuildRevision stores the commit in the git repository at build time and is
// specified with -ldflags at build time.
var BuildRevision = ""

const (
	// Auth constants.
	AuthTokenCookie        = "cedar-token"
	APIUserHeader          = "Api-User"
	APIKeyHeader           = "Api-Key"
	EvergreenAPIUserHeader = "Evergreen-Api-User"
	EvergreenAPIKeyHeader  = "Evergreen-Api-Key"
	TokenExpireAfter       = time.Hour

	// Version requester types.
	PatchVersionRequester       = "patch_request"
	GithubPRRequester           = "github_pull_request"
	GitTagRequester             = "git_tag_request"
	RepotrackerVersionRequester = "gitter_request"
	TriggerRequester            = "trigger_request"
	MergeTestRequester          = "merge_test"
	AdHocRequester              = "ad_hoc"

	// Stats cache names.
	StatsCacheBuildlogger = "buildlogger"
	StatsCacheTestResults = "test_results"
	StatsCachePerf        = "perf"
)

var (
	// Convenience slice for patch requester types.
	PatchRequesters = []string{
		PatchVersionRequester,
		GithubPRRequester,
		MergeTestRequester,
	}

	// Convenience slice for slice cache names.
	StatsCacheNames = []string{
		StatsCacheBuildlogger,
		StatsCacheTestResults,
		StatsCachePerf,
	}
)

// GetUserMiddlewareConfiguration returns the gimlet user middleware config
// with cedar's global auth constants.
func GetUserMiddlewareConfiguration() gimlet.UserMiddlewareConfiguration {
	return gimlet.UserMiddlewareConfiguration{
		HeaderKeyName:  APIKeyHeader,
		HeaderUserName: APIUserHeader,
		CookieName:     AuthTokenCookie,
		CookiePath:     "/",
		CookieTTL:      TokenExpireAfter,
	}
}
