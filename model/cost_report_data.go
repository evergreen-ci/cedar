package model

import (
	"time"

	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/anser/bsonutil"
)

// Report provides time information on the overall structure.
type CostReportMetadata struct {
	Generated  time.Time      `bson:"generated" json:"generated" yaml:"generated"`
	Range      util.TimeRange `bson:"time_range" json:"time_range" yaml:"time_range"`
	Incomplete bool           `bson:"incomplete" json:"incomplete" yaml:"incomplete"`
}

var (
	costReportMetadataGeneratedKey  = bsonutil.MustHaveTag(CostReportMetadata{}, "Generated")
	costReportMetadataRangeKey      = bsonutil.MustHaveTag(CostReportMetadata{}, "Range")
	costReportMetadataIncompleteKey = bsonutil.MustHaveTag(CostReportMetadata{}, "Incomplete")
)

// Evergreen provides a list of the projects and distros in Evergreen.
type EvergreenCost struct {
	Projects []EvergreenProjectCost `bson:"projects" json:"projects" yaml:"projects"`
	Distros  []EvergreenDistroCost  `bson:"distros" json:"distros" yaml:"distros"`

	distro   map[string]*EvergreenDistroCost
	projects map[string]*EvergreenProjectCost
}

func (c *EvergreenCost) refresh() {
	c.distro = make(map[string]*EvergreenDistroCost)
	c.projects = make(map[string]*EvergreenProjectCost)

	for _, p := range c.Projects {
		p.refresh()
		c.projects[p.Name] = &p
	}

	for _, d := range c.Distros {
		c.distro[d.Name] = &d
	}
}

// nolint
var (
	costReportEvergreenCostProjectsKey = bsonutil.MustHaveTag(EvergreenCost{}, "Projects")
	costReportEvergreenCostDistroskey  = bsonutil.MustHaveTag(EvergreenCost{}, "Distros")
)

// EvergreenProjectCost holds the name and tasks for a single project.
type EvergreenProjectCost struct {
	Name  string              `bson:"name" json:"name" yaml:"name"`
	Tasks []EvergreenTaskCost `bson:"tasks" json:"tasks" yaml:"tasks"`

	tasks map[string]*EvergreenTaskCost
}

func (c *EvergreenProjectCost) refresh() {
	c.tasks = make(map[string]*EvergreenTaskCost)
	for _, t := range c.Tasks {
		c.tasks[t.Name] = &t
	}
}

// nolint
var (
	costReportEvergreenProjectCostNameKey  = bsonutil.MustHaveTag(EvergreenProjectCost{}, "Name")
	costReportEvergreenProjectCostTaskskey = bsonutil.MustHaveTag(EvergreenProjectCost{}, "Tasks")
)

// EvergreenDistro holds the information for a single distro in Evergreen.
type EvergreenDistroCost struct {
	Name            string  `bson:"name" json:"name" yaml:"name"`
	Provider        string  `bson:"provider" json:"provider" yaml:"provider"`
	InstanceType    string  `bson:"instance_type,omitempty" json:"instance_type,omitempty" yaml:"instance_type,omitempty"`
	InstanceSeconds int64   `bson:"instance_seconds,omitempty" json:"instance_seconds,omitempty" yaml:"instance_seconds,omitempty"`
	EstimatedCost   float64 `bson:"estimated_cost" json:"estimated_cost" yaml:"estimated_cost"`
	NumTasks        int     `bson:"num_tasks" json:"num_tasks" yaml:"num_tasks"`
}

