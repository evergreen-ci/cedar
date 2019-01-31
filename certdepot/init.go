package certdepot

import (
	"strings"
	"time"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/square/certstrap/depot"
	"github.com/square/certstrap/pkix"
)

type InitOptions struct {
	// The depot which stores the id/cert.
	Depot depot.Depot
	// Passprhase to encrypt private-key PEM block.
	Passphrase string
	// Size (in bits) of RSA keypair to generate (defaults to 2048).
	KeyBits int
	// How long until the certificate expires.
	Expires time.Duration
	// Sets the Organization (O) field of the certificate.
	Organization string
	// Sets the Organizational Unit (OU) field of the certificate.
	Country string
	// Sets the Locality (L) field of the certificate.
	Locality string
	// Sets the Common Name (CN) field of the certificate.
	CommonName string
	// Sets the Organizational Unit (OU) field of the certificate.
	OrganizationalUnit string
	// Sets the State/Province (ST) field of the certificate.
	Province string
	// Path to private key PEM file (if blank, will generate new keypair).
	Key string
}

// Init initializes a new CA.
func Init(opts InitOptions) error {
	if opts.CommonName == "" {
		return errors.New("must provide Command Name for CA!")
	}
	formattedName := strings.Replace(opts.CommonName, " ", "_", -1)

	if depot.CheckCertificate(opts.Depot, formattedName) || depot.CheckPrivateKey(opts.Depot, formattedName) {
		return errors.New("CA with specified name already exists!")
	}

	var err error
	key, err := getOrCreatePrivateKey(opts.Key, opts.Passphrase, formattedName, opts.KeyBits)
	if err != nil {
		return errors.WithStack(err)
	}

	expiresTime := time.Now().Add(opts.Expires)
	crt, err := pkix.CreateCertificateAuthority(
		key,
		opts.OrganizationalUnit,
		expiresTime,
		opts.Organization,
		opts.Country,
		opts.Province,
		opts.Locality,
		opts.CommonName,
	)
	if err != nil {
		return errors.Wrap(err, "problem creating certificate authority")
	}
	grip.Noticef("created %s.crt\n", formattedName)

	if err = depot.PutCertificate(opts.Depot, formattedName, crt); err != nil {
		return errors.Wrap(err, "problem saving certificate authority")
	}

	if len(opts.Passphrase) > 0 {
		if err = depot.PutEncryptedPrivateKey(opts.Depot, formattedName, key, []byte(opts.Passphrase)); err != nil {
			return errors.Wrap(err, "problem saving encrypted private key")
		}
	} else {
		if err = depot.PutPrivateKey(opts.Depot, formattedName, key); err != nil {
			return errors.Wrap(err, "problem saving private key")
		}
	}

	// create an empty CRL, this is useful for Java apps which mandate a CRL
	crl, err := pkix.CreateCertificateRevocationList(key, crt, expiresTime)
	if err != nil {
		return errors.Wrap(err, "problem creating certificate revocation list")
	}
	if err = depot.PutCertificateRevocationList(opts.Depot, formattedName, crl); err != nil {
		return errors.Wrap(err, "problem saving certificate revocation list")
	}
	grip.Noticef("created %s.crl\n", formattedName)

	return nil
}
