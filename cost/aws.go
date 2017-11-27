package cost

import (
	"github.com/evergreen-ci/sink/amazon"
	"github.com/evergreen-ci/sink/model"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// setItems sets the number of launched and terminated instances of the given cost item.
// The sums are calculated from the information in the Item array.
func setSums(res *model.ServiceItem, items []amazon.Item) {
	res.Launched, res.Terminated, res.TotalHours = 0, 0, 0
	for _, item := range items {
		if item.Launched {
			if item.Count != 0 {
				res.Launched += item.Count
			} else {
				res.Launched++
			}
		}
		if item.Terminated {
			if item.Count != 0 {
				res.Terminated += item.Count
			} else {
				res.Terminated++
			}
		}
		res.TotalHours += item.Uptime
	}
}

// setAverages sets the average price, fixed price, and uptime of the given cost item.
// The averages are calculated from the information in the Item array.
func setAverages(res *model.ServiceItem, items []amazon.Item) {
	var prices, uptimes, fixedPrices []float64
	for _, item := range items {
		if item.Price != 0.0 {
			prices = append(prices, item.Price)
		}
		if item.FixedPrice != 0.0 {
			fixedPrices = append(fixedPrices, item.FixedPrice)
		}
		if item.Uptime != 0 {
			uptimes = append(uptimes, float64(item.Uptime))
		}
	}
	if len(prices) != 0 {
		res.AvgPrice = avg(prices)
	}
	if len(fixedPrices) != 0 {
		res.FixedPrice = avg(fixedPrices)
	}
	if len(uptimes) != 0 {
		res.AvgUptime = avg(uptimes)
	}
}

// createItemFromEC2Instance creates a new cost.Item using a key/item array pair.
func createCostItemFromAmazonItems(key amazon.ItemKey, items []amazon.Item) model.ServiceItem {
	item := model.ServiceItem{
		Name:     key.Name,
		ItemType: key.ItemType,
	}
	setSums(&item, items)
	setAverages(&item, items)

	return item
}

// getAllProviders returns the AWS provider and any providers in the config file
func getAllProviders(ctx context.Context, reportRange model.TimeRange, config *model.CostConfig) ([]model.CloudProvider, error) {
	awsProvider, err := getAWSProvider(ctx, reportRange, config)
	if err != nil {
		return nil, err
	}

	providers := []model.CloudProvider{*awsProvider}

	for _, p := range config.Providers {
		grip.Infoln("adding cost data for aws provider:", p.Name)
		providers = append(providers, model.CloudProvider{
			Name: p.Name,
			Cost: p.Cost,
		})
	}

	return providers, nil
}

// getAWSProvider specifically creates a provider for AWS and populates those accounts
func getAWSProvider(ctx context.Context, reportRange model.TimeRange, config *model.CostConfig) (*model.CloudProvider, error) {
	catcher := grip.NewSimpleCatcher()
	res := model.CloudProvider{
		Name: "aws",
	}

	for _, aws := range config.Amazon.Accounts {
		account, err := getAWSAccountByOwner(ctx, reportRange, config, aws)
		if err != nil {
			catcher.Add(err)
		}

		res.Cost += account.Cost
		res.Accounts = append(res.Accounts, *account)
	}

	if catcher.HasErrors() {
		return nil, catcher.Resolve()
	}

	return &res, nil
}

//getAWSAccountByOwner gets account information using the API keys labeled by the owner string.
func getAWSAccountByOwner(ctx context.Context, reportRange model.TimeRange, config *model.CostConfig, account model.CostConfigAmazonAccount) (*model.CloudAccount, error) {
	grip.Infof("Compiling data for account owner %s", account.Name)
	client, err := amazon.NewClientWithInfo(account.Key, account.Secret)
	if err != nil {
		return nil, errors.Wrapf(err, "Problem getting client %s", account.Name)
	}

	instances := amazon.NewServices()

	if err = client.GetEC2Instances(ctx, reportRange, instances); err != nil {
		return nil, errors.Wrap(err, "Problem getting EC2 instances")
	}

	if err = client.AddEBSItems(ctx, instances, reportRange, &config.Amazon.EBSPrices); err != nil {
		return nil, errors.Wrap(err, "Problem getting EBS instances")
	}
	s3info := config.Amazon.S3Info
	s3info.Owner = account.Name
	ec2Service := model.AccountService{
		Name: string(amazon.EC2Service),
	}
	ebsService := model.AccountService{
		Name: string(amazon.EBSService),
	}
	// s3Service := model.AccountService{
	//	Name: string(amazon.S3Service),
	// }
	// s3Service.Cost, err = client.GetS3Cost(&s3info, reportRange)
	// if err != nil {
	//	return nil, errors.Wrap(err, "Error fetching S3 Spending CSV")
	// }
	grip.Infof("Iterating through %d instance types", instances.Len())
	var accountCost float64
	for pair := range instances.Iter() {
		key := pair.Key
		items := pair.Value

		item := createCostItemFromAmazonItems(key, items)
		cost := item.GetCost(reportRange)
		accountCost += cost
		if key.Service == amazon.EC2Service {
			ec2Service.Cost += cost
			ec2Service.Items = append(ec2Service.Items, item)
		} else {
			ebsService.Cost += cost
			ebsService.Items = append(ebsService.Items, item)
		}
	}

	return &model.CloudAccount{
		Name: account.Name,
		Cost: accountCost,
		Services: []model.AccountService{
			ec2Service,
			ebsService,
			// s3Service,
		},
	}, nil
}
