package certdepot

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/square/certstrap/depot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mgo "gopkg.in/mgo.v2"
)

func getTagPath(tag *depot.Tag) string {
	if name := depot.GetNameFromCrtTag(tag); name != "" {
		return name + ".crt"
	}
	if name := depot.GetNameFromPrivKeyTag(tag); name != "" {
		return name + ".key"
	}
	if name := depot.GetNameFromCsrTag(tag); name != "" {
		return name + ".csr"
	}
	if name := depot.GetNameFromCrlTag(tag); name != "" {
		return name + ".crl"
	}
	return ""
}

func TestDepot(t *testing.T) {
	var tempDir string

	session, err := mgo.DialWithTimeout("mongodb://localhost:27017", 2*time.Second)
	require.NoError(t, err)
	session.SetSocketTimeout(time.Hour)
	databaseName := "certDepot"
	collectionName := "certs"
	defer func() {
		err = session.DB(databaseName).C(collectionName).DropCollection()
		if err != nil {
			assert.Equal(t, "ns not found", err.Error())
		}
	}()

	type testCase struct {
		name string
		test func(t *testing.T, d depot.Depot)
	}

	for _, impl := range []struct {
		name    string
		setup   func() depot.Depot
		check   func(*testing.T, *depot.Tag, []byte)
		cleanup func()
		tests   []testCase
	}{
		{
			name: "file",
			setup: func() depot.Depot {
				tempDir, err = ioutil.TempDir(".", "file_depot")
				require.NoError(t, err)
				d, err := depot.NewFileDepot(tempDir)
				require.NoError(t, err)
				return d
			},
			check: func(t *testing.T, tag *depot.Tag, data []byte) {
				path := getTagPath(tag)

				if data == nil {
					_, err = os.Stat(filepath.Join(tempDir, path))
					assert.True(t, os.IsNotExist(err))
					return
				}

				fileData, err := ioutil.ReadFile(filepath.Join(tempDir, path))
				require.NoError(t, err)
				assert.Equal(t, data, fileData)
			},
			cleanup: func() {
				require.NoError(t, os.RemoveAll(tempDir))
			},
			tests: []testCase{
				{
					name: "PutFailsWithExisting",
					test: func(t *testing.T, d depot.Depot) {
						name := "bob"

						assert.NoError(t, d.Put(depot.CrtTag(name), []byte("data")))
						assert.Error(t, d.Put(depot.CrtTag(name), []byte("other data")))
						assert.NoError(t, d.Put(depot.PrivKeyTag(name), []byte("data")))
						assert.Error(t, d.Put(depot.PrivKeyTag(name), []byte("other data")))
						assert.NoError(t, d.Put(depot.CsrTag(name), []byte("data")))
						assert.Error(t, d.Put(depot.CsrTag(name), []byte("other data")))
						assert.NoError(t, d.Put(depot.CrlTag(name), []byte("data")))
						assert.Error(t, d.Put(depot.CrlTag(name), []byte("other data")))
					},
				},
				{
					name: "DeleteWhenDNE",
					test: func(t *testing.T, d depot.Depot) {
						name := "bob"

						assert.Error(t, d.Delete(depot.CrtTag(name)))
						assert.Error(t, d.Delete(depot.PrivKeyTag(name)))
						assert.Error(t, d.Delete(depot.CsrTag(name)))
						assert.Error(t, d.Delete(depot.CrlTag(name)))
					},
				},
			},
		},
		{
			name: "mgo",
			setup: func() depot.Depot {
				mgoDepot := &mongoCertDepot{
					session:        session,
					databaseName:   databaseName,
					collectionName: collectionName,
					expireAfter:    30 * 24 * time.Hour,
				}
				return mgoDepot
			},
			check: func(t *testing.T, tag *depot.Tag, data []byte) {
				name, key := getNameAndKey(tag)

				u := &User{}
				assert.NoError(t, session.DB(databaseName).C(collectionName).FindId(name).One(u))
				assert.Equal(t, name, u.ID)
				assert.True(t, u.TTL.After(time.Now().Add(-time.Minute)))

				var value string
				switch key {
				case userCertKey:
					value = u.Cert
				case userPrivateKeyKey:
					value = u.PrivateKey
				case userCertReqKey:
					value = u.CertReq
				case userCertRevocListKey:
					value = u.CertRevocList
				}
				assert.Equal(t, string(data), value)
			},
			cleanup: func() {
				err = session.DB(databaseName).C(collectionName).DropCollection()
				if err != nil {
					require.Equal(t, "ns not found", err.Error())
				}
			},
			tests: []testCase{
				{
					name: "PutUpdates",
					test: func(t *testing.T, d depot.Depot) {
						name := "bob"
						user := &User{
							ID:            name,
							Cert:          "cert",
							PrivateKey:    "key",
							CertReq:       "certReq",
							CertRevocList: "certRevocList",
							TTL:           time.Now(),
						}
						require.NoError(t, session.DB(databaseName).C(collectionName).Insert(user))
						time.Sleep(time.Second)

						certData := []byte("bob's new fake certificate")
						assert.NoError(t, d.Put(depot.CrtTag(name), certData))
						u := &User{}
						assert.NoError(t, session.DB(databaseName).C(collectionName).FindId(name).One(u))
						assert.Equal(t, name, u.ID)
						assert.Equal(t, string(certData), u.Cert)
						assert.Equal(t, user.PrivateKey, u.PrivateKey)
						assert.Equal(t, user.CertReq, u.CertReq)
						assert.Equal(t, user.CertRevocList, u.CertRevocList)
						assert.True(t, u.TTL.After(user.TTL))
						newTTL := u.TTL

						keyData := []byte("bob's new fake private key")
						assert.NoError(t, d.Put(depot.PrivKeyTag(name), keyData))
						u = &User{}
						assert.NoError(t, session.DB(databaseName).C(collectionName).FindId(name).One(u))
						assert.Equal(t, name, u.ID)
						assert.Equal(t, string(certData), u.Cert)
						assert.Equal(t, string(keyData), u.PrivateKey)
						assert.Equal(t, user.CertReq, u.CertReq)
						assert.Equal(t, user.CertRevocList, u.CertRevocList)
						assert.Equal(t, newTTL, u.TTL)

						certReqData := []byte("bob's new fake certificate request")
						assert.NoError(t, d.Put(depot.CsrTag(name), certReqData))
						u = &User{}
						assert.NoError(t, session.DB(databaseName).C(collectionName).FindId(name).One(u))
						assert.Equal(t, name, u.ID)
						assert.Equal(t, string(certData), u.Cert)
						assert.Equal(t, string(keyData), u.PrivateKey)
						assert.Equal(t, string(certReqData), u.CertReq)
						assert.Equal(t, user.CertRevocList, u.CertRevocList)
						assert.Equal(t, newTTL, u.TTL)

						certRevocListData := []byte("bob's new fake certificate revocation list")
						assert.NoError(t, d.Put(depot.CrlTag(name), certRevocListData))
						u = &User{}
						assert.NoError(t, session.DB(databaseName).C(collectionName).FindId(name).One(u))
						assert.Equal(t, name, u.ID)
						assert.Equal(t, string(certData), u.Cert)
						assert.Equal(t, string(keyData), u.PrivateKey)
						assert.Equal(t, string(certReqData), u.CertReq)
						assert.Equal(t, string(certRevocListData), u.CertRevocList)
						assert.Equal(t, newTTL, u.TTL)
					},
				},
				{
					name: "CheckOnExistingUserWithNoData",
					test: func(t *testing.T, d depot.Depot) {
						name := "alice"
						u := &User{
							ID: name,
						}
						require.NoError(t, session.DB(databaseName).C(collectionName).Insert(u))

						assert.False(t, d.Check(depot.CrtTag(name)))
						assert.False(t, d.Check(depot.PrivKeyTag(name)))
						assert.False(t, d.Check(depot.CsrTag(name)))
						assert.False(t, d.Check(depot.CrlTag(name)))
					},
				},
				{
					name: "GetOnExistingUserWithNoData",
					test: func(t *testing.T, d depot.Depot) {
						name := "bob"
						u := &User{
							ID:  name,
							TTL: time.Now(),
						}
						require.NoError(t, session.DB(databaseName).C(collectionName).Insert(u))

						data, err := d.Get(depot.CrtTag(name))
						assert.Error(t, err)
						assert.Nil(t, data)
						data, err = d.Get(depot.PrivKeyTag(name))
						assert.Error(t, err)
						assert.Nil(t, data)
						data, err = d.Get(depot.CsrTag(name))
						assert.Error(t, err)
						assert.Nil(t, data)
						data, err = d.Get(depot.CrlTag(name))
						assert.Error(t, err)
						assert.Nil(t, data)
					},
				},
				{
					name: "GetOnExpiredTTL",
					test: func(t *testing.T, d depot.Depot) {
						name := "bob"
						u := &User{
							ID:            name,
							Cert:          "cert",
							PrivateKey:    "key",
							CertReq:       "certReq",
							CertRevocList: "certRevocList",
							TTL:           time.Time{},
						}
						require.NoError(t, session.DB(databaseName).C(collectionName).Insert(u))

						data, err := d.Get(depot.CrtTag(name))
						assert.Error(t, err)
						assert.Nil(t, data)
						data, err = d.Get(depot.PrivKeyTag(name))
						assert.NoError(t, err)
						assert.Equal(t, u.PrivateKey, string(data))
						data, err = d.Get(depot.CsrTag(name))
						assert.NoError(t, err)
						assert.Equal(t, u.CertReq, string(data))
						data, err = d.Get(depot.CrlTag(name))
						assert.Error(t, err)
						assert.Nil(t, data)
					},
				},

				{
					name: "DeleteWhenDNE",
					test: func(t *testing.T, d depot.Depot) {
						name := "bob"

						assert.NoError(t, d.Delete(depot.CrtTag(name)))
						assert.NoError(t, d.Delete(depot.PrivKeyTag(name)))
						assert.NoError(t, d.Delete(depot.CsrTag(name)))
						assert.NoError(t, d.Delete(depot.CrlTag(name)))
					},
				},
			},
		},
	} {
		t.Run(impl.name, func(t *testing.T) {
			for _, test := range impl.tests {
				t.Run(test.name, func(t *testing.T) {
					d := impl.setup()
					defer impl.cleanup()

					test.test(t, d)
				})
			}
			t.Run("Put", func(t *testing.T) {
				d := impl.setup()
				defer impl.cleanup()
				name := "bob"

				assert.Error(t, d.Put(depot.CrtTag(name), nil))

				certData := []byte("bob's fake certificate")
				assert.NoError(t, d.Put(depot.CrtTag(name), certData))
				impl.check(t, depot.CrtTag(name), certData)
				impl.check(t, depot.PrivKeyTag(name), nil)
				impl.check(t, depot.CsrTag(name), nil)
				impl.check(t, depot.CrlTag(name), nil)

				keyData := []byte("bob's fake private key")
				assert.NoError(t, d.Put(depot.PrivKeyTag(name), keyData))
				impl.check(t, depot.CrtTag(name), certData)
				impl.check(t, depot.PrivKeyTag(name), keyData)
				impl.check(t, depot.CsrTag(name), nil)
				impl.check(t, depot.CrlTag(name), nil)

				certReqData := []byte("bob's fake certificate request")
				assert.NoError(t, d.Put(depot.CsrTag(name), certReqData))
				impl.check(t, depot.CrtTag(name), certData)
				impl.check(t, depot.PrivKeyTag(name), keyData)
				impl.check(t, depot.CsrTag(name), certReqData)
				impl.check(t, depot.CrlTag(name), nil)

				certRevocListData := []byte("bob's fake certificate revocation list")
				assert.NoError(t, d.Put(depot.CrlTag(name), certRevocListData))
				impl.check(t, depot.CrtTag(name), certData)
				impl.check(t, depot.PrivKeyTag(name), keyData)
				impl.check(t, depot.CsrTag(name), certReqData)
				impl.check(t, depot.CrlTag(name), certRevocListData)
			})
			t.Run("Check", func(t *testing.T) {
				d := impl.setup()
				defer impl.cleanup()
				name := "alice"

				assert.False(t, d.Check(depot.CrtTag(name)))
				assert.False(t, d.Check(depot.PrivKeyTag(name)))
				assert.False(t, d.Check(depot.CsrTag(name)))
				assert.False(t, d.Check(depot.CrlTag(name)))

				data := []byte("alice's fake certificate")
				assert.NoError(t, d.Put(depot.CrtTag(name), data))
				assert.True(t, d.Check(depot.CrtTag(name)))
				assert.False(t, d.Check(depot.PrivKeyTag(name)))
				assert.False(t, d.Check(depot.CsrTag(name)))
				assert.False(t, d.Check(depot.CrlTag(name)))

				data = []byte("alice's fake private key")
				assert.NoError(t, d.Put(depot.PrivKeyTag(name), data))
				assert.True(t, d.Check(depot.CrtTag(name)))
				assert.True(t, d.Check(depot.PrivKeyTag(name)))
				assert.False(t, d.Check(depot.CsrTag(name)))
				assert.False(t, d.Check(depot.CrlTag(name)))

				data = []byte("alice's fake certificate request")
				assert.NoError(t, d.Put(depot.CsrTag(name), data))
				assert.True(t, d.Check(depot.CrtTag(name)))
				assert.True(t, d.Check(depot.PrivKeyTag(name)))
				assert.True(t, d.Check(depot.CsrTag(name)))
				assert.False(t, d.Check(depot.CrlTag(name)))

				data = []byte("alice's fake certificate revocation list")
				assert.NoError(t, d.Put(depot.CrlTag(name), data))
				assert.True(t, d.Check(depot.CrtTag(name)))
				assert.True(t, d.Check(depot.PrivKeyTag(name)))
				assert.True(t, d.Check(depot.CsrTag(name)))
				assert.True(t, d.Check(depot.CrlTag(name)))
			})
			t.Run("Get", func(t *testing.T) {
				d := impl.setup()
				defer impl.cleanup()
				name := "bob"

				data, err := d.Get(depot.CrtTag(name))
				assert.Error(t, err)
				assert.Nil(t, data)

				certData := []byte("bob's fake certificate")
				assert.NoError(t, d.Put(depot.CrtTag(name), certData))
				data, err = d.Get(depot.CrtTag(name))
				assert.NoError(t, err)
				assert.Equal(t, certData, data)

				keyData := []byte("bob's fake private key")
				assert.NoError(t, d.Put(depot.PrivKeyTag(name), keyData))
				data, err = d.Get(depot.PrivKeyTag(name))
				assert.NoError(t, err)
				assert.Equal(t, keyData, data)

				certReqData := []byte("bob's fake certificate request")
				assert.NoError(t, d.Put(depot.CsrTag(name), certReqData))
				data, err = d.Get(depot.CsrTag(name))
				assert.NoError(t, err)
				assert.Equal(t, certReqData, data)

				certRevocListData := []byte("bob's fake certificate revocation list")
				assert.NoError(t, d.Put(depot.CrlTag(name), certRevocListData))
				data, err = d.Get(depot.CrlTag(name))
				assert.NoError(t, err)
				assert.Equal(t, certRevocListData, data)
			})
			t.Run("Delete", func(t *testing.T) {
				d := impl.setup()
				defer impl.cleanup()
				deleteName := "alice"
				name := "bob"

				certData := []byte("alice's fake certificate")
				keyData := []byte("alice's fake private key")
				certReqData := []byte("alice's fake certificate request")
				certRevocListData := []byte("alice's fake certificate revocation list")
				assert.NoError(t, d.Put(depot.CrtTag(deleteName), certData))
				assert.NoError(t, d.Put(depot.PrivKeyTag(deleteName), keyData))
				assert.NoError(t, d.Put(depot.CsrTag(deleteName), certReqData))
				assert.NoError(t, d.Put(depot.CrlTag(deleteName), certRevocListData))

				data := []byte("bob's data")
				assert.NoError(t, d.Put(depot.CrtTag(name), data))
				assert.NoError(t, d.Put(depot.PrivKeyTag(name), data))
				assert.NoError(t, d.Put(depot.CsrTag(name), data))
				assert.NoError(t, d.Put(depot.CrlTag(name), data))

				assert.NoError(t, d.Delete(depot.CrtTag(deleteName)))
				impl.check(t, depot.CrtTag(deleteName), nil)
				impl.check(t, depot.PrivKeyTag(deleteName), keyData)
				impl.check(t, depot.CsrTag(deleteName), certReqData)
				impl.check(t, depot.CrlTag(deleteName), certRevocListData)
				impl.check(t, depot.CrtTag(name), data)
				impl.check(t, depot.PrivKeyTag(name), data)
				impl.check(t, depot.CsrTag(name), data)
				impl.check(t, depot.CrlTag(name), data)

				assert.NoError(t, d.Delete(depot.PrivKeyTag(deleteName)))
				impl.check(t, depot.CrtTag(deleteName), nil)
				impl.check(t, depot.PrivKeyTag(deleteName), nil)
				impl.check(t, depot.CsrTag(deleteName), certReqData)
				impl.check(t, depot.CrlTag(deleteName), certRevocListData)
				impl.check(t, depot.CrtTag(name), data)
				impl.check(t, depot.PrivKeyTag(name), data)
				impl.check(t, depot.CsrTag(name), data)
				impl.check(t, depot.CrlTag(name), data)

				assert.NoError(t, d.Delete(depot.CsrTag(deleteName)))
				impl.check(t, depot.CrtTag(deleteName), nil)
				impl.check(t, depot.PrivKeyTag(deleteName), nil)
				impl.check(t, depot.CsrTag(deleteName), nil)
				impl.check(t, depot.CrlTag(deleteName), certRevocListData)
				impl.check(t, depot.CrtTag(name), data)
				impl.check(t, depot.PrivKeyTag(name), data)
				impl.check(t, depot.CsrTag(name), data)
				impl.check(t, depot.CrlTag(name), data)

				assert.NoError(t, d.Delete(depot.CrlTag(deleteName)))
				impl.check(t, depot.CrtTag(deleteName), nil)
				impl.check(t, depot.PrivKeyTag(deleteName), nil)
				impl.check(t, depot.CsrTag(deleteName), nil)
				impl.check(t, depot.CrlTag(deleteName), nil)
				impl.check(t, depot.CrtTag(name), data)
				impl.check(t, depot.PrivKeyTag(name), data)
				impl.check(t, depot.CsrTag(name), data)
				impl.check(t, depot.CrlTag(name), data)
			})
		})
	}
}