package certdepot

import (
	"strings"
	"time"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/square/certstrap/depot"
	"github.com/square/certstrap/pkix"
)

type SignOptions struct {
	// The depot which stores the id/cert.
	Depot depot.Depot
	// Host name of the certificate to be signed.
	Host string
	// Passprhase to encrypt private-key PEM block.
	Passphrase string
	// How long until the certificate expires.
	Expires time.Duration
	// Name of CA to issue cert with.
	CA string
	// Passphrase to decrypt CA's private-key PEM block.
	CAPassphrase string
	// Whether generated certificate should be an intermediate.
	Intermediate bool
}

func Sign(opts SignOptions) error {
	if opts.Host == "" {
		return errors.New("must provide name of host!")
	}
	if opts.CA == "" {
		return errors.New("must provide name of CA")
	}
	formattedReqName := strings.Replace(opts.Host, " ", "_", -1)
	formattedCAName := strings.Replace(opts.CA, " ", "_", -1)

	if depot.CheckCertificate(opts.Depot, formattedReqName) {
		return errors.New("certificate has existed!")
	}

	csr, err := depot.GetCertificateSigningRequest(opts.Depot, formattedReqName)
	if err != nil {
		return errors.Wrap(err, "problem getting host's certificate signing request")
	}
	crt, err := depot.GetCertificate(opts.Depot, formattedCAName)
	if err != nil {
		return errors.Wrap(err, "problem getting CA certificate")
	}

	// validate that crt is allowed to sign certificates
	raw_crt, err := crt.GetRawCertificate()
	if err != nil {
		return errors.Wrap(err, "problem getting raw CA certificate")
	}
	// we punt on checking BasicConstraintsValid and checking MaxPathLen. The goal
	// is to prevent accidentally creating invalid certificates, not protecting
	// against malicious input.
	if !raw_crt.IsCA {
		return errors.Wrapf(err, "%s is not allowed to sign certificates", opts.CA)
	}

	var key *pkix.Key
	if opts.CAPassphrase == "" {
		key, err = depot.GetPrivateKey(opts.Depot, formattedCAName)
		if err != nil {
			return errors.Wrap(err, "problem getting unencrypted (assumed) CA key")
		}
	} else {
		key, err = depot.GetEncryptedPrivateKey(opts.Depot, formattedCAName, []byte(opts.CAPassphrase))
		if err != nil {
			return errors.Wrap(err, "problem getting encrypted CA key")
		}
	}

	expiresTime := time.Now().Add(opts.Expires)
	var crtOut *pkix.Certificate
	if opts.Intermediate {
		grip.Notice("Building intermediate")
		crtOut, err = pkix.CreateIntermediateCertificateAuthority(crt, key, csr, expiresTime)
	} else {
		crtOut, err = pkix.CreateCertificateHost(crt, key, csr, expiresTime)
	}
	if err != nil {
		return errors.Wrap(err, "problem creating certificate")
	}

	if err = depot.PutCertificate(opts.Depot, formattedReqName, crtOut); err != nil {
		return errors.Wrap(err, "problem saving certificate")
	}
	grip.Noticef(
		"created %s.crt from %s.csr signed by %s.key\n",
		formattedReqName,
		formattedReqName,
		formattedCAName,
	)

	return nil
}
