package evergreen

import (
	"encoding/json"
	"time"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// Project holds information for a single distro within a host
type Project struct {
	Identifier string `json:"identifier"`
}

// TaskCost holds full cost and provider information for a task.
type TaskCost struct {
	Githash      string        `json:"githash"`
	DisplayName  string        `json:"display_name"`
	DistroID     string        `json:"distro"`
	BuildVariant string        `json:"build_variant"`
	TimeTaken    time.Duration `json:"time_taken"`
	Cost         float64       `json:"estimated_cost"`
}

// ProjectUnit holds together all relevant task cost information for a project
type ProjectUnit struct {
	Name  string      `json:"name"`
	Tasks []*TaskCost `json:"tasks"`
}

type projectWorkUnit struct {
	output *Project
	err    error
}

type taskWorkUnit struct {
	taskcost *TaskCost
	err      error
}

// GetProjects is a wrapper function of get for retrieving all projects
// from the Evergreen API.
func (c *Client) getProjects() <-chan projectWorkUnit {
	output := make(chan projectWorkUnit)

	go func() {
		path := "projects"
		for {
			data, link, err := c.get(path)
			if err != nil {
				output <- projectWorkUnit{
					output: nil,
					err:    err,
				}
				break
			}

			projects := []*Project{}
			if err := json.Unmarshal(data, &projects); err != nil {
				output <- projectWorkUnit{
					output: nil,
					err:    err,
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
func (c *Client) getTaskCostsByProject(projectID, starttime,
	duration string) <-chan taskWorkUnit {
	output := make(chan taskWorkUnit)

	go func() {
		path := "cost/project/" + projectID + "/tasks?starttime=" + starttime +
			"&duration=" + duration
		for {
			data, link, err := c.get(path)
			if err != nil {
				output <- taskWorkUnit{
					taskcost: nil,
					err:      err,
				}
				break
			}

			tasks := []*TaskCost{}
			if err := json.Unmarshal(data, &tasks); err != nil {
				output <- taskWorkUnit{
					taskcost: nil,
					err:      err,
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
func (c *Client) getProjectIDs() ([]string, error) {
	projectIDs := []string{}
	catcher := grip.NewCatcher()
	output := c.getProjects()
	for out := range output {
		if out.err != nil {
			catcher.Add(out.err)
			break
		}
		projectIDs = append(projectIDs, out.output.Identifier)
	}
	if catcher.HasErrors() {
		err := errors.Wrapf(catcher.Resolve(), "error getting projects ids; got:", projectIDs)
		grip.Warning(err.Error())
		return nil, err
	}

	return projectIDs, nil
}

// A helper function
func (c *Client) readTaskCostsByProject(projectID string, st, dur string) ([]*TaskCost, error) {
	taskCosts := []*TaskCost{}
	catcher := grip.NewCatcher()
	output := c.getTaskCostsByProject(projectID, st, dur)
	for out := range output {
		if out.err != nil {
			catcher.Add(out.err)
		}
		taskCosts = append(taskCosts, out.taskcost)
	}
	if catcher.HasErrors() {
		return nil, errors.Wrap(catcher.Resolve(), "error when getting task cost data from Evergreen")
	}
	return taskCosts, nil
}

// GetEvergreenProjectsData retrieves project cost information from Evergreen.
func (c *Client) GetEvergreenProjectsData(starttime time.Time, duration time.Duration) ([]ProjectUnit, error) {
	st := starttime.Format(time.RFC3339)
	dur := duration.String()

	projectUnits := []ProjectUnit{}
	pu := ProjectUnit{}

	projectIDs, err := c.getProjectIDs()
	if err != nil {
		return nil, errors.Wrap(err, "error getting projects in GetEvergreenProjectsData")
	}
	for _, projectID := range projectIDs {
		taskCosts, err := c.readTaskCostsByProject(projectID, st, dur)
		if err != nil {
			return nil, errors.Wrap(err, "error in getting task costs in GetEvergreenProjectsData")
		}
		pu.Name = projectID
		pu.Tasks = taskCosts
		// Only include task costs with meaningful information
		if len(taskCosts) > 0 {
			projectUnits = append(projectUnits, pu)
		}
	}

	return projectUnits, nil
}
