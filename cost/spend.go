package cost

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/evergreen-ci/sink/evergreen"
	"github.com/evergreen-ci/sink/model"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// CreateReport returns an model.CostReport using a start string, duration, and Config information.
func CreateReport(ctx context.Context, start time.Time, duration time.Duration, config *model.CostConfig) (*model.CostReport, error) {
	grip.Info("Creating the report")
	output := &model.CostReport{}
	reportRange := getTimes(start, duration)

	var err error

	output.Providers, err = getAllProviders(ctx, reportRange, config)
	if err != nil {
		return nil, errors.Wrap(err, "Problem retrieving providers information")
	}

	c := evergreen.NewClient(&http.Client{}, &config.Evergreen)
	evg, err := getEvergreenData(c, reportRange.start, duration)
	if err != nil {
		return nil, errors.Wrap(err, "Problem retrieving evergreen information")
	}
	output.Evergreen = *evg

	output.Report = model.CostReportMetadata{
		Begin:     reportRange.start,
		End:       reportRange.end,
		Generated: time.Now(),
	}
	return output, nil
}

func WriteToFile(conf *model.CostConfig, report *model.CostReport, fn string) error {
	// no directory, print to stdout
	if conf.Opts.Directory == "" {
		return errors.New("output directory not specified, cannot write report")
	}

	fn = filepath.Join(conf.Opts.Directory, fn)
	grip.Infof("writing cost report to %s", fn)
	file, err := os.Create(fn)
	if err != nil {
		return errors.Wrapf(err, "Problem creating file %s", fn)
	}
	defer file.Close()

	rendered := report.String()
	if rendered == "" {
		return errors.New("problem rendering report")
	}

	if _, err = file.WriteString(rendered); err != nil {
		return errors.WithStack(err)
	}

	return errors.Wrap(err, "Problem writing to file")
}
