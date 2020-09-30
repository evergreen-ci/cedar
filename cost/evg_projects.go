package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// EvergreenProject holds information for a single distro within a host
type EvergreenProject struct {
	Identifier string `json:"identifier"`
}

// TaskCost holds full cost and provider information for a task.
type EvergreenTaskCost struct {
	ID           string  `json:"task_id"`
	DisplayName  string  `json:"display_name"`
	DistroID     string  `json:"distro"`
	BuildVariant string  `json:"build_variant"`
	TimeTakenMS  uint64  `json:"time_taken"`
	Githash      string  `json:"githash"`
	Cost         float64 `json:"estimated_cost"`
}

// ProjectUnit holds together all relevant task cost information for a project
type EvergreenProjectUnit struct {
	Name  string              `json:"name"`
	Tasks []EvergreenTaskCost `json:"tasks"`
}

type projectWorkUnit struct {
	output EvergreenProject
	err    error
}

type taskWorkUnit struct {
	taskcost EvergreenTaskCost
	err      error
}

// GetProjects is a wrapper function of get for retrieving all projects
// from the Evergreen API.
func (c *EvergreenClient) getProjects(ctx context.Context) <-chan projectWorkUnit {
	output := make(chan projectWorkUnit)

	go func() {
		path := "projects"
		for {
			data, link, err := c.Get(ctx, path)
			if err != nil {
				output <- projectWorkUnit{
					err: err,
				}
				break
			}

			projects := []EvergreenProject{}
			if err := json.Unmarshal(data, &projects); err != nil {
				output <- projectWorkUnit{
					err: err,
				}
				break
			}

			for _, p := range projects {
				output <- projectWorkUnit{
					output: p,
					err:    nil,
				}
			}

			if link == "" {
				break
			}
			path = link
		}
		close(output)
	}()

	return output
}

// GetTaskCostsForProject is a wrapper function of get for a getting all
// task costs for a project in a given time range from the Evergreen API.
func (c *EvergreenClient) getTaskCostsByProject(ctx context.Context, projectID, starttime, duration string) <-chan taskWorkUnit {
	output := make(chan taskWorkUnit)

	go func() {
		path := "cost/project/" + projectID + "/tasks?starttime=" + starttime +
			"&limit=20&duration=" + duration
		for {
			data, link, err := c.Get(ctx, path)
			if err != nil {
				output <- taskWorkUnit{
					err: err,
				}
				break
			}

			tasks := []EvergreenTaskCost{}
			if err := json.Unmarshal(data, &tasks); err != nil {
				output <- taskWorkUnit{
					err: err,
				}
				break
			}

			for _, t := range tasks {
				output <- taskWorkUnit{
					taskcost: t,
					err:      nil,
				}
			}

			if link == "" {
				break
			}
			path = link + "&starttime=" + starttime +
				"&duration=" + duration
		}
		close(output)
	}()

	return output
}

// A helper function for GetEvergreenProjectsData that gets projectID of
// all distros by calling GetProjects.
func (c *EvergreenClient) getProjectIDs(ctx context.Context) ([]string, error) {
	projectIDs := []string{}
	catcher := grip.NewCatcher()
	output := c.getProjects(ctx)
	for out := range output {
		if out.err != nil {
			catcher.Add(out.err)
			break
		}
		if ctx.Err() != nil {
			catcher.Add(errors.New("operation canceled"))
			break
		}

		projectIDs = append(projectIDs, out.output.Identifier)
	}
	if catcher.HasErrors() {
		err := errors.Wrapf(catcher.Resolve(), "error getting projects ids; got: %s", projectIDs)
		grip.Warning(err.Error())
		if c.allowIncompleteResults {
			return projectIDs, nil
		}
		return nil, err
	}

	return projectIDs, nil
}

// A helper function
func (c *EvergreenClient) readTaskCostsByProject(ctx context.Context, projectID string, st, dur string) ([]EvergreenTaskCost, error) {
	taskCosts := []EvergreenTaskCost{}
	catcher := grip.NewCatcher()
	output := c.getTaskCostsByProject(ctx, projectID, st, dur)
	for out := range output {
		if out.err != nil {
			catcher.Add(out.err)
			continue
		}
		taskCosts = append(taskCosts, out.taskcost)
	}
	if catcher.HasErrors() {
		return nil, errors.Wrap(catcher.Resolve(), "error when getting task cost data from Evergreen")
	}
	return taskCosts, nil
}

// GetEvergreenProjectsData retrieves project cost information from Evergreen.
func (c *EvergreenClient) GetEvergreenProjectsData(ctx context.Context, starttime time.Time, duration time.Duration) ([]EvergreenProjectUnit, error) {
	st := starttime.Format(time.RFC3339)
	dur := fmt.Sprint(duration)

	projectIDs, err := c.getProjectIDs(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting projects in GetEvergreenProjectsData")
	}

	catcher := grip.NewSimpleCatcher()
	wg := &sync.WaitGroup{}
	costs := make(chan EvergreenProjectUnit, len(projectIDs))
	projects := make(chan string, len(projectIDs))
	projectUnits := []EvergreenProjectUnit{}

	for _, idx := range rand.Perm(len(projectIDs)) {
		projects <- projectIDs[idx]
	}
	close(projects)

	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for projectID := range projects {
				if ctx.Err() != nil {
					return
				}

				taskCosts, err := c.readTaskCostsByProject(ctx, projectID, st, dur)
				catcher.Add(errors.Wrap(err, "error in getting task costs in GetEvergreenProjectsData"))
				if taskCosts == nil {
					continue
				}

				costs <- EvergreenProjectUnit{
					Name:  projectID,
					Tasks: taskCosts,
				}
			}
		}()
	}

	wg.Wait()
	close(costs)

	for pu := range costs {
		if len(pu.Tasks) > 0 {
			projectUnits = append(projectUnits, pu)
		}
	}

	if catcher.HasErrors() {
		if c.allowIncompleteResults {
			grip.Warning(catcher.Resolve())
			return projectUnits, nil
		}

		return nil, catcher.Resolve()
	}

	return projectUnits, nil
}
