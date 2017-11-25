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

	return errors.WithStack(writeBytes(f, out))
}

func writeString(fn string, data string) error {
	f, err := os.Create(fn)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	return errors.WithStack(writeBytes(f, []byte(data)))
}

func writeBytes(f *os.File, data []byte) error {
	if _, err := f.Write(data); err != nil {
		return errors.WithStack(err)
	}

	if _, err := f.WriteString("\n"); err != nil {
		return errors.WithStack(err)
	}

	if err := f.Sync(); err != nil {
		return err
	}

	return nil
}
