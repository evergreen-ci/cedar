package cost

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/evergreen-ci/cedar/util"
	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
)

const startTag = "start-time"

func init() {
	grip.SetName("cedar.amazon.test")
}

type AWSClientSuite struct {
	suite.Suite
	spot     *ec2.SpotInstanceRequest
	reserved *ec2.ReservedInstances
	ondemand *ec2.Instance
}

func TestAWSClientSuite(t *testing.T) {
	suite.Run(t, new(AWSClientSuite))
}

//
func (s *AWSClientSuite) SetupSuite() {
	s.spot = &ec2.SpotInstanceRequest{}
	s.spot.Status = &ec2.SpotInstanceStatus{}
	s.spot.LaunchSpecification = &ec2.LaunchSpecification{}
	s.reserved = &ec2.ReservedInstances{}
	s.ondemand = &ec2.Instance{}
	s.ondemand.State = &ec2.InstanceState{}
}

func (s *AWSClientSuite) TestGetTagVal() {
	key := "owner"
	owner := "me"
	tag := &ec2.Tag{Key: &key, Value: &owner}
	tags := []*ec2.Tag{tag}
	val, err := getTagVal(tags, "owner")
	s.NoError(err)
	s.Equal(val, "me")

	val, err = getTagVal(tags, "nothing")
	s.Error(err)
	s.Empty(val)

	_, err = getTagVal([]*ec2.Tag{}, "owner")
	s.Error(err)
}

func (s *AWSClientSuite) TestPopulateOnDemandKey() {
	instance := "c3.4xlarge"
	s.ondemand.InstanceType = &instance
	itemkey := populateOnDemandKey(s.ondemand)
	s.Equal(itemkey.ItemType, onDemand)
	s.Equal(itemkey.Name, instance)
}

func (s *AWSClientSuite) TestPopulateSpotKey() {
	instance := "c3.4xlarge"
	s.spot.LaunchSpecification.InstanceType = &instance
	itemkey := populateSpotKey(s.spot)
	s.Equal(itemkey.ItemType, spot)
	s.Equal(itemkey.Name, instance)
}

func (s *AWSClientSuite) TestPopulateReservedSpotKey() {
	instance := "c3.4xlarge"
	duration := int64(31536000)
	offeringType := ec2.OfferingTypeValuesAllUpfront
	s.reserved.InstanceType = &instance
	s.reserved.Duration = &duration
	s.reserved.OfferingType = &offeringType
	itemkey := populateReservedKey(s.reserved)
	s.Equal(itemkey.ItemType, reserved)
	s.Equal(itemkey.Name, instance)
	s.EqualValues(itemkey.Duration, duration)
	s.Equal(itemkey.OfferingType, ec2.OfferingTypeValuesAllUpfront)
}

func (s *AWSClientSuite) TestPopulateItemFromSpotActive() {
	state := "stopped"
	code := marked
	s.spot.State = &state
	s.spot.Status.Code = &code
	item := populateItemFromSpot(s.spot)
	s.False(item.Terminated)
	s.True(item.Launched)
	state = ec2.SpotInstanceStateActive
	s.spot.State = &state
	item = populateItemFromSpot(s.spot)
	s.False(item.Terminated)
	s.True(item.Launched)
}

func (s *AWSClientSuite) TestPopulateItemFromSpotNotActive() {
	state := ec2.SpotInstanceStateOpen
	s.spot.State = &state
	item := populateItemFromSpot(s.spot)
	s.Nil(item)
	state = ec2.SpotInstanceStateFailed
	s.spot.State = &state
	item = populateItemFromSpot(s.spot)
	s.Nil(item)

	state = ec2.SpotInstanceStateClosed
	s.spot.State = &state
	state = "terminated-by-user"
	s.spot.Status.Code = &state
	item = populateItemFromSpot(s.spot)
	s.True(item.Terminated)
	s.False(item.Launched)
}

func (s *AWSClientSuite) TestPopulateItemFromReserved() {
	state := ec2.ReservedInstanceStateRetired
	count := int64(12)
	s.reserved.State = &state
	s.reserved.InstanceCount = &count
	item := populateItemFromReserved(s.reserved)
	s.True(item.Terminated)
	s.False(item.Launched)
	s.Equal(item.Count, 12)
	state = ec2.ReservedInstanceStateActive
	s.reserved.State = &state
	item = populateItemFromReserved(s.reserved)
	s.False(item.Terminated)
	s.True(item.Launched)
	s.Equal(item.Count, 12)
	state = ec2.ReservationStatePaymentPending
	s.reserved.State = &state
	item = populateItemFromReserved(s.reserved)
	s.Nil(item)
}