var (
	costReportEvergreenDistroNameKey            = bsonutil.MustHaveTag(EvergreenDistroCost{}, "Name")
	costReportEvergreenDistroProviderKey        = bsonutil.MustHaveTag(EvergreenDistroCost{}, "Provider")
	costReportEvergreenDistroInstanceTypeKey    = bsonutil.MustHaveTag(EvergreenDistroCost{}, "InstanceType")
	costReportEvergreenDistroInstanceSecondsKey = bsonutil.MustHaveTag(EvergreenDistroCost{}, "InstanceSeconds")
	costReportEvergreenDistroEstimatedCostKey   = bsonutil.MustHaveTag(EvergreenDistroCost{}, "EstimatedCost")
	costReportEvergreenDistroNumTasksKey        = bsonutil.MustHaveTag(EvergreenDistroCost{}, "NumTasks")
)

// Task holds the information for a single task within a project.
type EvergreenTaskCost struct {
	Githash       string  `bson:"githash" json:"githash" yaml:"githash"`
	Name          string  `bson:"name" json:"name" yaml:"name"`
	Distro        string  `bson:"distro" json:"distro" yaml:"distro"`
	BuildVariant  string  `bson:"variant" json:"variant" yaml:"variant"`
	TaskSeconds   uint64  `bson:"seconds" json:"seconds" yaml:"seconds"`
	EstimatedCost float64 `bson:"estimated_cost" json:"estimated_cost" yaml:"estimated_cost"`
}

var (
	costReportEvergreenTaskCostGithashKey      = bsonutil.MustHaveTag(EvergreenTaskCost{}, "Githash")
	costReportEvergreenTaskCostNameKey         = bsonutil.MustHaveTag(EvergreenTaskCost{}, "Name")
	costReportEvergreenTaskCostDistroKey       = bsonutil.MustHaveTag(EvergreenTaskCost{}, "Distro")
	costReportEvergreenTaskCostBuildVariantKey = bsonutil.MustHaveTag(EvergreenTaskCost{}, "BuildVariant")
	costReportEvergreenTaskCostSecondKey       = bsonutil.MustHaveTag(EvergreenTaskCost{}, "TaskSeconds")
	costReportEvergreenTaskCostEstimationKey   = bsonutil.MustHaveTag(EvergreenTaskCost{}, "EstimatedCost")
)

// Provider holds account information for a single provider.
type CloudProvider struct {
	Name     string         `bson:"name" json:"name" yaml:"name"`
	Cost     float64        `bson:"cost" json:"cost" yaml:"cost"`
	Accounts []CloudAccount `bson:"accounts" json:"accounts" yaml:"accounts"`

	accounts map[string]*CloudAccount
}

func (c *CloudProvider) refresh(reportRange util.TimeRange) {
	c.accounts = make(map[string]*CloudAccount)
	c.Cost = 0

	for _, a := range c.Accounts {
		a.refresh(reportRange)
		c.Cost += a.Cost
		c.accounts[a.Name] = &a
	}
}

var (
	costReportCloudProviderNameKey     = bsonutil.MustHaveTag(CloudProvider{}, "Name")
	costReportCloudProviderAccountsKey = bsonutil.MustHaveTag(CloudProvider{}, "Accounts")
	costReportCloudProviderCostKey     = bsonutil.MustHaveTag(CloudProvider{}, "Cost")
)

// Account holds the name and services of a single account for a provider.
type CloudAccount struct {
	Name     string           `bson:"name" json:"name" yaml:"name"`
	Cost     float64          `bson:"cost" json:"cost" yaml:"cost"`
	Services []AccountService `bson:"services" json:"services" yaml:"services"`

	services map[string]*AccountService
}

func (c *CloudAccount) refresh(reportRange util.TimeRange) {
	c.services = make(map[string]*AccountService)
	c.Cost = 0

	for _, s := range c.Services {
		s.refresh(reportRange)
		c.Cost += s.Cost
		c.services[s.Name] = &s
	}
}

var (
	costReportCloudAccountNameKey     = bsonutil.MustHaveTag(CloudAccount{}, "Name")
	costReportCloudAccountServicesKey = bsonutil.MustHaveTag(CloudAccount{}, "Services")
)

