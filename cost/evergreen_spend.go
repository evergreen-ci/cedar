package cost

import (
	"context"

	"github.com/evergreen-ci/sink/model"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

// GetEvergreenDistrosData returns distros cost data stored in Evergreen by
// calling evergreen GetEvergreenDistrosData function.
func getEvergreenDistrosData(ctx context.Context, c *EvergreenClient, opts *EvergreenReportOptions) ([]model.EvergreenDistroCost, error) {
	distros := []model.EvergreenDistroCost{}
	evgDistros, err := c.GetEvergreenDistroCosts(ctx, opts.StartAt, opts.Duration)
	if err != nil {
		return nil, errors.Wrap(err, "error in getting Evergreen distros data")
	}
	for idx := range evgDistros {
		distros = append(distros, convertEvgDistroToCostDistro(evgDistros[idx]))
	}

	return distros, nil
}

func convertEvgDistroToCostDistro(evgdc EvergreenDistroCost) model.EvergreenDistroCost {
	d := model.EvergreenDistroCost{}
	d.Name = evgdc.DistroID
	d.Provider = evgdc.Provider
	d.InstanceType = evgdc.InstanceType
	d.InstanceSeconds = evgdc.SumTimeTakenMS / 1000
	d.EstimatedCost = evgdc.EstimatedCost
	d.NumTasks = evgdc.NumTasks
	return d
}

// GetEvergreenProjectsData returns distros cost data stored in Evergreen by
// calling evergreen GetEvergreenDistrosData function.
func getEvergreenProjectsData(ctx context.Context, c *EvergreenClient, opts *EvergreenReportOptions) ([]model.EvergreenProjectCost, error) {
	projects := []model.EvergreenProjectCost{}
	evgProjects, err := c.GetEvergreenProjectsData(ctx, opts.StartAt, opts.Duration)
	if err != nil {
		return nil, errors.Wrap(err, "error in getting Evergreen projects data")
	}

	for _, p := range evgProjects {
		projects = append(projects, convertEvgProjectUnitToCostProject(p))
	}

	return projects, nil
}

func convertEvgProjectUnitToCostProject(evgpu EvergreenProjectUnit) model.EvergreenProjectCost {
	p := model.EvergreenProjectCost{}
	p.Name = evgpu.Name

	grip.Info(message.Fields{
		"message":   "building cost data for evergreen",
		"project":   evgpu.Name,
		"num_tasks": len(evgpu.Tasks),
	})
	for _, task := range evgpu.Tasks {
		costTask := model.EvergreenTaskCost{}
		costTask.Githash = task.Githash
		costTask.Distro = task.DistroID
		costTask.Name = task.DisplayName
		costTask.BuildVariant = task.BuildVariant
		costTask.TaskSeconds = task.TimeTakenMS / uint64(1000)
		costTask.EstimatedCost = task.Cost
		p.Tasks = append(p.Tasks, costTask)
	}

	return p
}

func getEvergreenData(ctx context.Context, c *EvergreenClient, opts *EvergreenReportOptions) (*model.EvergreenCost, error) {
	out := &model.EvergreenCost{}

	if opts.DisableDistros {
		grip.Info("skipping evergreen distros data collection")
	} else {
		grip.Info("collecting evergreen distro cost data")
		distros, err := getEvergreenDistrosData(ctx, c, opts)
		if err != nil {
			return nil, errors.Wrap(err, "problem getting distro data from evergreen")
		}
		out.Distros = distros
	}

	if opts.DisableProjects {
		grip.Info("skipping entire evergreen report")
	} else {
		grip.Info("getting evergreen projects data")
		projects, err := getEvergreenProjectsData(ctx, c, opts)
		if err != nil {
			return nil, errors.Wrap(err, "problem getting project data from evergreen")
		}
		out.Projects = projects
	}

	return out, nil
}