func (s *AWSClientSuite) TestPopulateItemFromOnDemand() {
	state := ec2.InstanceStateNamePending
	s.ondemand.State.Name = &state
	item := populateItemFromOnDemand(s.ondemand)
	s.Nil(item)

	state = ec2.InstanceStateNameRunning
	s.ondemand.State.Name = &state
	item = populateItemFromOnDemand(s.ondemand)
	s.True(item.Launched)
	s.False(item.Terminated)

	state = ec2.InstanceStateNameTerminated
	s.ondemand.State.Name = &state
	item = populateItemFromOnDemand(s.ondemand)
	s.False(item.Launched)
	s.True(item.Terminated)
}

func (s *AWSClientSuite) TestGetSpotRangeTerminatedbyUser() {

	key := startTag
	val := "20170705164309"
	tag := ec2.Tag{
		Key:   &key,
		Value: &val,
	}
	s.spot.Tags = []*ec2.Tag{&tag}
	updateTime, _ := time.Parse(utcLayout, "2017-07-05T19:04:05.000Z")
	code := "terminated-by-user"
	state := ec2.SpotInstanceStateClosed
	s.spot.Status.UpdateTime = &updateTime
	s.spot.Status.Code = &code
	s.spot.State = &state
	times := getSpotRange(s.spot)
	tagTime, _ := time.Parse(tagLayout, "20170705164309")
	s.Equal(updateTime, times.EndAt)
	s.Equal(tagTime, times.StartAt)
}

func (s *AWSClientSuite) TestGetSpotRangeTerminatedByAmazon() {
	key := startTag
	val := "20170705164309"
	tag := ec2.Tag{
		Key:   &key,
		Value: &val,
	}
	s.spot.Tags = []*ec2.Tag{&tag}
	updateTime, _ := time.Parse(utcLayout, "2017-07-05T19:04:05.000Z")
	code := "instance-terminated-by-price"
	state := ec2.SpotInstanceStateClosed
	s.spot.Status.UpdateTime = &updateTime
	s.spot.Status.Code = &code
	s.spot.State = &state
	times := getSpotRange(s.spot)
	tagTime, _ := time.Parse(tagLayout, "20170705164309")
	s.NotEqual(updateTime, times.EndAt)
	s.Equal(updateTime.Add(-time.Hour), times.EndAt)
	s.Equal(tagTime, times.StartAt)

	// terminated by amazon, < 1 hour
	updateTime, _ = time.Parse(utcLayout, "2017-07-05T17:04:05.000Z")
	s.spot.Status.UpdateTime = &updateTime
	times = getSpotRange(s.spot)
	s.Equal(times, util.TimeRange{})
}

func (s *AWSClientSuite) TestGetReservedRange() {
	start, _ := time.Parse(utcLayout, "2016-03-18T00:00:00.000Z")
	end, _ := time.Parse(utcLayout, "2017-03-17T23:59:59.000Z")
	s.reserved.Start = &start
	s.reserved.End = &end
	times := getReservedRange(s.reserved)

	s.Equal(start, times.StartAt)
	s.Equal(end, times.EndAt)

	//error
	var zeroTime *time.Time
	s.reserved.Start = zeroTime
	times = getReservedRange(s.reserved)
	s.Equal(times, util.TimeRange{})
}

func (s *AWSClientSuite) TestGetOnDemandRangeNotTerminated() {
	s.T().Skip("stabalization pass")
	s.ondemand.LaunchTime = nil
	times := getOnDemandRange(s.ondemand)
	s.Equal(times, util.TimeRange{})

	start, _ := time.Parse(utcLayout, "2016-03-18T00:00:00.000Z")
	s.ondemand.LaunchTime = &start
	state := ec2.InstanceStateNameRunning
	s.ondemand.State.Name = &state
	times = getOnDemandRange(s.ondemand)
	s.Equal(times.StartAt, start)
	s.True(times.EndAt.IsZero())
}

