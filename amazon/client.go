package amazon

import (
	"math"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// Client holds information for the amazon client
type Client struct {
	ec2Client *ec2.EC2
	s3Client  *s3.S3
}

// newClient returns a new populated client
func newClient(auth *credentials.Credentials) (*Client, error) {
	client := &Client{}
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Credentials: auth,
	})

	if err != nil {
		return nil, errors.WithStack(err)
	}

	client.ec2Client = ec2.New(sess)
	client.s3Client = s3.New(sess)

	return client, nil
}

func NewClientAuto() (*Client, error) {
	creds := credentials.NewEnvCredentials()
	if _, err := creds.Get(); err == nil {
		return newClient(creds)
	}

	for _, profile := range []string{"sink", "bcr", "xgen", "default"} {
		creds = credentials.NewSharedCredentials("", profile)

		if _, err := creds.Get(); err == nil {
			return newClient(creds)
		}
	}

	return nil, errors.New("could not auto-discover aws credentials in the environment or config file ")
}

func NewClientWithInfo(keyID, secret string) (*Client, error) {
	creds := credentials.NewStaticCredentials(keyID, secret, "")
	if _, err := creds.Get(); err != nil {
		return nil, errors.Wrapf(err, "problem resolving credentials for keyID: %s", keyID)
	}

	return newClient(creds)
}

// getTagVal retrieves from an array of spotEC2 tags the value string for the given key
func getTagVal(tags []*ec2.Tag, key string) (string, error) {
	if tags == nil {
		return "", errors.New("No tags given")
	}
	for _, tag := range tags {
		if *tag.Key == key {
			return *tag.Value, nil
		}
	}
	return "", errors.New("Tag doesn't exist")
}

// populateSpotKey creates an ItemKey using a spot request and the item type
func populateSpotKey(inst *ec2.SpotInstanceRequest) ItemKey {
	return ItemKey{
		Service:  EC2Service,
		Name:     *inst.LaunchSpecification.InstanceType,
		ItemType: spot,
	}
}

// populateReservedKey creates an ItemKey using a reserved instance
func populateReservedKey(inst *ec2.ReservedInstances) ItemKey {
	return ItemKey{
		Service:      EC2Service,
		Name:         *inst.InstanceType,
		ItemType:     reserved,
		duration:     *inst.Duration,
		offeringType: *inst.OfferingType,
	}
}

// populateOnDemandKey creates an ItemKey using an on-demand instance
func populateOnDemandKey(inst *ec2.Instance) ItemKey {
	return ItemKey{
		Service:  EC2Service,
		Name:     *inst.InstanceType,
		ItemType: onDemand,
	}
}

// populateItemFromSpot creates an Item from a spot request result and fills in
// the isLaunched and isTerminated values.
func populateItemFromSpot(req *ec2.SpotInstanceRequest) *Item {
	if *req.State == ec2.SpotInstanceStateOpen || *req.State == ec2.SpotInstanceStateFailed {
		return nil
	}
	if req.Status == nil || stringInSlice(*req.Status.Code, ignoreCodes) {
		return nil
	}

	item := &Item{}

	if *req.State == ec2.SpotInstanceStateActive || *(req.Status.Code) == marked {
		item.Launched = true
		return item
	}
	item.Terminated = true
	return item
}

// populateItemFromReserved creates an Item from a reserved response item and
// fills in the isLaunched, isTerminated, and count values.
func populateItemFromReserved(inst *ec2.ReservedInstances) *Item {
	var isTerminated, isLaunched bool
	if *inst.State == ec2.ReservedInstanceStateRetired {
		isTerminated = true
	} else if *inst.State == ec2.ReservedInstanceStateActive {
		isLaunched = true
	} else {
		return nil
	}

	return &Item{
		Launched:   isLaunched,
		Terminated: isTerminated,
		Count:      int(*inst.InstanceCount),
	}
}

// populateItemFromOnDemandcreates an Item from an on-demand instance and
// fills in the isLaunched and isTerminated fields.
func populateItemFromOnDemand(inst *ec2.Instance) *Item {
	item := &Item{}
	if *inst.State.Name == ec2.InstanceStateNamePending {
		return nil
	} else if *inst.State.Name == ec2.InstanceStateNameRunning {
		item.Launched = true
	} else {
		item.Terminated = true
	}
	return item
}

// getSpotRange returns the instance running time range from the spot request result.
// Note: if the instance was terminated by amazon, we subtract one hour.
func getSpotRange(req *ec2.SpotInstanceRequest) TimeRange {
	endTime, _ := time.Parse(utcLayout, "")
	res := TimeRange{}
	start, err := getTagVal(req.Tags, "start-time")
	if err != nil {
		return res
	}
	startTime, err := time.Parse(tagLayout, start)
	if err != nil {
		return res
	}
	res.Start = startTime
	if *req.State == ec2.SpotInstanceStateActive { // no end time
		return res
	}
	if req.Status != nil && req.Status.UpdateTime != nil {
		endTime = *req.Status.UpdateTime
		if stringInSlice(*req.Status.Code, amazonTerminated) {
			endTime = endTime.Add(-time.Hour)
			// If our instance was running for less than an hour
			if endTime.Before(startTime) {
				return TimeRange{}
			}
		}
	}
	res.End = endTime
	return res
}

