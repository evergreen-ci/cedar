package cost

//Output provides the structure for the report that will be returned for
//the build cost reporting tool.
type Output struct {
	Report    Report      `json:"report,omitempty"`
	Evergreen Evergreen   `json:"evergreen,omitempty"`
	Providers []*Provider `json:"providers,omitempty"`
}

//Report provides time information on the overall structure.
type Report struct {
	Generated string `json:"generated,omitempty"`
	Begin     string `json:"begin,omitempty"`
	End       string `json:"end,omitempty"`
}

//Evergreen provides a list of the projects and distros in Evergreen.
type Evergreen struct {
	Projects []*Project `json:"projects,omitempty"`
	Distros  []*Distro  `json:"distros,omitempty"`
}

//Provider holds account information for a single provider.
type Provider struct {
	Name     string     `json:"name,omitempty" yaml:"name,omitempty"`
	Accounts []*Account `json:"accounts,omitempty" yaml:",omitempty"`
	Cost     float32    `json:"cost,omitempty" yaml:"cost,omitempty"`
}

//Project holds the name and tasks for a single project.
type Project struct {
	Name  string  `json:"name,omitempty"`
	Tasks []*Task `json:"tasks,omitempty"`
}

//Distro holds the information for a single distro in Evergreen.
type Distro struct {
	Name          string `json:"name,omitempty"`
	Provider      string `json:"provider,omitempty"`
	InstanceType  string `json:"instance_type,omitempty"`
	InstanceHours int    `json:"instance_hours,omitempty"`
}

//Account holds the name and services of a single account for a provider.
type Account struct {
	Name     string     `json:"name,omitempty"`
	Services []*Service `json:"services,omitempty"`
}

//Task holds the information for a single task within a project.
type Task struct {
	Githash      string `json:"githash,omitempty"`
	Name         string `json:"name,omitempty"`
	Distro       string `json:"distro,omitempty"`
	BuildVariant string `json:"build_variant,omitempty"`
	TaskMinutes  int    `json:"task_minutes,omitempty"`
}

//Service holds the item information for a single service within an account.
type Service struct {
	Name  string  `json:"name,omitempty"`
	Items []*Item `json:"items,omitempty"`
	Cost  float32 `json:"cost,omitempty"`
}

//Item holds the information for a single item for a service.
type Item struct {
	Name       string  `json:"name,omitempty"`
	ItemType   string  `json:"type,omitempty"`
	Launched   int     `json:"launched,omitempty"`
	Terminated int     `json:"terminated,omitempty"`
	FixedPrice float32 `json:"fixed_price,omitempty"`
	AvgPrice   float32 `json:"avg_price,omitempty"`
	AvgUptime  float32 `json:"avg_uptime,omitempty"`
	TotalHours int     `json:"total_hours,omitempty"`
}