func (s *AWSClientSuite) TestGetOnDemandRangeTerminated() {
	start, _ := time.Parse(utcLayout, "2016-03-18T00:00:00.000Z")
	s.ondemand.LaunchTime = &start
	reason := "User initiated (2016-04-18 21:15:09 GMT)"
	end, _ := time.Parse(ondemandLayout, "2016-04-18 21:15:09 GMT")
	state := ec2.InstanceStateNameStopped
	s.ondemand.State.Name = &state
	s.ondemand.StateTransitionReason = &reason
	times := getOnDemandRange(s.ondemand)
	s.Equal(times.StartAt, start)
	s.Equal(times.EndAt, end)

	reason = "Service initiated (2016-04-18 21:15:09 GMT)"
	s.ondemand.StateTransitionReason = &reason
	times = getOnDemandRange(s.ondemand)
	s.Equal(times.StartAt, start)
	s.Equal(times.EndAt, end)

	reason = "InternalError"
	s.ondemand.StateTransitionReason = &reason
	times = getOnDemandRange(s.ondemand)
	s.Equal(times, util.TimeRange{})
}

func (s *AWSClientSuite) TestGetUptimeRangeWhenHalfInReport() {
	//report start and end are both before their respective tag values
	repStart, _ := time.Parse(tagLayout, "20170705144309")
	repEnd, _ := time.Parse(tagLayout, "20170705175600")
	repRange := util.TimeRange{
		StartAt: repStart,
		EndAt:   repEnd,
	}
	itemStart, _ := time.Parse(tagLayout, "20170705164309")
	itemEnd, _ := time.Parse(tagLayout, "20170705183800")
	itemRange := util.TimeRange{
		StartAt: itemStart,
		EndAt:   itemEnd,
	}

	times := getUptimeRange(itemRange, repRange)
	s.Equal(times.StartAt, itemStart)
	s.Equal(times.EndAt, repEnd)

	//report start and end are both after their respective tags
	repRange.StartAt, _ = time.Parse(tagLayout, "20170705165500")
	repRange.EndAt, _ = time.Parse(tagLayout, "20170705195800")
	times = getUptimeRange(itemRange, repRange)
	s.Equal(times.StartAt, repRange.StartAt)
	s.Equal(times.EndAt, itemRange.EndAt)
}

func (s *AWSClientSuite) TestGetUptimeRangeWhenLongerThanReport() {
	//report start and end are both between the tags
	repStart, _ := time.Parse(tagLayout, "20170705165500")
	repEnd, _ := time.Parse(tagLayout, "20170705175800")
	repRange := util.TimeRange{
		StartAt: repStart,
		EndAt:   repEnd,
	}
	itemStart, _ := time.Parse(tagLayout, "20170705164309")
	itemEnd, _ := time.Parse(tagLayout, "20170705183800")
	itemRange := util.TimeRange{
		StartAt: itemStart,
		EndAt:   itemEnd,
	}
	uptimeRange := getUptimeRange(itemRange, repRange)
	s.Equal(uptimeRange.StartAt, repStart)
	s.Equal(uptimeRange.EndAt, repEnd)
}

func (s *AWSClientSuite) TestGetUptimeRangeWhenUnterminated() {
	// unterminated instance, report start is before tagged start
	itemStart, _ := time.Parse(tagLayout, "20170705164309")
	itemEnd, _ := time.Parse(tagLayout, "")
	itemRange := util.TimeRange{
		StartAt: itemStart,
		EndAt:   itemEnd,
	}
	repStart, _ := time.Parse(tagLayout, "20170705155500")
	repEnd, _ := time.Parse(tagLayout, "20170705193800")
	repRange := util.TimeRange{
		StartAt: repStart,
		EndAt:   repEnd,
	}
	uptimeRange := getUptimeRange(itemRange, repRange)
	s.Equal(uptimeRange.StartAt, itemStart)
	s.Equal(uptimeRange.EndAt, repEnd)
}

