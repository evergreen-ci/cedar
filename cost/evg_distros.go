package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// EvergreenDistro holds information for a single distro within a host.
type EvergreenDistro struct {
	DistroID string `json:"name"`
}

// DistroCost holds full cost and provider information for a distro.
type EvergreenDistroCost struct {
	DistroID       string  `json:"distro_id"`
	SumTimeTakenMS int64   `json:"sum_time_taken"`
	Provider       string  `json:"provider"`
	InstanceType   string  `json:"instance_type,omitempty"`
	EstimatedCost  float64 `json:"estimated_cost"`
	NumTasks       int     `json:"num_tasks"`
}

// GetDistros returns a slice of the names of all distros from the
// Evergreen API.
func (c *EvergreenClient) GetDistros(ctx context.Context) ([]string, error) {
	data, link, err := c.Get(ctx, "/distros")
	if err != nil {
		if c.allowIncompleteResults {
			return []string{}, nil
		}
		return nil, errors.Wrap(err, "error in getting distros")
	}
	grip.WarningWhen(link != "", "/distros should not be a paginated route")
	distros := []EvergreenDistro{}
	if err := json.Unmarshal(data, &distros); err != nil {
		if c.allowIncompleteResults {
			return []string{}, nil
		}
		return nil, errors.WithStack(err)
	}

	distroIDs := make([]string, len(distros))
	for idx := range distros {
		distroIDs[idx] = distros[idx].DistroID
	}
	return distroIDs, nil
}

// GetEvergreenDistrosData retrieves distros cost data from Evergreen.
func (c *EvergreenClient) GetEvergreenDistroCosts(ctx context.Context, startAt time.Time, duration time.Duration) ([]EvergreenDistroCost, error) {
	distroIDs, err := c.GetDistros(ctx)
	if err != nil {
		if c.allowIncompleteResults {
			return []EvergreenDistroCost{}, nil
		}

		return nil, errors.Wrap(err, "error in getting distroID in GetEvergreenDistrosData")
	}

	distroCosts := []EvergreenDistroCost{}
	costs := make(chan EvergreenDistroCost, len(distroIDs))
	distros := make(chan string, len(distroIDs))
	catcher := grip.NewCatcher()
	wg := &sync.WaitGroup{}

	for _, idx := range rand.Perm(len(distroIDs)) {
		distros <- distroIDs[idx]
	}
	close(distros)

	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for distro := range distros {
				if ctx.Err() != nil {
					return
				}

				dc, err := c.GetDistroCost(ctx, distro, startAt, duration)
				catcher.Add(errors.Wrap(err, "error when getting distro cost data from Evergreen"))
				if dc == nil {
					continue
				}

				costs <- *dc
			}
		}()
	}

	wg.Wait()
	close(costs)

	for evgdc := range costs {
		if evgdc.SumTimeTakenMS > 0 || evgdc.EstimatedCost > 0 {
			distroCosts = append(distroCosts, evgdc)
		}
	}

	if catcher.HasErrors() {
		grip.Warning(catcher.Resolve())
		if c.allowIncompleteResults {
			return distroCosts, nil
		}
		return nil, catcher.Resolve()
	}

	return distroCosts, nil
}

// GetDistroCost is a wrapper function of get for getting all distro costs
// from the evergreen API given a distroID.
func (c *EvergreenClient) GetDistroCost(ctx context.Context, distroID string, startAt time.Time, duration time.Duration) (*EvergreenDistroCost, error) {
	st := startAt.Format("2006-01-02T15:04:05Z07:00")
	dur := strings.TrimRight(fmt.Sprintf("%s ", duration), "0s ")

	data, link, err := c.Get(ctx, "/cost/distro/"+distroID+"?starttime="+st+"&duration="+dur)
	if link != "" {
		return nil, errors.New("/cost/distro should not be a paginated route")
	}
	if err != nil {
		return nil, errors.Wrap(err, "error in GetDistroCost")
	}
	distro := EvergreenDistroCost{}
	if err := json.Unmarshal(data, &distro); err != nil {
		return nil, err
	}
	return &distro, nil
}
