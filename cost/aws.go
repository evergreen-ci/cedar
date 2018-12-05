package cost

import (
	"context"

	"github.com/evergreen-ci/cedar/model"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

// setItems sets the number of launched and terminated instances of the given cost item.
// The sums are calculated from the information in the Item array.
func setSums(res *model.ServiceItem, items []AWSItem) {
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

		res.TotalCost += item.Price
	}
}

// setAverages sets the average price, fixed price, and uptime of the given cost item.
// The averages are calculated from the information in the Item array.
func setAverages(res *model.ServiceItem, items []AWSItem) {
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
		res.AvgPrice = util.Average(prices)
	}
	if len(fixedPrices) != 0 {
		res.FixedPrice = util.Average(fixedPrices)
	}
	if len(uptimes) != 0 {
		res.AvgUptime = util.Average(uptimes)
	}
}

// createItemFromEC2Instance creates a new cost.Item using a key/item array pair.
func createCostItemFromAmazonItems(key AWSItemKey, items []AWSItem) model.ServiceItem {
	item := model.ServiceItem{
		Name:     key.Name,
		ItemType: key.ItemType,
	}
	setSums(&item, items)
	setAverages(&item, items)

	return item
}

// getAllProviders returns the AWS provider and any providers in the config file
func getAllProviders(ctx context.Context, reportRange util.TimeRange, config *model.CostConfig) ([]model.CloudProvider, error) {
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
func getAWSProvider(ctx context.Context, reportRange util.TimeRange, config *model.CostConfig) (*model.CloudProvider, error) {
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
func getAWSAccountByOwner(ctx context.Context, reportRange util.TimeRange, config *model.CostConfig, account model.CostConfigAmazonAccount) (*model.CloudAccount, error) {
	grip.Infof("Compiling data for account owner %s", account.Name)
	client, err := NewAWSClientWithInfo(account.Key, account.Secret)
	if err != nil {
		return nil, errors.Wrapf(err, "Problem getting client %s", account.Name)
	}

	instances := NewAWSServices()

	if err = client.GetEC2Instances(ctx, reportRange, instances); err != nil {
		return nil, errors.Wrap(err, "Problem getting EC2 instances")
	}

	if err = client.AddEBSItems(ctx, instances, reportRange, &config.Amazon.EBSPrices); err != nil {
		return nil, errors.Wrap(err, "Problem getting EBS instances")
	}
	s3conf := config.Amazon.S3Info
	s3conf.Owner = account.Name
	ec2Info := model.AccountService{
		Name: string(ec2Service),
	}
	ebsInfo := model.AccountService{
		Name: string(ebsService),
	}
	// s3SInfo := model.AccountService{
	//	Name: string(s3Service),
	// }
	// s3Info.Cost, err = client.GetS3Cost(&s3conf, reportRange)
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
		if key.Service == ec2Service {
			ec2Info.Cost += cost
			ec2Info.Items = append(ec2Info.Items, item)
		} else {
			ebsInfo.Cost += cost
			ebsInfo.Items = append(ebsInfo.Items, item)
		}
	}

	return &model.CloudAccount{
		Name: account.Name,
		Cost: accountCost,
		Services: []model.AccountService{
			ec2Info,
			ebsInfo,
			// s3info,
		},
	}, nil
}
