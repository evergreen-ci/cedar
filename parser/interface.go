package parser

import (
	"fmt"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/registry"
	"github.com/pkg/errors"
)

type ParserFactoryFactory func(string) Parser
type ParserFactory func(string, interface{}) (Parser, error)

type Parser interface {
	amboy.Job
	SetOptions(interface{}) error
	SetID(string)
}

func RegisterParserUnit(name string, pf ParserFactoryFactory) ParserFactory {
	f := func() amboy.Job {
		return pf(name)
	}

	registry.AddJobType(name, f)

	return func(id string, opts interface{}) (Parser, error) {
		j := f().(Parser)
		j.SetID(fmt.Sprintf("%s-%s", j.Type().Name, id))

		if err := j.SetOptions(opts); err != nil {
			return nil, errors.Wrapf(err, "problem constructing %s (%s)", id, j.ID())
		}
		return j, nil
	}

}
