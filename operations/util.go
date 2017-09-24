package operations

import (
	"encoding/json"
	"os"

	"github.com/pkg/errors"
)

func writeJSON(fn string, data interface{}) error {
	out, err := json.MarshalIndent(data, "", "   ")
	if err != nil {
		return errors.Wrap(err, "problem writing data")
	}

	f, err := os.Create(fn)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	if _, err = f.Write(out); err != nil {
		return err
	}

	if _, err = f.WriteString("\n"); err != nil {
		return errors.WithStack(err)
	}

	if err = f.Sync(); err != nil {
		return err
	}

	return nil
}

func writeString(fn string, data string) error {
	f, err := os.Create(fn)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	if _, err = f.WriteString(data); err != nil {
		return errors.WithStack(err)
	}

	if _, err = f.WriteString("\n"); err != nil {
		return errors.WithStack(err)
	}

	if err = f.Sync(); err != nil {
		return err
	}

	return nil
}
