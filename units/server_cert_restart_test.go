package units

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerCertRestartJob(t *testing.T) {
	tempDir, err := ioutil.TempDir(".", "server-cert-restart")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tempDir))
	}()
	dbName := "cert-restart"

	env, err := cedar.NewEnvironment(context.Background(), dbName, &cedar.Configuration{
		MongoDBURI:    "mongodb://localhost:27017",
		DatabaseName:  dbName,
		SocketTimeout: time.Minute,
		NumWorkers:    2,
	})
	baseConfigFile := filepath.Join(tempDir, "base_config.yaml")
	f, err := os.Create(baseConfigFile)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	baseConfig, err := model.LoadCedarConfig(baseConfigFile)
	require.NoError(t, err)
	baseConfig.Setup(env)
	require.NoError(t, baseConfig.Save())

	for _, test := range []struct {
		name        string
		confVersion int
		envVersion  int
		hasCloser   bool
		restart     bool
		returnErr   error
	}{
		{
			name:        "Unset",
			confVersion: 0,
			envVersion:  -1,
			hasCloser:   true,
		},
		{
			name:        "NoCloserFunc",
			confVersion: 1,
			envVersion:  0,
		},
		{
			name:        "CloserReturnError",
			confVersion: 2,
			envVersion:  1,
			hasCloser:   true,
			restart:     true,
			returnErr:   errors.New("dummy err"),
		},
		{
			name:        "Outdated",
			confVersion: 1,
			envVersion:  0,
			hasCloser:   true,
			restart:     true,
		},
		{
			name:        "UpToDate",
			confVersion: 10,
			envVersion:  10,
			hasCloser:   true,
		},
		{
			name:        "AheadOfDate",
			confVersion: 0,
			envVersion:  1,
			hasCloser:   true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			env, err = cedar.NewEnvironment(context.Background(), dbName, &cedar.Configuration{
				MongoDBURI:    "mongodb://localhost:27017",
				DatabaseName:  dbName,
				SocketTimeout: time.Minute,
				NumWorkers:    2,
			})
			require.NoError(t, err)
			envCtx, envCancel := env.Context()
			defer envCancel()
			conf := model.NewCedarConfig(env)
			require.NoError(t, conf.Find())
			conf.CA.ServerCertVersion = test.confVersion
			require.NoError(t, conf.Save())
			env.SetServerCertVersion(test.envVersion)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if test.hasCloser {
				env.RegisterCloser("web-services-closer", func(_ context.Context) error {
					cancel()
					return test.returnErr
				})
			}

			j := NewServerCertRestartJob()
			j.(*serverCertRestartJob).env = env
			j.Run(envCtx)

			if test.returnErr != nil {
				assert.Error(t, j.Error())
			} else {
				assert.NoError(t, j.Error())
			}
			if test.restart {
				assert.Error(t, ctx.Err())
			} else {
				assert.NoError(t, ctx.Err())
			}
		})
	}
}
