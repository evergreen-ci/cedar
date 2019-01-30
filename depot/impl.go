package depot

import (
	"time"

	"github.com/mongodb/anser/bsonutil"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/square/certstrap/depot"
	mgo "gopkg.in/mgo.v2"
)

type DBUser struct {
	ID   string    `bson:"_id"`
	Cert string    `bson:"cert"`
	TTL  time.Time `bson:"ttl"`
}

var (
	dbUserIDKey   = bsonutil.MustHaveTag(DBUser{}, "ID")
	dbUserCertKey = bsonutil.MustHaveTag(DBUser{}, "Cert")
	dbUserTTLKey  = bsonutil.MustHaveTag(DBUser{}, "TTL")
)

type mongoCertDepot struct {
	session        *mgo.Session
	databaseName   string
	collectionName string
	expireAfter    time.Duration
}

type MongoCertDepotOptions struct {
	MongoDBURI           string
	MongoDBDialTimeout   time.Duration
	MongoDBSocketTimeout time.Duration
	DatabaseName         string
	CollectionName       string
	ExpireAfter          time.Duration
}

// Create a new cert depot in the specified MongoDB.
func NewMongoCertDepot(opts MongoCertDepotOptions) (depot.Depot, error) {
	if opts.MongoDBURI == "" {
		return nil, errors.New("must specify a MongoDB URI!")
	}
	if opts.MongoDBDialTimeout <= 0 {
		opts.MongoDBDialTimeout = 2 * time.Second
	}
	if opts.MongoDBSocketTimeout <= 0 {
		opts.MongoDBSocketTimeout = time.Minute
	}
	if opts.DatabaseName == "" {
		opts.DatabaseName = "certDepot"
	}
	if opts.CollectionName == "" {
		opts.CollectionName = "certs"
	}
	if opts.ExpireAfter <= 0 {
		opts.ExpireAfter = 30 * 24 * time.Hour
	}

	session, err := mgo.DialWithTimeout("mongodb://localhost:27017", opts.MongoDBDialTimeout)
	if err != nil {
		return nil, errors.Wrapf(err, "could not connect to db %s", opts.MongoDBURI)
	}
	session.SetSocketTimeout(opts.MongoDBSocketTimeout)

	return &mongoCertDepot{
		session:        session,
		databaseName:   opts.DatabaseName,
		collectionName: opts.CollectionName,
		expireAfter:    opts.ExpireAfter,
	}, nil
}

// Put inserts the data into the document specified by the tag.
func (m *mongoCertDepot) Put(tag *depot.Tag, data []byte) error {
	if data == nil {
		return errors.New("data is nil")
	}

	name := depot.GetNameFromCrtTag(tag)
	u := &DBUser{
		ID:   name,
		Cert: string(data),
		TTL:  time.Now(),
	}
	changeInfo, err := m.session.DB(m.databaseName).C(m.collectionName).UpsertId(name, u)
	grip.DebugWhen(err == nil, message.Fields{
		"db":     m.databaseName,
		"coll":   m.collectionName,
		"id":     name,
		"change": changeInfo,
		"op":     "put",
	})
	return errors.Wrap(err, "problem adding data to the database")
}

// Check returns whether the id/cert pair specified by the tag exists.
func (m *mongoCertDepot) Check(tag *depot.Tag) bool {
	name := depot.GetNameFromCrtTag(tag)

	u := &DBUser{}
	err := m.session.DB(m.databaseName).C(m.collectionName).FindId(name).One(u)
	grip.WarningWhenf(err != nil && err != mgo.ErrNotFound, "problem looking up '%s' in the database", name)

	return err != mgo.ErrNotFound && u.Cert != ""
}

// Get reads the certificate for the id specified by tag. Returns an error if
// the user does not exist or if the TTL has expired.
func (m *mongoCertDepot) Get(tag *depot.Tag) ([]byte, error) {
	name := depot.GetNameFromCrtTag(tag)

	u := &DBUser{}
	err := m.session.DB(m.databaseName).C(m.collectionName).FindId(name).One(u)
	if err == mgo.ErrNotFound {
		return nil, errors.Errorf("could not find %s in the database", name)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "problem looking up %s in the database", name)
	}
	if time.Since(u.TTL) > m.expireAfter {
		return nil, errors.Errorf("certificate for %s has expired!", name)
	}

	return []byte(u.Cert), nil
}

// Delete removes the id/cert pair specified by the tag.
func (m *mongoCertDepot) Delete(tag *depot.Tag) error {
	name := depot.GetNameFromCrtTag(tag)

	err := m.session.DB(m.databaseName).C(m.collectionName).RemoveId(name)
	if err != nil && err != mgo.ErrNotFound {
		return errors.Wrapf(err, "problem deleting %s from the database", name)
	}

	return nil
}
