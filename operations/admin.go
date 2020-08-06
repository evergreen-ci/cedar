package operations

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/evergreen-ci/cedar/rest"
	"github.com/evergreen-ci/certdepot"
	"github.com/evergreen-ci/utility"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func Admin() cli.Command {
	return cli.Command{
		Name:  "admin",
		Usage: "manage a deployed cedar application",
		Subcommands: []cli.Command{
			{
				Name:  "conf",
				Usage: "cedar application configuration",
				Subcommands: []cli.Command{
					loadCedarConfig(),
					dumpCedarConfig(),
				},
			},
			{
				Name:  "flags",
				Usage: "manage cedar feature flags over a rest interface",
				Subcommands: []cli.Command{
					setFeatureFlag(),
					unsetFeatureFlag(),
				},
			},
			{
				Name:  "auth",
				Usage: "manage user authentication",
				Subcommands: []cli.Command{
					getUserCert(),
					uploadCerts(),
				},
			},
		},
	}
}

func setFeatureFlag() cli.Command {
	return cli.Command{
		Name:   "set",
		Usage:  "set a named feature flag",
		Flags:  restServiceFlags(addModifyFeatureFlagFlags()...),
		Before: mergeBeforeFuncs(setFlagOrFirstPositional(flagNameflag), requireStringFlag(flagNameflag)),
		Action: func(c *cli.Context) error {
			flag := c.String(flagNameflag)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opts := rest.ClientOptions{
				Host: c.String(clientHostFlag),
				Port: c.Int(clientPortFlag),
			}
			client, err := rest.NewClient(opts)
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			state, err := client.EnableFeatureFlag(ctx, flag)
			if err != nil {
				return errors.Wrapf(err, "problem encountered setting flag '%s', reported state %t", flag, state)
			}
			grip.Infof("successfully set '%s' to '%t", flag, state)
			return nil
		},
	}
}

func unsetFeatureFlag() cli.Command {
	return cli.Command{
		Name:   "unset",
		Usage:  "set a named feature flag",
		Flags:  restServiceFlags(addModifyFeatureFlagFlags()...),
		Before: mergeBeforeFuncs(setFlagOrFirstPositional(flagNameflag), requireStringFlag(flagNameflag)),
		Action: func(c *cli.Context) error {
			flag := c.String(flagNameflag)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opts := rest.ClientOptions{
				Host:   c.String(clientHostFlag),
				Port:   c.Int(clientPortFlag),
				Prefix: "rest",
			}
			client, err := rest.NewClient(opts)
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			state, err := client.DisableFeatureFlag(ctx, flag)
			if err != nil {
				return errors.Wrapf(err, "problem encountered setting flag '%s', reported state %t", flag, state)
			}
			grip.Infof("successfully set '%s' to '%t", flag, state)
			return nil
		},
	}
}

func getUserCert() cli.Command {
	const (
		userNameFlag    = "username"
		passwordFlag    = "password"
		apiKeyFlag      = "api_key"
		writeToFileFlag = "dump"
	)

	return cli.Command{
		Name:  "cert",
		Usage: "get certificates for a user",
		Flags: restServiceFlags(
			cli.StringFlag{
				Name: userNameFlag,
			},
			cli.StringFlag{
				Name: passwordFlag,
			},
			cli.StringFlag{
				Name: apiKeyFlag,
			},
			cli.BoolFlag{
				Name:  writeToFileFlag,
				Usage: "specify to write certificate files to a file",
			},
		),
		Before: mergeBeforeFuncs(requireStringFlag(userNameFlag), requireOneFlag(passwordFlag, apiKeyFlag)),
		Action: func(c *cli.Context) error {
			user := c.String(userNameFlag)
			pass := c.String(passwordFlag)
			apiKey := c.String(apiKeyFlag)
			host := c.String(clientHostFlag)
			port := c.Int(clientPortFlag)
			writeToFile := c.Bool(writeToFileFlag)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			client, err := rest.NewClient(rest.ClientOptions{
				Host:   host,
				Port:   port,
				Prefix: "rest",
			})
			if err != nil {
				return errors.Wrap(err, "problem creating REST client")
			}

			ca, err := client.GetRootCertificate(ctx)
			if err != nil {
				return errors.Wrap(err, "problem fetching authority certificate")
			}
			grip.Notice("fetched certificate authority certificate")

			if writeToFile {
				path := "cedar.ca"
				if err = utility.WriteFile(path, ca); err != nil {
					return errors.WithStack(err)
				}
				grip.Notice(message.Fields{
					"path":    path,
					"content": "certificate authority",
				})
			} else {
				fmt.Println(ca)
			}

			cert, err := client.GetUserCertificate(ctx, user, pass, apiKey)
			if err != nil {
				return errors.Wrap(err, "problem resolving certificate")
			}
			grip.Notice(message.Fields{
				"op":   "retrieved user certificate",
				"user": user,
			})

			if writeToFile {
				path := user + ".crt"
				if err = utility.WriteFile(path, cert); err != nil {
					return errors.WithStack(err)
				}
				grip.Notice(message.Fields{
					"path":    path,
					"content": "user certificate",
				})
			} else {
				fmt.Println(cert)
			}

			key, err := client.GetUserCertificateKey(ctx, user, pass, apiKey)
			if err != nil {
				return errors.Wrap(err, "problem resolving certificate key")
			}

			grip.Notice(message.Fields{
				"op":   "retrieved user certificate key",
				"user": user,
			})

			if writeToFile {
				path := user + ".key"
				if err = utility.WriteFile(path, key); err != nil {
					return errors.WithStack(err)
				}
				grip.Notice(message.Fields{
					"path":    path,
					"content": "user certificate",
				})
			} else {
				fmt.Println(key)
			}

			return nil
		},
	}
}

func uploadCerts() cli.Command {
	return cli.Command{
		Name:  "upload-cert",
		Usage: "upload certificate to a database backed depot",
		Flags: dbFlags(
			cli.StringFlag{
				Name:  "name",
				Usage: "specify name of the certificate and key to upload",
			},
			cli.StringFlag{
				Name:  "depotName",
				Value: "certdepot",
				Usage: "specify name of the certificate depot",
			}),
		Action: func(c *cli.Context) error {
			certName := c.String("name")
			mongodbURI := c.String(dbURIFlag)
			dbName := c.String(dbNameFlag)
			collName := c.String("depotName")

			if !utility.FileExists(certName+".crt") || !utility.FileExists(certName+".key") {
				return errors.New("certificate of that name does not exist")
			}

			opts := &certdepot.MongoDBOptions{
				MongoDBURI:     mongodbURI,
				DatabaseName:   dbName,
				CollectionName: collName,
			}

			crt, err := ioutil.ReadFile(certName + ".crt")
			if err != nil {
				return errors.Wrap(err, "could not read cert file")
			}

			key, err := ioutil.ReadFile(certName + ".key")
			if err != nil {
				return errors.Wrap(err, "could not read cert key file")
			}

			mdepot, err := certdepot.NewMgoCertDepot(opts)
			if err != nil {
				return errors.Wrap(err, "problem creating depot interface")
			}

			err = mdepot.Put(certdepot.CrtTag(certName), crt)
			if err != nil {
				return errors.Wrap(err, "could not save cert file")
			}

			err = mdepot.Put(certdepot.PrivKeyTag(certName), key)
			if err != nil {
				return errors.Wrap(err, "could not save cert key file")
			}

			return nil
		},
	}
}
