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
	"github.com/evergreen-ci/certdepot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerCertRotationJob(t *testing.T) {
	ctx := context.TODO()
	env := cedar.GetEnvironment()
	tempDir, err := ioutil.TempDir(".", "server-cert-rotation")
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

	caName := "ca"
	serviceName := "service"
	d, err := certdepot.NewFileDepot(tempDir)
	require.NoError(t, err)

	for _, test := range []struct {
		name                  string
		caConf                *model.CAConfig
		expires               time.Duration
		expectedNewExpireTime time.Duration
		expectedVersion       int
		hasErr                bool
	}{
		{
			name: "InvalidDepot",
			caConf: &model.CAConfig{
				CertDepot: certdepot.BootstrapDepotConfig{},
			},
			hasErr: true,
		},
		{
			name: "ExpiringWithOpts",
			caConf: &model.CAConfig{
				CertDepot: certdepot.BootstrapDepotConfig{
					FileDepot:   tempDir,
					CAName:      caName,
					ServiceName: serviceName + "1",
					ServiceOpts: &certdepot.CertificateOptions{
						CA:         caName,
						CommonName: serviceName + "1",
						Host:       serviceName + "1",
						Expires:    24 * time.Hour,
					},
				},
			},
			expires:               24 * time.Hour,
			expectedNewExpireTime: 24 * time.Hour,
			expectedVersion:       1,
		},
		{
			name: "ExpiringWithOutOpts",
			caConf: &model.CAConfig{
				CertDepot: certdepot.BootstrapDepotConfig{
					FileDepot:   tempDir,
					CAName:      caName,
					ServiceName: serviceName + "2",
				},
				ServerCertVersion: 2,
			},
			expires:               6 * 24 * time.Hour,
			expectedNewExpireTime: 90 * 24 * time.Hour,
			expectedVersion:       3,
		},
		{
			name: "NotExpiring",
			caConf: &model.CAConfig{
				CertDepot: certdepot.BootstrapDepotConfig{
					FileDepot:   tempDir,
					CAName:      caName,
					ServiceName: serviceName + "3",
				},
			},
			expires:         8 * 24 * time.Hour,
			expectedVersion: 0,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var expectedNotAfter time.Time

			if test.caConf != nil {
				conf := model.NewCedarConfig(env)
				require.NoError(t, conf.Find())
				conf.CA = *test.caConf
				require.NoError(t, conf.Save())

				if !test.hasErr {
					d, err := certdepot.BootstrapDepot(ctx, certdepot.BootstrapDepotConfig{
						FileDepot:   tempDir,
						CAName:      caName,
						ServiceName: test.caConf.CertDepot.ServiceName,
						CAOpts: &certdepot.CertificateOptions{
							CommonName: caName,
							Expires:    365 * 24 * time.Hour,
						},
						ServiceOpts: &certdepot.CertificateOptions{
							CA:         caName,
							CommonName: test.caConf.CertDepot.ServiceName,
							Host:       test.caConf.CertDepot.ServiceName,
							Expires:    test.expires,
						},
					})
					require.NoError(t, err)
					_, expectedNotAfter, err = certdepot.ValidityBounds(d, test.caConf.CertDepot.ServiceName)
					require.NoError(t, err)
				}
			}

			j := NewServerCertRotationJob()
			j.Run(ctx)

			if test.hasErr {
				assert.Error(t, j.Error())
			} else {
				assert.NoError(t, j.Error())

				_, notAfter, err := certdepot.ValidityBounds(d, test.caConf.CertDepot.ServiceName)
				require.NoError(t, err)

				if test.expires < 7*24*time.Hour {
					after := time.Now().Add(test.expectedNewExpireTime).Add(-time.Minute)
					assert.True(t, notAfter.After(after))
				} else {
					assert.Equal(t, expectedNotAfter, notAfter)
				}
			}

			if test.caConf != nil {
				conf := model.NewCedarConfig(env)
				require.NoError(t, conf.Find())
				assert.Equal(t, test.expectedVersion, conf.CA.ServerCertVersion)
			}
		})
	}
}
