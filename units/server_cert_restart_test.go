package units

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/evergreen-ci/cedar"
	"github.com/evergreen-ci/cedar/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerCertRestartJob(t *testing.T) {
	env := cedar.GetEnvironment()
	tempDir, err := ioutil.TempDir(".", "server-cert-restart")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

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
		hasCancel   bool
		hasErr      bool
	}{
		{
			name:        "Unset0",
			confVersion: 0,
			envVersion:  -1,
			hasCancel:   true,
		},
		{
			name:        "NoCancelFunc",
			confVersion: 1,
			envVersion:  0,
			hasErr:      true,
		},
		{
			name:        "Outdated",
			confVersion: 5,
			envVersion:  2,
			hasCancel:   true,
		},
		{
			name:        "UpToDate",
			confVersion: 10,
			envVersion:  10,
			hasCancel:   true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			conf := model.NewCedarConfig(env)
			require.NoError(t, conf.Find())
			conf.CA.ServerCertVersion = test.confVersion
			require.NoError(t, conf.Save())
			env.SetServerCertVersion(test.envVersion)

			ctx, cancel := context.WithCancel(context.Background())
			if test.hasCancel {
				env.SetCancel(cancel)
			} else {
				env.SetCancel(nil)
			}

			j := NewServerCertRestartJob()
			j.Run(ctx)

			if test.hasErr {
				assert.Error(t, j.Error())
				assert.NoError(t, ctx.Err())
			} else {
				assert.NoError(t, j.Error())
				if test.envVersion >= 0 && test.confVersion > test.envVersion {
					assert.Error(t, ctx.Err())
				}
			}
		})
	}
}
