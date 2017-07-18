package amazon

import (
	"math"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

type itemType string
type resType string

const (
	// layouts use reference Mon Jan 2 15:04:05 -0700 MST 2006
	tagLayout      = "20060102150405"
	utcLayout      = "2006-01-02T15:04:05.000Z"
	spot           = itemType("spot")
	reserved       = itemType("reserved")
	fixed          = resType("Fixed Price")
	hourly         = resType("No Upfront")
	partial        = resType("Partial Upfront")
	startTag       = "start-time"
	active         = "active"
	open           = "open"
	failed         = "failed"
	retired        = "retired"
	marked         = "marked-for-termination"
	defaultAccount = "default"
)

var ignoreCodes = []string{"canceled-before-fulfillment", "schedule-expired", "bad-parameters", "system-error"}
var amazonTerminated = []string{"instance-terminated-by-price", "instance-terminated-no-capacity",
	"instance-terminated-capacity-oversubscribed", "instance-terminated-launch-group-constraint"}

// Client holds information for the amazon client
type Client struct {
	ec2Client *ec2.EC2
}

// EC2Item is information for an item for a particular Name and ItemType
type EC2Item struct {
	Product    string
	Launched   bool
	Terminated bool
	Price      float64
	FixedPrice float64
	Uptime     int //stored in number of hours
	Count      int
}

// ItemKey is used together with EC2Item to create a hashtable from ItemKey to []EC2Item
type ItemKey struct {
	Name         string
	ItemType     itemType
	offeringType resType
	duration     int64
}

// TimeRange defines a time range by storing a start/end time
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Maps the ItemKey to an array of EC2Items
type itemHash map[*ItemKey][]*EC2Item

// AccountHash maps an owner to an itemHash, i.e. ItemKeys and EC2Items
type AccountHash map[string]itemHash

// NewClient returns a new populated client
func NewClient() *Client {
	client := &Client{}
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	}))
	client.ec2Client = ec2.New(sess)
	return client
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
func populateSpotKey(inst *ec2.SpotInstanceRequest) *ItemKey {
	return &ItemKey{
		Name:     *inst.LaunchSpecification.InstanceType,
		ItemType: spot,
	}
}

// populateReservedKey creates an ItemKey using a reserved instance and the item type
func populateReservedKey(inst *ec2.ReservedInstances) *ItemKey {
	return &ItemKey{
		Name:         *inst.InstanceType,
		ItemType:     reserved,
		duration:     *inst.Duration,
		offeringType: resType(*inst.OfferingType),
	}
}

// populateItemFromSpot creates an EC2Item from a spot request result and fills in
// the isLaunched and isTerminated values.
func populateItemFromSpot(req *ec2.SpotInstanceRequest) *EC2Item {
	item := &EC2Item{}
	if *req.State == open || *req.State == failed {
		return nil
	} else if stringInSlice(*req.Status.Code, ignoreCodes) {
		return nil
	} else if *req.State == active || *(req.Status.Code) == marked {
		item.Launched = true
		return item
	}
	item.Terminated = true
	return item
}

// populateItemFromReserved creates an EC2Item from a reserved response item and
// fills in the isLaunched, isTerminated, and count values.
func populateItemFromReserved(inst *ec2.ReservedInstances) *EC2Item {
	var isTerminated, isLaunched bool
	if *inst.State == retired {
		isTerminated = true
	} else if *inst.State == active {
		isLaunched = true
	} else {
		return nil
	}

	return &EC2Item{
		Launched:   isLaunched,
		Terminated: isTerminated,
		Count:      int(*inst.InstanceCount),
	}
}

