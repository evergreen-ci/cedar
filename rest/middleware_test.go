package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateEvgAuthRequest(t *testing.T) {
	evgConf := &model.EvergreenConfig{
		URL:             "https://evergreen.mongodb.com",
		AuthTokenCookie: "mci-token",
		HeaderKeyName:   "Api-Key",
		HeaderUserName:  "Username",
	}
	resourceID := "project"
	expectedURL := fmt.Sprintf("%s/rest/v2/auth?resource=%s&resource_type=project&permission=project_logs&required_level=10", evgConf.URL, resourceID)

	t.Run("NoHeaderAndNoCookie", func(t *testing.T) {
		originalReq, err := http.NewRequest(http.MethodGet, "https://cedar.mongodb.com", nil)
		require.NoError(t, err)
		req, errResp := createEvgAuthRequest(context.TODO(), originalReq, evgConf, resourceID)
		assert.Nil(t, req)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status())
	})
	t.Run("NoHeaderButCookie", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		originalReq, err := http.NewRequest(http.MethodGet, "https://cedar.mongodb.com", nil)
		require.NoError(t, err)
		originalCookie := &http.Cookie{Name: evgConf.AuthTokenCookie, Value: "value"}
		originalReq.AddCookie(originalCookie)
		req, errResp := createEvgAuthRequest(ctx, originalReq, evgConf, resourceID)
		require.Nil(t, errResp)
		require.NotNil(t, req)
		assert.Equal(t, expectedURL, req.URL.String())
		assert.Equal(t, req.Context(), ctx)
		cookie, err := originalReq.Cookie(evgConf.AuthTokenCookie)
		require.NoError(t, err)
		assert.Equal(t, originalCookie, cookie)
		require.Len(t, req.Header[evgConf.HeaderKeyName], 0)
		require.Len(t, req.Header[evgConf.HeaderUserName], 0)
	})
	t.Run("HeaderButNoCookie", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		originalReq, err := http.NewRequest(http.MethodGet, "https://cedar.mongodb.com", nil)
		require.NoError(t, err)
		originalReq.Header.Set(evgConf.HeaderKeyName, "key")
		originalReq.Header.Set(evgConf.HeaderUserName, "user")
		req, errResp := createEvgAuthRequest(ctx, originalReq, evgConf, resourceID)
		require.Nil(t, errResp)
		require.NotNil(t, req)
		assert.Equal(t, expectedURL, req.URL.String())
		assert.Equal(t, req.Context(), ctx)
		_, err = originalReq.Cookie(evgConf.AuthTokenCookie)
		assert.Error(t, err)
		require.Len(t, req.Header[evgConf.HeaderKeyName], 1)
		assert.Equal(t, "key", req.Header[evgConf.HeaderKeyName][0])
		require.Len(t, req.Header[evgConf.HeaderUserName], 1)
		assert.Equal(t, "user", req.Header[evgConf.HeaderUserName][0])
	})
	t.Run("HeaderAndCookie", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		originalReq, err := http.NewRequest(http.MethodGet, "https://cedar.mongodb.com", nil)
		require.NoError(t, err)
		originalCookie := &http.Cookie{Name: evgConf.AuthTokenCookie, Value: "value"}
		originalReq.AddCookie(originalCookie)
		originalReq.Header.Set(evgConf.HeaderKeyName, "key")
		originalReq.Header.Set(evgConf.HeaderUserName, "user")
		req, errResp := createEvgAuthRequest(ctx, originalReq, evgConf, resourceID)
		require.Nil(t, errResp)
		require.NotNil(t, req)
		assert.Equal(t, expectedURL, req.URL.String())
		assert.Equal(t, req.Context(), ctx)
		cookie, err := originalReq.Cookie(evgConf.AuthTokenCookie)
		require.NoError(t, err)
		assert.Equal(t, originalCookie, cookie)
		require.Len(t, req.Header[evgConf.HeaderKeyName], 1)
		assert.Equal(t, "key", req.Header[evgConf.HeaderKeyName][0])
		require.Len(t, req.Header[evgConf.HeaderUserName], 1)
		assert.Equal(t, "user", req.Header[evgConf.HeaderUserName][0])
	})
}

func TestDoEvgAuthRequest(t *testing.T) {
	handler := &evgAuthMockHandler{}
	server := httptest.NewServer(handler)
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	t.Run("Unauthorized", func(t *testing.T) {
		handler.returnUnauthorized = true
		resp := doEvgAuthRequest(req, "project")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.Status())
		handler.returnUnauthorized = false
	})
	t.Run("False", func(t *testing.T) {
		resp := doEvgAuthRequest(req, "project")
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.Status())
	})
	t.Run("True", func(t *testing.T) {
		handler.returnTrue = true
		resp := doEvgAuthRequest(req, "project")
		assert.Nil(t, resp)
		handler.returnTrue = false
	})

}

type evgAuthMockHandler struct {
	returnUnauthorized bool
	returnTrue         bool
}

func (h *evgAuthMockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.returnUnauthorized {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	_, _ = w.Write([]byte(fmt.Sprintf("%v", h.returnTrue)))
}