// getReservedRange returns the instance running time range from the reserved instance response item.
func getReservedRange(inst *ec2.ReservedInstances) TimeRange {
	res := TimeRange{}
	if inst.Start == nil || inst.End == nil {
		return res
	}

	res.Start = *inst.Start
	res.End = *inst.End
	return res
}

// getOnDemandRange returns the instance running time range from the reserved instance response item.
// We assume end time in the state transition reason to be "<some message> (YYYY-MM-DD HH:MM:SS MST)"
func getOnDemandRange(inst *ec2.Instance) TimeRange {
	res := TimeRange{}
	if inst.LaunchTime == nil || *inst.State.Name == ec2.InstanceStateNamePending {
		return res
	}
	res.Start = *inst.LaunchTime

	if *inst.State.Name == ec2.InstanceStateNameRunning {
		return res
	}
	//retrieving the end time from the state transition reason
	reason := *inst.StateTransitionReason
	split := strings.Split(reason, "(")
	if len(split) <= 1 { // no time in state transition reason
		return TimeRange{}
	}
	timeString := strings.Trim(split[1], ")")
	end, err := time.Parse(ondemandLayout, timeString)
	if err != nil {
		return TimeRange{}
	}
	res.End = end
	return res
}

// getUptimeRange returns the time range that the item is running, within
// the constraints of the report range. Note if the instance doesn't overlap
// with the report time range, it returns an empty range.
func getUptimeRange(itemRange TimeRange, reportRange TimeRange) TimeRange {
	if itemRange == (TimeRange{}) {
		return TimeRange{}
	} else if itemRange.Start.After(reportRange.End) {
		return TimeRange{}
	} else if !itemRange.End.IsZero() && itemRange.End.Before(reportRange.Start) {
		return TimeRange{}
	}
	// decide uptime start value
	start := reportRange.Start
	if itemRange.Start.After(start) {
		start = itemRange.Start
	}
	// decide uptime end value
	end := reportRange.End
	if !itemRange.End.IsZero() && itemRange.End.Before(end) {
		end = itemRange.End
	}
	return TimeRange{Start: start, End: end}
}

// setUptime returns the start/end time within the report for the item given,
// and sets the end time - start time as the item's uptime.
// Note that the uptime is rounded up, in hours.
func (item *Item) setUptime(times TimeRange) {
	uptime := times.End.Sub(times.Start).Hours()
	item.Uptime = int(math.Ceil(uptime))
}

// setReservedPrice takes in a reserved instance item and sets the item price
// based on the instance's offering type and prices.

func (item *Item) setReservedPrice(inst *ec2.ReservedInstances) {
	instType := *inst.OfferingType
	if instType == ec2.OfferingTypeValuesAllUpfront || instType == ec2.OfferingTypeValuesPartialUpfront {
		item.FixedPrice = *inst.FixedPrice
	}
	if instType == ec2.OfferingTypeValuesNoUpfront || instType == ec2.OfferingTypeValuesPartialUpfront {
		if inst.RecurringCharges != nil {
			item.Price = *inst.RecurringCharges[0].Amount * float64(item.Uptime)
		}
	}
}

// setOnDemandPrice takes in an on-demand instance item and prices object and
// sets the item price based on the instance's availability zone, instance type,
// product description, and uptime. In case of error, the price is set to 0.

func (item *Item) setOnDemandPrice(inst *ec2.Instance, pricing *prices) {
	var productDesc string
	if inst.Placement == nil || inst.Placement.AvailabilityZone == nil {
		return
	}
	if inst.InstanceType == nil {
		return
	}
	if inst.Platform == nil {
		productDesc = "Linux"
	} else {
		productDesc = *inst.Platform
	}
	instanceType := *inst.InstanceType
	availZone := *inst.Placement.AvailabilityZone
	price := pricing.fetchPrice(productDesc, instanceType, availZone)
	item.Price = price * float64(item.Uptime)
}

// isValidInstance takes in an item, an error, and two time ranges.
// It returns true if the item is not nil, there is no error, and the TimeRanges are non empty.
func isValidInstance(item *Item, err error, itemRange TimeRange, uptimeRange TimeRange) bool {
	if item == nil || err != nil || itemRange == (TimeRange{}) || uptimeRange == (TimeRange{}) {
		return false
	}
	return true
}

