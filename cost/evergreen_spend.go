package cost

import (
	"time"

	"github.com/evergreen-ci/sink/evergreen"
	"github.com/evergreen-ci/sink/model"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// GetEvergreenDistrosData returns distros cost data stored in Evergreen by
// calling evergreen GetEvergreenDistrosData function.
func getEvergreenDistrosData(c *evergreen.Client, starttime time.Time, duration time.Duration) ([]model.EvergreenDistroCost, error) {
	distros := []model.EvergreenDistroCost{}
	evgDistros, err := c.GetEvergreenDistrosData(starttime, duration)
	if err != nil {
		return nil, errors.Wrap(err, "error in getting Evergreen distros data")
	}
	for idx := range evgDistros {
		distros = append(distros, convertEvgDistroToCostDistro(evgDistros[idx]))
	}

	return distros, nil
}

func convertEvgDistroToCostDistro(evgdc *evergreen.DistroCost) model.EvergreenDistroCost {
	d := model.EvergreenDistroCost{}
	d.Name = evgdc.DistroID
	d.Provider = evgdc.Provider
	d.InstanceType = evgdc.InstanceType
	d.InstanceSeconds = int64(evgdc.SumTimeTaken / time.Second)
	return d
}

// GetEvergreenProjectsData returns distros cost data stored in Evergreen by
// calling evergreen GetEvergreenDistrosData function.
func getEvergreenProjectsData(c *evergreen.Client, starttime time.Time, duration time.Duration) ([]model.EvergreenProjectCost, error) {
	projects := []model.EvergreenProjectCost{}
	evgProjects, err := c.GetEvergreenProjectsData(starttime, duration)
	if err != nil {
		return nil, errors.Wrap(err, "error in getting Evergreen projects data")
	}
	for idx := range evgProjects {
		projects = append(projects, convertEvgProjectUnitToCostProject(evgProjects[idx]))
	}

	return projects, nil
}

func convertEvgProjectUnitToCostProject(evgpu evergreen.ProjectUnit) model.EvergreenProjectCost {
	p := model.EvergreenProjectCost{}
	p.Name = evgpu.Name
	for _, task := range evgpu.Tasks {
		costTask := model.EvergreenTaskCost{}
		costTask.Githash = task.Githash
		costTask.Name = task.DisplayName
		costTask.BuildVariant = task.BuildVariant
		costTask.TaskSeconds = int64(task.TimeTaken / time.Second)
		p.Tasks = append(p.Tasks, costTask)
	}

	return p
}

func getEvergreenData(c *evergreen.Client, starttime time.Time, duration time.Duration) (*model.EvergreenCost, error) {
	grip.Info("Getting Evergreen Distros")

	distros, err := getEvergreenDistrosData(c, starttime, duration)
	if err != nil {
		return nil, errors.Wrap(err, "error in GetEvergreenData")
	}
	grip.Info("Getting Evergreen Projects")
	projects, err := getEvergreenProjectsData(c, starttime, duration)
	if err != nil {
		return nil, errors.Wrap(err, "error in GetEvergreenData")
	}
	return &model.EvergreenCost{Distros: distros, Projects: projects}, nil
}