// Service holds the item information for a single service within an account.
type AccountService struct {
	Name  string        `bson:"name" json:"name" yaml:"name"`
	Cost  float64       `bson:"cost" json:"cost" yaml:"cost"`
	Items []ServiceItem `bson:"items" json:"items" yaml:"items"`

	items map[string]*ServiceItem
}

func (s *AccountService) refresh(reportRange util.TimeRange) {
	s.items = make(map[string]*ServiceItem)
	s.Cost = 0
	for _, i := range s.Items {
		s.Cost += i.GetCost(reportRange)
		s.items[i.ID()] = &i
	}
}

var (
	costReportAccountServiceNameKey  = bsonutil.MustHaveTag(AccountService{}, "Name")
	costReportAccountServiceItemsKey = bsonutil.MustHaveTag(AccountService{}, "Items")
	costReportAccountServiceCostKey  = bsonutil.MustHaveTag(AccountService{}, "Cost")
)

// Item holds the information for a single item for a service.
type ServiceItem struct {
	Name       string  `bson:"name,omitempty" json:"name,omitempty" yaml:"name,omitempty"`
	ItemType   string  `bson:"type" json:"type" yaml:"type"`
	Launched   int     `bson:"launched" json:"launched" yaml:"launched"`
	Terminated int     `bson:"terminated" json:"terminated" yaml:"terminated"`
	FixedPrice float64 `bson:"fixed_price,omitempty" json:"fixed_price,omitempty" yaml:"fixed_price,omitempty"`
	AvgPrice   float64 `bson:"avg_price,omitempty" json:"avg_price,omitempty" yaml:"avg_price,omitempty"`
	AvgUptime  float64 `bson:"avg_uptime,omitempty" json:"avg_uptime,omitempty" yaml:"avg_uptime,omitempty"`
	TotalHours int     `bson:"total_hours,omitempty" json:"total_hours,omitempty" yaml:"total_hours,omitempty"`
	TotalCost  float64 `bson:"total_cost,omitempty" json:"total_cost,omitempty" yaml:"total_cost,omitempty"`
}

func (i *ServiceItem) ID() string {
	if i.Name != "" {
		return i.Name
	}

	return i.ItemType
}

func (i *ServiceItem) GetCost(reportRange util.TimeRange) float64 {
	if i.TotalCost > 0 {
		return i.TotalCost
	}

	if i.TotalHours > 0 {
		if i.FixedPrice > 0 {
			return float64(i.TotalHours) * i.FixedPrice
		} else if i.AvgPrice > 0 {
			return float64(i.TotalHours) * i.AvgPrice
		}

		return 0
	}

	var hours float64

	if i.AvgUptime > 0 {
		hours = i.AvgUptime
	} else {
		hours = float64(i.Launched-i.Terminated) * reportRange.Duration().Hours()
		if hours < 0 {
			return 0
		}
	}

	if i.FixedPrice > 0 {
		return hours * i.FixedPrice
	}

	if i.AvgPrice > 0 {
		return hours * i.AvgPrice
	}

	return 0
}

var (
	costReportServiceItemNameKey       = bsonutil.MustHaveTag(ServiceItem{}, "Name")
	costReportServiceItemItemTpyeKey   = bsonutil.MustHaveTag(ServiceItem{}, "ItemType")
	costReportServiceItemLaunchedKey   = bsonutil.MustHaveTag(ServiceItem{}, "Launched")
	costReportServiceItemTerminatedKey = bsonutil.MustHaveTag(ServiceItem{}, "Terminated")
	costReportServiceItemFixedPriceKey = bsonutil.MustHaveTag(ServiceItem{}, "FixedPrice")
	costReportServiceItemAvgPriceKey   = bsonutil.MustHaveTag(ServiceItem{}, "AvgPrice")
	costReportServiceItemAvgUptimeKey  = bsonutil.MustHaveTag(ServiceItem{}, "AvgUptime")
	costReportServiceItemTotalHoursKey = bsonutil.MustHaveTag(ServiceItem{}, "TotalHours")
)
