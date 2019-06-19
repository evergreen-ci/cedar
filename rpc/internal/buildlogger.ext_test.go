package internal

import (
	"testing"

	"github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/assert"
)

func TestLogFormatExport(t *testing.T) {
	for _, test := range []struct {
		name                string
		rpcFormat           LogFormat
		expectedModelFormat model.LogFormat
	}{
		{
			name:                "Uknown",
			rpcFormat:           LogFormat_LOG_FORMAT_UNKNOWN,
			expectedModelFormat: model.LogFormatUnknown,
		},
		{
			name:                "Text",
			rpcFormat:           LogFormat_LOG_FORMAT_TEXT,
			expectedModelFormat: model.LogFormatText,
		},
		{
			name:                "JSON",
			rpcFormat:           LogFormat_LOG_FORMAT_JSON,
			expectedModelFormat: model.LogFormatJSON,
		},
		{
			name:                "BSON",
			rpcFormat:           LogFormat_LOG_FORMAT_BSON,
			expectedModelFormat: model.LogFormatBSON,
		},
		{
			name:                "Invalid",
			rpcFormat:           LogFormat(4),
			expectedModelFormat: model.LogFormatUnknown,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedModelFormat, test.rpcFormat.Export())
		})
	}
}
