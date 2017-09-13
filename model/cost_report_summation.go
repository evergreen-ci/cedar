package model

type CostReportSummary struct {
	Metadata          CostReportMetadata        `bson:"metadata" json:"metadata" yaml:"metadata"`
	EvergreenProjects []EvergreenProjectSummary `bson:"projects" json:"projects" yaml:"projects"`
	ProviderSummary   []CloudProviderSummary    `bson:"providers" json:"providers" yaml:"providers"`
	TotalCost         float64                   `bson:"total_cost" json:"total_cost" yaml:"total_cost"`
}

type EvergreenProjectSummary struct {
	Name        string           `bson:"name" json:"name" yaml:"name"`
	Versions    int              `bson:"versions" json:"versions" yaml:"versions"`
	ResourceUse map[string]int64 `bson:"resource_use" json:"resource_use" yaml:"resource_use"`
}

type CloudProviderSummary struct {
	Name     string
	Services map[string]float64
}

func NewCostReportSummary(r *CostReport) *CostReportSummary {
	r.refresh()

	out := CostReportSummary{
		Metadata: r.Report,
	}

	for _, p := range r.Evergreen.Projects {
		psum := EvergreenProjectSummary{Name: p.Name, ResourceUse: map[string]int64{}}
		commitSet := map[string]struct{}{}
		for _, t := range p.Tasks {
			commitSet[t.Githash] = struct{}{}
			distro := r.Evergreen.distro[t.Distro]
			if distro.InstanceType == "" {
				psum.ResourceUse[distro.Name] += t.TaskSeconds
			} else {
				psum.ResourceUse[distro.InstanceType] += t.TaskSeconds
			}
		}

		psum.Versions = len(commitSet)
		out.EvergreenProjects = append(out.EvergreenProjects, psum)
	}

	for _, p := range r.Providers {
		psum := CloudProviderSummary{Name: p.Name, Services: map[string]float64{}}
		for _, account := range p.Accounts {
			for _, service := range account.Services {
				psum.Services[service.Name] += float64(service.Cost)
				out.TotalCost += float64(service.Cost)
			}
		}

		out.ProviderSummary = append(out.ProviderSummary, psum)
	}

	return &out
}