// getSpotRange returns the instance running time range from the spot request result.
// Note: if the instance was terminated by amazon, we subtract one hour.
func getSpotRange(req *ec2.SpotInstanceRequest) TimeRange {
	endTime, _ := time.Parse(utcLayout, "") //TODO: Should be the zero instance but check this
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
	if *req.State == active { // no end time
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
func (item *EC2Item) setUptime(times TimeRange) {
	uptime := times.End.Sub(times.Start).Hours()
	item.Uptime = int(math.Ceil(uptime))
}

// setReservedPrice takes in a reserved instance item and sets the item price
// based on the instance's offering type and prices.
func (item *EC2Item) setReservedPrice(inst *ec2.ReservedInstances) {
	instType := resType(*inst.OfferingType)
	if instType == fixed || instType == partial {
		item.FixedPrice = *inst.FixedPrice
	}
	if instType == hourly || instType == partial {
		if inst.RecurringCharges != nil {
			item.Price = *inst.RecurringCharges[0].Amount * float64(item.Uptime)
		}
	}
}

// isValidInstance takes in an item, an error, and two time ranges.
// It returns true if the item is not nil, there is no error, and the TimeRanges are non empty.
func isValidInstance(item *EC2Item, err error, itemRange TimeRange, uptimeRange TimeRange) bool {
	if item == nil || err != nil || itemRange == (TimeRange{}) || uptimeRange == (TimeRange{}) {
		return false
	}
	return true
}

// updateAccounts uses the given key and owner to add the item to the accounts object.
func (accounts AccountHash) updateAccounts(owner string, item *EC2Item, key *ItemKey) {
	curAccount := accounts[owner]
	if curAccount == nil {
		curAccount = make(itemHash)
	}
	placed := false
	for curKey, curItems := range curAccount {
		//Check if we can add it to an existing key
		if key.Name == curKey.Name && key.ItemType == curKey.ItemType &&
			key.duration == curKey.duration {
			placed = true
			curAccount[curKey] = append(curItems, item)
			break
		}
	}
	if placed == false {
		curAccount[key] = []*EC2Item{item}
	}
	accounts[owner] = curAccount
	//return accounts
}

// getSpotPrice takes in a spot request, a product description, and a time range.
// It queries the EC2 API and returns the overall price.
func (c *Client) getSpotPrice(req *ec2.SpotInstanceRequest, times TimeRange) float64 {
	//How to get description?
	input := &ec2.DescribeSpotPriceHistoryInput{
		InstanceTypes:       []*string{req.LaunchSpecification.InstanceType},
		ProductDescriptions: []*string{req.ProductDescription},
		AvailabilityZone:    req.AvailabilityZoneGroup,
		StartTime:           &times.Start,
		EndTime:             &times.End,
	}

	res, err := c.ec2Client.DescribeSpotPriceHistory(input)
	// TODO: work in something for nextToken
	if err != nil {
		return 0.0
	}
	return spotPrices(res.SpotPriceHistory).calculatePrice(times)
}

// getEC2SpotInstances gets reserved EC2 Instances and retrieves its uptime,
// average (hourly and fixed) price, number of launched and terminated instances,
// and item type. These instances are then added to the given accounts.
func (c *Client) getEC2SpotInstances(accounts AccountHash, reportRange TimeRange) (AccountHash, error) {
	resp, err := c.ec2Client.DescribeSpotInstanceRequests(nil)
	if err != nil {
		return nil, errors.Wrap(err, "Error from SpotRequests API call")
	}
	for _, req := range resp.SpotInstanceRequests {
		owner := defaultAccount
		key := populateSpotKey(req)
		item := populateItemFromSpot(req)
		itemRange := getSpotRange(req)
		//instance start and end
		uptimeRange := getUptimeRange(itemRange, reportRange)
		if !isValidInstance(item, err, itemRange, uptimeRange) {
			//skip to the next instance
			break
		}
		item.Product = *req.ProductDescription
		item.setUptime(uptimeRange)
		item.Price = c.getSpotPrice(req, uptimeRange)

		accounts.updateAccounts(owner, item, key)

	}
	return accounts, nil
}

// getEC2ReservedInstances gets reserved EC2 Instances and retrieves its uptime,
// average (hourly and fixed) price, number of launched and terminated instances,
// and item type. These instances are then added to the given accounts.
func (c *Client) getEC2ReservedInstances(accounts AccountHash,
	reportRange TimeRange) (AccountHash, error) {
	resp, err := c.ec2Client.DescribeReservedInstances(nil)
	if err != nil {
		return nil, err
	}
	for _, inst := range resp.ReservedInstances {
		owner := defaultAccount
		key := populateReservedKey(inst)
		item := populateItemFromReserved(inst)
		itemRange := getReservedRange(inst)
		//instance start and end
		uptimeRange := getUptimeRange(itemRange, reportRange)
		if !isValidInstance(item, err, itemRange, uptimeRange) {
			break
		}

		item.setUptime(uptimeRange)
		item.setReservedPrice(inst)

		accounts.updateAccounts(owner, item, key)

	}
	return accounts, nil
}

// GetEC2Instances gets all EC2Instances and creates an array of accounts.
// Note this function is public but I may change that when adding non EC2 Amazon services.
func (c *Client) GetEC2Instances(reportRange TimeRange) (AccountHash, error) {
	// accounts maps from account name to the items
	accounts := make(AccountHash)
	grip.Notice("Getting EC2 Reserved Instances")
	accounts, err := c.getEC2ReservedInstances(accounts, reportRange)
	if err != nil {
		return nil, err
	}
	grip.Notice("Getting EC2 Spot Instances")
	accounts, err = c.getEC2SpotInstances(accounts, reportRange)
	if err != nil {
		return nil, err
	}

	return accounts, nil
}
