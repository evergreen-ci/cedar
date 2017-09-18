package cost

import (
	"github.com/evergreen-ci/sink/amazon"
	"github.com/evergreen-ci/sink/model"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const aws = "aws"

// setItems sets the number of launched and terminated instances of the given cost item.
// The sums are calculated from the information in the Item array.
func setSums(res *model.ServiceItem, items []*amazon.Item) {
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
func setAverages(res *model.ServiceItem, items []*amazon.Item) {
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
		res.AvgPrice = float32(avg(prices))
	}
	if len(fixedPrices) != 0 {
		res.FixedPrice = float32(avg(fixedPrices))
	}
	if len(uptimes) != 0 {
		res.AvgUptime = float32(avg(uptimes))
	}
}

// createItemFromEC2Instance creates a new cost.Item using a key/item array pair.
func createCostItemFromAmazonItems(key amazon.ItemKey, items []*amazon.Item) model.ServiceItem {
	item := model.ServiceItem{
		Name:     key.Name,
		ItemType: string(key.ItemType),
	}
	setSums(&item, items)
	setAverages(&item, items)

	return item
}

// getAllProviders returns the AWS provider and any providers in the config file
func getAllProviders(ctx context.Context, reportRange timeRange, config *model.CostConfig) ([]model.CloudProvider, error) {
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

//getAWSAccountByOwner gets account information using the API keys labeled by the owner string.
func getAWSAccountByOwner(ctx context.Context, reportRange amazon.TimeRange, config *model.CostConfig, owner string) (*model.CloudAccount, error) {
	grip.Infof("Compiling data for account owner %s", owner)
	client, err := amazon.NewClientAuto()
	if err != nil {
		return nil, errors.Wrapf(err, "Problem getting client %s", owner)
	}
	instances, err := client.GetEC2Instances(ctx, reportRange)
	if err != nil {
		return nil, errors.Wrap(err, "Problem getting EC2 instances")
	}
	instances, err = client.AddEBSItems(ctx, instances, reportRange, &config.Amazon.EBSPrices)
	if err != nil {
		return nil, errors.Wrap(err, "Problem getting EBS instances")
	}
	s3info := config.Amazon.S3Info
	s3info.Owner = owner
	ec2Service := model.AccountService{
		Name: string(amazon.EC2Service),
	}
	ebsService := model.AccountService{
		Name: string(amazon.EBSService),
	}
	// s3Service := model.AccountService{
	// 	Name: string(amazon.S3Service),
	// }
	// s3Service.Cost, err = client.GetS3Cost(&s3info, reportRange)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "Error fetching S3 Spending CSV")
	// }
	grip.Infof("Iterating through %d instance types", len(instances))
	for key, items := range instances {
		item := createCostItemFromAmazonItems(key, items)
		if key.Service == amazon.EC2Service {
			ec2Service.Items = append(ec2Service.Items, item)
		} else {
			ebsService.Items = append(ebsService.Items, item)
		}
	}
	account := &model.CloudAccount{
		Name: owner,
		Services: []model.AccountService{
			ec2Service,
			ebsService,
			// s3Service,
		},
	}
	return account, nil
}

// getAWSAccounts takes in a range for the report, and returns an array of accounts
// containing EC2 and EBS instances.
func getAWSAccounts(ctx context.Context, reportRange timeRange, config *model.CostConfig) ([]model.CloudAccount, error) {
	awsReportRange := amazon.TimeRange{
		Start: reportRange.start,
		End:   reportRange.end,
	}
	var allAccounts []model.CloudAccount
	for _, owner := range config.Amazon.Accounts {
		account, err := getAWSAccountByOwner(ctx, awsReportRange, config, owner)
		if err != nil {
			return nil, err
		}
		allAccounts = append(allAccounts, *account)
	}
	return allAccounts, nil
}

// getAWSProvider specifically creates a provider for AWS and populates those accounts
func getAWSProvider(ctx context.Context, reportRange timeRange, config *model.CostConfig) (*model.CloudProvider, error) {
	var err error
	res := &model.CloudProvider{
		Name: aws,
	}
	res.Accounts, err = getAWSAccounts(ctx, reportRange, config)
	if err != nil {
		return nil, err
	}
	return res, nil
}
