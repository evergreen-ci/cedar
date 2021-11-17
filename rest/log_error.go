package rest

import (
	"net/http"

	"github.com/evergreen-ci/gimlet"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
)

func logFindError(err error, fields message.Fields) {
	logLevel := level.Error
	if errResp, ok := err.(gimlet.ErrorResponse); ok && errResp.StatusCode == http.StatusNotFound {
		logLevel = level.Info
	}
	grip.Log(logLevel, message.WrapError(err, fields))
}
