package certdepot

import (
	"regexp"
	"strings"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/square/certstrap/depot"
	"github.com/square/certstrap/pkix"
)

type CertRequestOptions struct {
	// The depot which stores the id/cert.
	Depot depot.Depot
	// Passprhase to encrypt private-key PEM block.
	Passphrase string
	// Size (in bits) of RSA keypair to generate (defaults to 2048).
	KeyBits int
	// Sets the Organization (O) field of the certificate.
	Organization string
	// Sets the Country (C) field of the certificate.
	Country string
	// Sets the Locality (L) field of the certificate.
	Locality string
	// Sets the Common Name (CN) field of the certificate.
	CommonName string
	// Sets the Organizational Unit (OU) field of the certificate.
	OrganizationalUnit string
	// Sets the State/Province (ST) field of the certificate.
	Province string
	// IP addresses to add as subject alt name.
	IP []string
	// DNS entries to add as subject alt name.
	Domain []string
	// URI values to add as subject alt name.
	URI []string
	// Path to private key PEM file (if blank, will generate new keypair).
	Key string
}

// NewCertRequest creates a new certificate (CSR).
func NewCertRequest(opts CertRequestOptions) error {
	var name = ""
	var err error

	ips, err := pkix.ParseAndValidateIPs(strings.Join(opts.IP, ","))
	if err != nil {
		return errors.Wrapf(err, "problem parsing and validating IPs: %s", opts.IP)
	}

	uris, err := pkix.ParseAndValidateURIs(strings.Join(opts.URI, ","))
	if err != nil {
		return errors.Wrapf(err, "problem parsing and validating URIs: %s", opts.URI)
	}

	switch {
	case opts.CommonName == "":
		name = opts.CommonName
	case len(opts.Domain) != 0:
		name = opts.Domain[0]
	default:
		return errors.New("must provide a common name or domain!")
	}

	formattedName, err := formatName(name)
	if err != nil {
		return errors.Wrap(err, "problem getting formatted name")
	}

	if depot.CheckCertificateSigningRequest(opts.Depot, formattedName) || depot.CheckPrivateKey(opts.Depot, formattedName) {
		return errors.New("certificate request has existed!")
	}

	key, err := getOrCreatePrivateKey(opts.Key, opts.Passphrase, formattedName, opts.KeyBits)
	if err != nil {
		return errors.WithStack(err)
	}

	csr, err := pkix.CreateCertificateSigningRequest(
		key,
		opts.OrganizationalUnit,
		ips,
		opts.Domain,
		uris,
		opts.Organization,
		opts.Country,
		opts.Province,
		opts.Locality,
		name,
	)
	if err != nil {
		return errors.Wrap(err, "problem creating certificate request")
	}
	grip.Noticef("created %s.csr\n", formattedName)

	if err = depot.PutCertificateSigningRequest(opts.Depot, formattedName, csr); err != nil {
		return errors.Wrap(err, "problem saving certificate request")
	}
	if opts.Passphrase != "" {
		if err = depot.PutEncryptedPrivateKey(opts.Depot, formattedName, key, []byte(opts.Passphrase)); err != nil {
			return errors.Wrap(err, "problem saving encrypted private key")
		}
	} else {
		if err = depot.PutPrivateKey(opts.Depot, formattedName, key); err != nil {
			return errors.Wrap(err, "problem saving private key error")
		}
	}

	return nil
}

func formatName(name string) (string, error) {
	var filenameAcceptable, err = regexp.Compile("[^a-zA-Z0-9._-]")
	if err != nil {
		return "", errors.Wrap(err, "problem compiling regex")
	}
	return string(filenameAcceptable.ReplaceAll([]byte(name), []byte("_"))), nil
}
