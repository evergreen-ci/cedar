package rest

import (
	"context"
	"net/http"
	"testing"

	"github.com/evergreen-ci/cedar/rest/data"
	"github.com/evergreen-ci/cedar/rest/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerfGetByIdHandler(t *testing.T) {
	sc := data.MockConnector{
		CachedPerformanceResults: map[string]model.APIPerformanceResult{
			"abc": model.APIPerformanceResult{Name: model.ToAPIString("abc")},
		},
	}
	routeHandler := makeGetPerfById(&sc)

	t.Run("IdFound", func(t *testing.T) {
		routeHandler.(*perfGetByIdHandler).id = "abc"
		expected := sc.CachedPerformanceResults["abc"]

		resp := routeHandler.Run(context.TODO())
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.Status())
		require.NotNil(t, resp.Data())
		assert.Equal(t, &expected, resp.Data())
	})

	t.Run("IdNotFound", func(t *testing.T) {
		routeHandler.(*perfGetByIdHandler).id = "DNE"

		resp := routeHandler.Run(context.TODO())
		require.NotNil(t, resp)
		assert.NotEqual(t, http.StatusOK, resp.Status())
	})
}