// updateAccounts uses the given key and owner to add the item to the accounts object.
func (items itemHash) updateItems(item *Item, key ItemKey) {
	placed := false
	for curKey, curItems := range items {
		//Check if we can add it to an existing key
		if key.Name == curKey.Name && key.ItemType == curKey.ItemType &&
			key.duration == curKey.duration && key.Service == curKey.Service {
			placed = true
			items[curKey] = append(curItems, item)
			break
		}
	}
	if placed == false {
		items[key] = []*Item{item}
	}
}

// invalidVolume returns true if a necessary volume field is nil, or if
// the volume was created after the report range or is still being created.
func invalidVolume(vol *ec2.Volume, reportRange TimeRange) bool {
	if vol.State == nil || vol.CreateTime == nil || vol.VolumeType == nil {
		return true
	}
	state := *vol.State
	createTime := *vol.CreateTime
	if createTime.After(reportRange.End) || state == ec2.VolumeStateCreating ||
		state == ec2.VolumeStateError {
		return true
	}
	return false
}

// getSpotPricePage recursively iterates through pages of spot requests and returns
// a compiled ec2.DescribeSpotPriceHistoryOutput object.
func (c *Client) getSpotPricePage(ctx context.Context, req *ec2.SpotInstanceRequest, times TimeRange,
	nextToken *string) *ec2.DescribeSpotPriceHistoryOutput {
	input := &ec2.DescribeSpotPriceHistoryInput{
		InstanceTypes:       []*string{req.LaunchSpecification.InstanceType},
		ProductDescriptions: []*string{req.ProductDescription},
		AvailabilityZone:    req.AvailabilityZoneGroup,
		StartTime:           &times.Start,
		EndTime:             &times.End,
	}
	if nextToken != nil && *nextToken != "" {
		input = input.SetNextToken(*nextToken)
	}

	res, err := c.ec2Client.DescribeSpotPriceHistoryWithContext(ctx, input)
	if err != nil {
		return nil
	}
	for res.NextToken != nil && *res.NextToken != "" {
		prevPrices := res.SpotPriceHistory
		res = c.getSpotPricePage(ctx, req, times, res.NextToken)
		if res == nil {
			return nil
		}
		res.SpotPriceHistory = append(prevPrices, res.SpotPriceHistory...)
	}
	return res
}

// getSpotPrice takes in a spot request, a product description, and a time range.
// It queries the EC2 API and returns the overall price.
func (c *Client) getSpotPrice(ctx context.Context, req *ec2.SpotInstanceRequest, times TimeRange) float64 {
	//How to get description?
	priceData := c.getSpotPricePage(ctx, req, times, nil)
	if priceData == nil {
		return 0.0
	}
	return spotPrices(priceData.SpotPriceHistory).calculatePrice(times)
}

// getEC2SpotInstances gets spot EC2 Instances and retrieves its uptime,
// average (hourly and fixed) price, number of launched and terminated instances,
// and item type. These instances are then added to the given accounts.
func (c *Client) getEC2SpotInstances(ctx context.Context, items itemHash, reportRange TimeRange) (itemHash, error) {
	resp, err := c.ec2Client.DescribeSpotInstanceRequestsWithContext(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Error from SpotRequests API call")
	}
	for _, req := range resp.SpotInstanceRequests {
		key := populateSpotKey(req)
		item := populateItemFromSpot(req)
		itemRange := getSpotRange(req)
		//instance start and end
		uptimeRange := getUptimeRange(itemRange, reportRange)
		if !isValidInstance(item, err, itemRange, uptimeRange) {
			//skip to the next instance
			continue
		}
		item.Product = *req.ProductDescription
		item.setUptime(uptimeRange)
		item.Price = c.getSpotPrice(ctx, req, uptimeRange)

		items.updateItems(item, key)

	}
	return items, nil
}

// getEC2ReservedInstances gets reserved EC2 Instances and retrieves its uptime,
// average (hourly and fixed) price, number of launched and terminated instances,
// and item type. These instances are then added to the given accounts.
func (c *Client) getEC2ReservedInstances(ctx context.Context, items itemHash,
	reportRange TimeRange) (itemHash, error) {
	resp, err := c.ec2Client.DescribeReservedInstancesWithContext(ctx, nil)
	if err != nil {
		return nil, err
	}
	for _, inst := range resp.ReservedInstances {
		key := populateReservedKey(inst)
		item := populateItemFromReserved(inst)
		itemRange := getReservedRange(inst)
		//instance start and end
		uptimeRange := getUptimeRange(itemRange, reportRange)
		if !isValidInstance(item, err, itemRange, uptimeRange) {
			continue
		}

		item.setUptime(uptimeRange)
		item.setReservedPrice(inst)

		items.updateItems(item, key)

	}
	return items, nil
}

