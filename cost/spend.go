package cost

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
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

// Print writes the report to the given file, using the directory in the config file.
// If no directory is given, print report to stdout.
func Print(conf *model.CostConfig, report *model.CostReport, filepath string) error {
	jsonReport, err := json.MarshalIndent(report, "", "    ") // pretty print

	if err != nil {
		return errors.Wrap(err, "Problem marshalling report into JSON")
	}
	// no directory, print to stdout
	if conf.Opts.Directory == "" {
		fmt.Printf("%s\n", string(jsonReport))
		return nil
	}

	filepath = strings.Join([]string{conf.Opts.Directory, filepath}, "/")
	grip.Infof("Printing the report to %s\n", filepath)
	file, err := os.Create(filepath)
	if err != nil {
		return errors.Wrap(err, "Problem creating file")
	}
	defer file.Close()
	_, err = file.Write(jsonReport)
	if err != nil {
		return err
	}
	return errors.Wrap(err, "Problem writing to file")
}
