package db

import "github.com/pkg/errors"

// Q holds all information necessary to execute a query
type Q struct {
	filter     interface{} // should be bson.D or bson.M
	projection interface{} // should be bson.D or bson.M
	sort       []string
	skip       int
	limit      int
}

// Query creates a db.Q for the given MongoDB query. The filter
// can be a struct, bson.D, bson.M, nil, etc.
func Query(filter interface{}) *Q {
	return &Q{filter: filter}
}

func (q *Q) Filter(filter interface{}) *Q {
	q.filter = filter
	return q
}

func (q *Q) Project(projection interface{}) *Q {
	q.projection = projection
	return q
}

func (q *Q) WithFields(fields ...string) *Q {
	projection := map[string]int{}
	for _, f := range fields {
		projection[f] = 1
	}
	q.projection = projection
	return q
}

func (q *Q) WithoutFields(fields ...string) *Q {
	projection := map[string]int{}
	for _, f := range fields {
		projection[f] = 0
	}
	q.projection = projection
	return q
}

func (q *Q) Sort(sort ...string) *Q {
	q.sort = sort
	return q
}

func (q *Q) Skip(skip int) *Q {
	q.skip = skip
	return q
}

func (q *Q) Limit(limit int) *Q {
	q.limit = limit
	return q
}

// FindOne runs a Q query against the given collection, applying the results to "out."
// Only reads one document from the DB.
func (q *Q) FindOne(collection string, out interface{}) error {
	return errors.WithStack(findOne(
		collection,
		q.filter,
		q.projection,
		q.sort,
		out,
	))
}

// FindAll runs a Q query against the given collection, applying the results to "out."
func (q *Q) FindAll(collection string, out interface{}) error {
	return errors.WithStack(findAll(
		collection,
		q.filter,
		q.projection,
		q.sort,
		q.skip,
		q.limit,
		out,
	))
}

func (q *Q) Update(coll string, update interface{}) error {
	return errors.WithStack(runUpdate(coll, q.filter, update))
}

// Count runs a Q count query against the given collection.
func (q *Q) Count(collection string) (int, error) {
	count, err := count(collection, q.filter)
	err = errors.WithStack(err)

	return count, err
}

func (q *Q) RemoveOne(collection string) error {
	return errors.WithStack(removeOne(collection, q.filter))
}

func (q *Q) Iter(collection string) ResultsIterator {
	return iter(collection, q.filter, q.projection, q.sort, q.skip, q.limit)
}
