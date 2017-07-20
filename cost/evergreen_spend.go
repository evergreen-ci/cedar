package cost

import (
	"time"

	"github.com/evergreen-ci/sink/evergreen"
	"github.com/pkg/errors"
)

func (d *Distro) convertEvgDistroToCostDistro(evgdc *evergreen.DistroCost) {
	d.Name = evgdc.DistroID
	d.Provider = evgdc.Provider
	d.InstanceType = evgdc.InstanceType
	d.InstanceSeconds = int(evgdc.SumTimeTaken / time.Second)
}

// GetEvergreenDistrosData returns distros cost data stored in Evergreen by
// calling evergreen GetEvergreenDistrosData function.
func GetEvergreenDistrosData(c *evergreen.Client, starttime time.Time,
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