// getEC2OnDemandInstancesPage recursively iterates through pages of on-demand instances
// and retrieves its uptime, average hourly price, number of launched and terminated instances,
// and item type. These instances are then added to the given accounts.
func (c *Client) getEC2OnDemandInstancesPage(ctx context.Context, items itemHash, reportRange TimeRange,
	pricing *prices, nextToken *string) (itemHash, error) {
	var input *ec2.DescribeInstancesInput
	if nextToken != nil && *nextToken != "" {
		//create filter
		input = input.SetNextToken(*nextToken)
	}
	resp, err := c.ec2Client.DescribeInstancesWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	//iterate through instances
	for _, res := range resp.Reservations {
		for _, inst := range res.Instances {
			if inst.InstanceLifecycle != nil {
				continue
			}
			key := populateOnDemandKey(inst)
			item := populateItemFromOnDemand(inst)
			itemRange := getOnDemandRange(inst)
			//instance start and end
			uptimeRange := getUptimeRange(itemRange, reportRange)
			if !isValidInstance(item, err, itemRange, uptimeRange) {
				continue
			}
			item.setUptime(uptimeRange)
			item.setOnDemandPrice(inst, pricing)

			items.updateItems(item, key)
		}
	}
	// if there's a next page, recursively add next page information
	for resp.NextToken != nil && *resp.NextToken != "" {
		items, err = c.getEC2OnDemandInstancesPage(ctx, items, reportRange,
			pricing, resp.NextToken)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

// getEC2OnDemandInstances gets all EC2 on-demand instances and retrieves its uptime,
// average hourly price, number of launched and terminated instances,
// and item type. These instances are then added to the given accounts.
func (c *Client) getEC2OnDemandInstances(ctx context.Context, items itemHash,
	reportRange TimeRange) (itemHash, error) {
	pricing, err := getOnDemandPriceInformation()
	if err != nil {
		return nil, errors.Wrap(err, "Problem fetching on-demand price information")
	}
	items, err = c.getEC2OnDemandInstancesPage(ctx, items, reportRange,
		pricing, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Problem fetching on-demand instances page")
	}
	return items, nil
}

// GetEC2Instances gets all EC2Instances and creates an array of accounts.
// Note this function is public but I may change that when adding non EC2 Amazon services.
func (c *Client) GetEC2Instances(ctx context.Context, reportRange TimeRange) (itemHash, error) {
	// accounts maps from account name to the items
	items := make(itemHash)
	grip.Info("Getting EC2 Reserved Instances")
	items, err := c.getEC2ReservedInstances(ctx, items, reportRange)
	if err != nil {
		return nil, err
	}

	grip.Info("Getting EC2 On-Demand Instances")
	items, err = c.getEC2OnDemandInstances(ctx, items, reportRange)
	if err != nil {
		return nil, err
	}

	grip.Info("Getting EC2 Spot Instances")
	items, err = c.getEC2SpotInstances(ctx, items, reportRange)
	if err != nil {
		return nil, err
	}

	return items, nil
}

// addEBSItemsPage recursively iterates through pages of EBS volumes
// and retrieves its average hourly price, number of launched and terminated instances,
// and volume type and adds this information to accounts.
func (c *Client) addEBSItemsPage(ctx context.Context, items itemHash, reportRange TimeRange, pricing *EBSPrices,
	nextToken *string) (itemHash, error) {
	grip.Info("Getting EBS Items")
	input := &ec2.DescribeVolumesInput{}
	if nextToken != nil && *nextToken != "" {
		//create filter
		input = input.SetNextToken(*nextToken)
	}
	resp, err := c.ec2Client.DescribeVolumesWithContext(ctx, input)
	if err != nil || resp == nil {
		return nil, err
	}
	for _, vol := range resp.Volumes {
		if invalidVolume(vol, reportRange) {
			continue
		}
		key := ItemKey{
			ItemType: itemType(*vol.VolumeType),
			Service:  EBSService,
		}
		item := &Item{}
		if *vol.State == ec2.VolumeStateAvailable || *vol.State == ec2.VolumeStateInUse {
			item.Launched = true
			item.Price = pricing.getEBSPrice(vol, reportRange)
		} else { //state is deleting, deleted, or error
			item.Terminated = true
		}
		items.updateItems(item, key)
	}
	// if there's a next page, recursively add next page information
	for resp.NextToken != nil && *resp.NextToken != "" {
		items, err = c.addEBSItemsPage(ctx, items, reportRange,
			pricing, resp.NextToken)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

// AddEBSItems gets all EBSVolumes and adds these items to accounts.
func (c *Client) AddEBSItems(ctx context.Context, items itemHash, reportRange TimeRange, pricing *EBSPrices) (itemHash, error) {
	items, err := c.addEBSItemsPage(ctx, items, reportRange, pricing, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Problem fetching EBS items page")
	}
	return items, nil
}