func (s *AWSClientSuite) TestGetUptimeInstanceOutsideReport() {
	//instance ends before report starts
	itemStart, _ := time.Parse(tagLayout, "20170703164309")
	itemEnd, _ := time.Parse(tagLayout, "20170704155500")
	itemRange := util.TimeRange{
		StartAt: itemStart,
		EndAt:   itemEnd,
	}
	repStart, _ := time.Parse(tagLayout, "20170705155500")
	repEnd, _ := time.Parse(tagLayout, "20170705193800")
	repRange := util.TimeRange{
		StartAt: repStart,
		EndAt:   repEnd,
	}
	uptimeRange := getUptimeRange(itemRange, repRange)
	s.Equal(uptimeRange, util.TimeRange{})

	//instance starts after report ends
	itemStart, _ = time.Parse(tagLayout, "20170706164309")
	itemEnd, _ = time.Parse(tagLayout, "20170706185500")
	itemRange = util.TimeRange{
		StartAt: itemStart,
		EndAt:   itemEnd,
	}
	uptimeRange = getUptimeRange(itemRange, repRange)
	s.Equal(uptimeRange, util.TimeRange{})

	//empty item
	uptimeRange = getUptimeRange(util.TimeRange{}, repRange)
	s.Equal(uptimeRange, util.TimeRange{})
}

func (s *AWSClientSuite) TestSetUptime() {
	start, _ := time.Parse(tagLayout, "20170706164309")
	end, _ := time.Parse(tagLayout, "20170706185500")
	times := util.TimeRange{
		StartAt: start,
		EndAt:   end,
	}
	item := &AWSItem{}
	item.setUptime(times)
	s.Equal(item.Uptime, 3)
}

func (s *AWSClientSuite) TestSetReservedPriceFixed() {
	offeringType := ec2.OfferingTypeValuesAllUpfront
	price := float64(120)
	s.reserved.OfferingType = &offeringType
	s.reserved.FixedPrice = &price
	item := &AWSItem{}
	item.setReservedPrice(s.reserved)
	s.Equal(item.FixedPrice, 120.0)
	s.Equal(item.Price, 0.0)
}

func (s *AWSClientSuite) TestSetReservedPriceNoUpfront() {
	offeringType := ec2.OfferingTypeValuesNoUpfront
	s.reserved.OfferingType = &offeringType
	amount := 0.84
	charge := &ec2.RecurringCharge{
		Amount: &amount,
	}
	s.reserved.RecurringCharges = []*ec2.RecurringCharge{charge}
	item := &AWSItem{
		Uptime: 3,
	}
	item.setReservedPrice(s.reserved)
	s.Equal(item.FixedPrice, 0.0)
	s.Equal(item.Price, 0.84*3)
}

func (s *AWSClientSuite) TestSetReservedPricePartial() {
	offeringType := ec2.OfferingTypeValuesPartialUpfront
	price := float64(120)
	amount := 0.84
	s.reserved.OfferingType = &offeringType
	s.reserved.FixedPrice = &price
	item := &AWSItem{
		Uptime: 3,
	}
	charge := &ec2.RecurringCharge{
		Amount: &amount,
	}
	s.reserved.RecurringCharges = []*ec2.RecurringCharge{charge}
	item.setReservedPrice(s.reserved)
	s.Equal(item.Price, 0.84*3)
	s.Equal(item.FixedPrice, 120.0)
}

func (s *AWSClientSuite) TestSetOnDemandPrice() {
	info := odInfo{
		os:       "Windows",
		instance: "c3.4xlarge",
		region:   "US East (N. Virginia)",
	}
	price := 1.2
	pricing := &prices{}
	(*pricing)[info] = price

	item := &AWSItem{Uptime: 4}
	s.ondemand.Placement = nil
	item.setOnDemandPrice(s.ondemand, pricing)
	s.Equal(item.Price, 0.0)

	zone := "us-east-1b"
	instanceType := "c3.4xlarge"
	os := "windows"
	s.ondemand.Placement = &ec2.Placement{AvailabilityZone: &zone}
	s.ondemand.InstanceType = &instanceType
	s.ondemand.Platform = &os

	item.setOnDemandPrice(s.ondemand, pricing)
	s.Equal(item.Price, 4.8)
}

func (s *AWSClientSuite) TestIsValidInstance() {
	item := &AWSItem{Price: 0.48}
	time1, _ := time.Parse(utcLayout, "2017-07-05T19:04:05.000Z")
	time2, _ := time.Parse(utcLayout, "2017-07-08T20:04:05.000Z")
	range1 := util.TimeRange{
		StartAt: time1,
		EndAt:   time2,
	}
	range2 := util.TimeRange{
		StartAt: time1,
		EndAt:   time2,
	}
	res := isValidInstance(item, nil, range1, range2)
	s.True(res)
	res = isValidInstance(nil, nil, range1, range2)
	s.False(res)
	res = isValidInstance(item, nil, util.TimeRange{}, range2)
	s.False(res)
}
