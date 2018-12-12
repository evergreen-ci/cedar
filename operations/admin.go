package operations

import (
	"context"

	"github.com/evergreen-ci/cedar/rest"
	"github.com/mongodb/grip"
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

			client, err := rest.NewClient(c.String(clientHostFlag), c.Int(clientPortFlag), "")
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

			client, err := rest.NewClient(c.String(clientHostFlag), c.Int(clientPortFlag), "")
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
