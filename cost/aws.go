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
func setSums(res *Item, items []*amazon.Item) {
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
func setAverages(res *Item, items []*amazon.Item) {
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
func createCostItemFromAmazonItems(key amazon.ItemKey, items []*amazon.Item) Item {
	item := Item{
		Name:     key.Name,
		ItemType: string(key.ItemType),
	}
	setSums(&item, items)
	setAverages(&item, items)

	return item
}

// getAllProviders returns the AWS provider and any providers in the config file
func getAllProviders(ctx context.Context, reportRange timeRange, config *model.CostConfig) ([]Provider, error) {
	awsProvider, err := getAWSProvider(ctx, reportRange, config)
	if err != nil {
		return nil, err
	}

	providers := []Provider{*awsProvider}

	for _, p := range config.Providers {
		providers = append(providers, Provider{
			Name: p.Name,
			Cost: p.Cost,
		})
	}

	return providers, nil
}

//getAWSAccountByOwner gets account information using the API keys labeled by the owner string.
func getAWSAccountByOwner(ctx context.Context, reportRange amazon.TimeRange, config *model.CostConfig,
	owner string) (*Account, error) {
	grip.Infof("Compiling data for account owner %s", owner)
	client, err := amazon.NewClient(owner)
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
	ec2Service := Service{
		Name: string(amazon.EC2Service),
	}
	ebsService := Service{
		Name: string(amazon.EBSService),
	}
	s3Service := Service{
		Name: string(amazon.S3Service),
	}
	s3Service.Cost, err = client.GetS3Cost(&s3info, reportRange)
	if err != nil {
		return nil, errors.Wrap(err, "Error fetching S3 Spending CSV")
	}
	grip.Infof("Iterating through %d instance types", len(instances))
	for key, items := range instances {
		item := createCostItemFromAmazonItems(key, items)
		if key.Service == amazon.EC2Service {
			ec2Service.Items = append(ec2Service.Items, item)
		} else {
			ebsService.Items = append(ebsService.Items, item)
		}
	}
	account := &Account{
		Name:     owner,
		Services: []Service{ec2Service, ebsService, s3Service},
	}
	return account, nil
}

// getAWSAccounts takes in a range for the report, and returns an array of accounts
// containing EC2 and EBS instances.
func getAWSAccounts(ctx context.Context, reportRange timeRange, config *model.CostConfig) ([]Account, error) {
	awsReportRange := amazon.TimeRange{
		Start: reportRange.start,
		End:   reportRange.end,
	}
	var allAccounts []Account
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
func getAWSProvider(ctx context.Context, reportRange timeRange, config *model.CostConfig) (*Provider, error) {
	var err error
	res := &Provider{
		Name: aws,
	}
	res.Accounts, err = getAWSAccounts(ctx, reportRange, config)
	if err != nil {
		return nil, err
	}
	return res, nil
}
