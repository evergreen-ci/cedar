package cost

import (
	"time"

	"github.com/evergreen-ci/sink/evergreen"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

func (d *Distro) convertEvgDistroToCostDistro(evgdc *evergreen.DistroCost) {
	d.Name = evgdc.DistroID
	d.Provider = evgdc.Provider
	d.InstanceType = evgdc.InstanceType
	d.InstanceSeconds = int64(evgdc.SumTimeTaken / time.Second)
}

func (p *Project) convertEvgProjectUnitToCostProject(evgpu evergreen.ProjectUnit) {
	p.Name = evgpu.Name
	for _, task := range evgpu.Tasks {
		costTask := &Task{}
		costTask.Githash = task.Githash
		costTask.Name = task.DisplayName
		costTask.BuildVariant = task.BuildVariant
		costTask.TaskSeconds = int64(task.TimeTaken / time.Second)
		p.Tasks = append(p.Tasks, costTask)
	}
}

// GetEvergreenDistrosData returns distros cost data stored in Evergreen by
// calling evergreen GetEvergreenDistrosData function.
func getEvergreenDistrosData(c *evergreen.Client, starttime time.Time,
	duration time.Duration) ([]*Distro, error) {
	distros := []*Distro{}
	evgDistros, err := c.GetEvergreenDistrosData(starttime, duration)
	if err != nil {
		return nil, errors.Wrap(err, "error in getting Evergreen distros data")
	}
	for idx := range evgDistros {
		d := &Distro{}
		d.convertEvgDistroToCostDistro(evgDistros[idx])
		distros = append(distros, d)
	}

	return distros, nil
}

// GetEvergreenProjectsData returns distros cost data stored in Evergreen by
// calling evergreen GetEvergreenDistrosData function.
func getEvergreenProjectsData(c *evergreen.Client, starttime time.Time,
	duration time.Duration) ([]*Project, error) {
	projects := []*Project{}
	evgProjects, err := c.GetEvergreenProjectsData(starttime, duration)
	if err != nil {
		return nil, errors.Wrap(err, "error in getting Evergreen projects data")
	}
	for idx := range evgProjects {
		p := &Project{}
		p.convertEvgProjectUnitToCostProject(evgProjects[idx])
		projects = append(projects, p)
	}

	return projects, nil
}

func getEvergreenData(c *evergreen.Client, starttime time.Time,
	duration time.Duration) (*Evergreen, error) {
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
	return &Evergreen{Distros: distros, Projects: projects}, nil
}
