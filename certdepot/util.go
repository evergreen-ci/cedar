package certdepot

import (
	"io/ioutil"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/square/certstrap/pkix"
)

func getOrCreatePrivateKey(keyPath, passphrase, name string, keyBits int) (*pkix.Key, error) {
	var key *pkix.Key
	if keyPath != "" {
		keyBytes, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return nil, errors.Wrapf(err, "problem reading key from %s", keyPath)
		}
		key, err = pkix.NewKeyFromPrivateKeyPEM(keyBytes)
		if err != nil {
			return nil, errors.Wrapf(err, "problem getting key from PEM")
		}
		grip.Noticef("read key from %s", keyPath)
	} else {
		if keyBits == 0 {
			keyBits = 2048
		}
		var err error
		key, err = pkix.CreateRSAKey(keyBits)
		if err != nil {
			return nil, errors.Wrap(err, "problem creating RSA key")
		}
		if len(passphrase) > 0 {
			grip.Noticef("created %s.key (encrypted by passphrase)\n", name)
		} else {
			grip.Noticef("created %s.key\n", name)
		}
	}
	return key, nil
}
