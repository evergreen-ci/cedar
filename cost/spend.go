// kim: TODO: remove
package cost

// import (
//     "context"
//     "fmt"
//     "net/http"
//     "os"
//     "path/filepath"
//     "time"
//
//     "github.com/evergreen-ci/cedar/model"
//     "github.com/mongodb/grip"
//     "github.com/pkg/errors"
// )
//
// type EvergreenReportOptions struct {
//     DisableAll             bool
//     DisableProjects        bool
//     DisableDistros         bool
//     AllowIncompleteResults bool
//     DryRun                 bool
//     Duration               time.Duration
//     StartAt                time.Time
// }
//
// // CreateReport returns an model.CostReport using a start string, duration, and Config information.
// func CreateReport(ctx context.Context, config *model.CostConfig, opts *EvergreenReportOptions) (*model.CostReport, error) {
//     grip.Info("Creating the report")
//     output := &model.CostReport{}
//     reportRange := model.GetTimeRange(opts.StartAt, opts.Duration)
//
//     if opts.DisableAll {
//         grip.Info("skipping entire evergreen report.")
//     } else {
//         c := NewEvergreenClient(&http.Client{}, &config.Evergreen)
//         c.SetAllowIncompleteResults(opts.AllowIncompleteResults)
//         evg, err := getEvergreenData(ctx, c, opts)
//         if err != nil {
//             return nil, errors.Wrap(err, "Problem retrieving evergreen information")
//         }
//         grip.Info("collected data from evergreen")
//         output.Evergreen = *evg
//
//     }
//
//     var err error
//     output.Providers, err = getAllProviders(ctx, reportRange, config)
//     if err != nil {
//         return nil, errors.Wrap(err, "Problem retrieving providers information")
//     }
//     grip.Info("collected data from aws")
//
//     output.Report = model.CostReportMetadata{
//         Range:      reportRange,
//         Generated:  time.Now(),
//         Incomplete: opts.AllowIncompleteResults,
//     }
//
//     return output, nil
// }
//
// func WriteToFile(conf *model.CostConfig, report *model.CostReport, fn string) error {
//     var err error
//     // no directory, print to stdout
//     outputDir := conf.Opts.Directory
//     if outputDir == "" {
//         outputDir, err = os.Getwd()
//         if err != nil {
//             return errors.WithStack(err)
//         }
//     }
//
//     if _, err = os.Stat(outputDir); os.IsNotExist(err) {
//         if err = os.MkdirAll(outputDir, 0755); err != nil {
//             return errors.WithStack(err)
//         }
//     }
//
//     fn = filepath.Join(outputDir, fn)
//     grip.Infof("writing cost report to %s", fn)
//     file, err := os.Create(fn)
//     if err != nil {
//         return errors.Wrapf(err, "Problem creating file %s", fn)
//     }
//     defer file.Close()
//
//     rendered := fmt.Sprint(report)
//     if rendered == "" {
//         return errors.New("problem rendering report")
//     }
//
//     if _, err = file.WriteString(rendered); err != nil {
//         return errors.WithStack(err)
//     }
//
//     return errors.Wrap(err, "Problem writing to file")
// }
