package amazon

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/mongodb/grip"
	"github.com/stretchr/testify/suite"
)

func init() {
	grip.SetName("sink.amazon.test")

}

type ClientSuite struct {
	suite.Suite
	spot     *ec2.SpotInstanceRequest
	reserved *ec2.ReservedInstances
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

//
func (s *ClientSuite) SetupSuite() {
	s.spot = &ec2.SpotInstanceRequest{}
	s.spot.Status = &ec2.SpotInstanceStatus{}
	s.spot.LaunchSpecification = &ec2.LaunchSpecification{}
	s.reserved = &ec2.ReservedInstances{}
}

func (s *ClientSuite) TestGetTagVal() {
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

func (s *ClientSuite) TestPopulateSpotKey() {
	instance := "c3.4xlarge"
	s.spot.LaunchSpecification.InstanceType = &instance
	itemkey := populateSpotKey(s.spot)
	s.Equal(itemkey.ItemType, spot)
	s.Equal(itemkey.Name, "c3.4xlarge")
}

func (s *ClientSuite) TestReservedSpotKey() {
	instance := "c3.4xlarge"
	duration := int64(31536000)
	offeringType := string(fixed)
	s.reserved.InstanceType = &instance
	s.reserved.Duration = &duration
	s.reserved.OfferingType = &offeringType
	itemkey := populateReservedKey(s.reserved)
	s.Equal(itemkey.ItemType, reserved)
	s.Equal(itemkey.Name, "c3.4xlarge")
	s.EqualValues(itemkey.duration, 31536000)
	s.Equal(itemkey.offeringType, fixed)
}

func (s *ClientSuite) TestPopulateItemFromSpotActive() {
	state := "stopped"
	code := marked
	s.spot.State = &state
	s.spot.Status.Code = &code
	item := populateItemFromSpot(s.spot)
	s.False(item.Terminated)
	s.True(item.Launched)
	state = active
	s.spot.State = &state
	item = populateItemFromSpot(s.spot)
	s.False(item.Terminated)
	s.True(item.Launched)
}

func (s *ClientSuite) TestPopulateItemFromSpotNotActive() {
	state := open
	s.spot.State = &state
	item := populateItemFromSpot(s.spot)
	s.Nil(item)
	state = failed
	s.spot.State = &state
	item = populateItemFromSpot(s.spot)
	s.Nil(item)

	state = "terminated"
	s.spot.State = &state
	state = "terminated-by-user"
	s.spot.Status.Code = &state
	item = populateItemFromSpot(s.spot)
	s.True(item.Terminated)
	s.False(item.Launched)
}

func (s *ClientSuite) TestPopulateItemFromReserved() {
	state := retired
	count := int64(12)
	s.reserved.State = &state
	s.reserved.InstanceCount = &count
	item := populateItemFromReserved(s.reserved)
	s.True(item.Terminated)
	s.False(item.Launched)
	s.Equal(item.Count, 12)
	state = active
	s.reserved.State = &state
	item = populateItemFromReserved(s.reserved)
	s.False(item.Terminated)
	s.True(item.Launched)
	s.Equal(item.Count, 12)
	state = "payment-pending"
	s.reserved.State = &state
	item = populateItemFromReserved(s.reserved)
	s.Nil(item)
}

func (s *ClientSuite) TestGetSpotRangeTerminatedbyUser() {
	key := "start-time"
	val := "20170705164309"
	tag := ec2.Tag{
		Key:   &key,
		Value: &val,
	}
	s.spot.Tags = []*ec2.Tag{&tag}
	updateTime, _ := time.Parse(utcLayout, "2017-07-05T19:04:05.000Z")
	code := "terminated-by-user"
	state := "terminated"
	s.spot.Status.UpdateTime = &updateTime
	s.spot.Status.Code = &code
	s.spot.State = &state
	times := getSpotRange(s.spot)
	tagTime, _ := time.Parse(tagLayout, "20170705164309")
	s.Equal(updateTime, times.End)
	s.Equal(tagTime, times.Start)
}

func (s *ClientSuite) TestGetSpotRangeTerminatedByAmazon() {
	key := "start-time"
	val := "20170705164309"
	tag := ec2.Tag{
		Key:   &key,
		Value: &val,
	}
	s.spot.Tags = []*ec2.Tag{&tag}
	updateTime, _ := time.Parse(utcLayout, "2017-07-05T19:04:05.000Z")
	code := "instance-terminated-by-price"
	state := "terminated"
	s.spot.Status.UpdateTime = &updateTime
	s.spot.Status.Code = &code
	s.spot.State = &state
	times := getSpotRange(s.spot)
	tagTime, _ := time.Parse(tagLayout, "20170705164309")
	fmt.Println(times.End.String())
	s.NotEqual(updateTime, times.End)
	s.Equal(updateTime.Add(-time.Hour), times.End)
	s.Equal(tagTime, times.Start)

	// terminated by amazon, < 1 hour
	updateTime, _ = time.Parse(utcLayout, "2017-07-05T17:04:05.000Z")
	s.spot.Status.UpdateTime = &updateTime
	times = getSpotRange(s.spot)
	s.Equal(times, TimeRange{})
}

func (s *ClientSuite) TestGetReservedRange() {
	start, _ := time.Parse(utcLayout, "2016-03-18T00:00:00.000Z")
	end, _ := time.Parse(utcLayout, "2017-03-17T23:59:59.000Z")
	s.reserved.Start = &start
	s.reserved.End = &end
	times := getReservedRange(s.reserved)

	s.Equal(start, times.Start)
	s.Equal(end, times.End)

	//error
	var zeroTime *time.Time
	s.reserved.Start = zeroTime
	times = getReservedRange(s.reserved)
	s.Equal(times, TimeRange{})
}

func (s *ClientSuite) TestGetUptimeRangeWhenHalfInReport() {
	//report start and end are both before their respective tag values
	repStart, _ := time.Parse(tagLayout, "20170705144309")
	repEnd, _ := time.Parse(tagLayout, "20170705175600")
	repRange := TimeRange{
		Start: repStart,
		End:   repEnd,
	}
	itemStart, _ := time.Parse(tagLayout, "20170705164309")
	itemEnd, _ := time.Parse(tagLayout, "20170705183800")
	itemRange := TimeRange{
		Start: itemStart,
		End:   itemEnd,
	}

	times := getUptimeRange(itemRange, repRange)
	s.Equal(times.Start, itemStart)
	s.Equal(times.End, repEnd)

	//report start and end are both after their respective tags
	repRange.Start, _ = time.Parse(tagLayout, "20170705165500")
	repRange.End, _ = time.Parse(tagLayout, "20170705195800")
	times = getUptimeRange(itemRange, repRange)
	s.Equal(times.Start, repRange.Start)
	s.Equal(times.End, itemRange.End)
}

func (s *ClientSuite) TestGetUptimeRangeWhenLongerThanReport() {
	//report start and end are both between the tags
	repStart, _ := time.Parse(tagLayout, "20170705165500")
	repEnd, _ := time.Parse(tagLayout, "20170705175800")
	repRange := TimeRange{
		Start: repStart,
		End:   repEnd,
	}
	itemStart, _ := time.Parse(tagLayout, "20170705164309")
	itemEnd, _ := time.Parse(tagLayout, "20170705183800")
	itemRange := TimeRange{
		Start: itemStart,
		End:   itemEnd,
	}
	uptimeRange := getUptimeRange(itemRange, repRange)
	s.Equal(uptimeRange.Start, repStart)
	s.Equal(uptimeRange.End, repEnd)
}

func (s *ClientSuite) TestGetUptimeRangeWhenUnterminated() {
	// unterminated instance, report start is before tagged start
	itemStart, _ := time.Parse(tagLayout, "20170705164309")
	itemEnd, _ := time.Parse(tagLayout, "")
	itemRange := TimeRange{
		Start: itemStart,
		End:   itemEnd,
	}
	repStart, _ := time.Parse(tagLayout, "20170705155500")
	repEnd, _ := time.Parse(tagLayout, "20170705193800")
	repRange := TimeRange{
		Start: repStart,
		End:   repEnd,
	}
	uptimeRange := getUptimeRange(itemRange, repRange)
	s.Equal(uptimeRange.Start, itemStart)
	s.Equal(uptimeRange.End, repEnd)
}

func (s *ClientSuite) TestGetUptimeInstanceOutsideReport() {
	//instance ends before report starts
	itemStart, _ := time.Parse(tagLayout, "20170703164309")
	itemEnd, _ := time.Parse(tagLayout, "20170704155500")
	itemRange := TimeRange{
		Start: itemStart,
		End:   itemEnd,
	}
	repStart, _ := time.Parse(tagLayout, "20170705155500")
	repEnd, _ := time.Parse(tagLayout, "20170705193800")
	repRange := TimeRange{
		Start: repStart,
		End:   repEnd,
	}
	uptimeRange := getUptimeRange(itemRange, repRange)
	s.Equal(uptimeRange, TimeRange{})

	//instance starts after report ends
	itemStart, _ = time.Parse(tagLayout, "20170706164309")
	itemEnd, _ = time.Parse(tagLayout, "20170706185500")
	itemRange = TimeRange{
		Start: itemStart,
		End:   itemEnd,
	}
	uptimeRange = getUptimeRange(itemRange, repRange)
	s.Equal(uptimeRange, TimeRange{})

	//empty item
	uptimeRange = getUptimeRange(TimeRange{}, repRange)
	s.Equal(uptimeRange, TimeRange{})
}

func (s *ClientSuite) TestSetUptime() {
	start, _ := time.Parse(tagLayout, "20170706164309")
	end, _ := time.Parse(tagLayout, "20170706185500")
	times := TimeRange{
		Start: start,
		End:   end,
	}
	item := &EC2Item{}
	item.setUptime(times)
	s.Equal(item.Uptime, 3)
}

func (s *ClientSuite) TestSetReservedPriceFixed() {
	offeringType := string(fixed)
	price := float64(120)
	s.reserved.OfferingType = &offeringType
	s.reserved.FixedPrice = &price
	item := &EC2Item{}
	item.setReservedPrice(s.reserved)
	s.Equal(item.FixedPrice, 120.0)
	s.Equal(item.Price, 0.0)
}

func (s *ClientSuite) TestSetReservedPriceNoUpfront() {
	offeringType := string(hourly)
	s.reserved.OfferingType = &offeringType
	amount := 0.84
	charge := &ec2.RecurringCharge{
		Amount: &amount,
	}
	s.reserved.RecurringCharges = []*ec2.RecurringCharge{charge}
	item := &EC2Item{
		Uptime: 3,
	}
	item.setReservedPrice(s.reserved)
	s.Equal(item.FixedPrice, 0.0)
	s.Equal(item.Price, 0.84*3)
}

func (s *ClientSuite) TestSetReservedPricePartial() {
	offeringType := string(partial)
	price := float64(120)
	amount := 0.84
	s.reserved.OfferingType = &offeringType
	s.reserved.FixedPrice = &price
	item := &EC2Item{
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

func (s *ClientSuite) TestIsValidInstance() {
	item := &EC2Item{Price: 0.48}
	time1, _ := time.Parse(utcLayout, "2017-07-05T19:04:05.000Z")
	time2, _ := time.Parse(utcLayout, "2017-07-08T20:04:05.000Z")
	range1 := TimeRange{
		Start: time1,
		End:   time2,
	}
	range2 := TimeRange{
		Start: time1,
		End:   time2,
	}
	res := isValidInstance(item, nil, range1, range2)
	s.True(res)
	res = isValidInstance(nil, nil, range1, range2)
	s.False(res)
	res = isValidInstance(item, nil, TimeRange{}, range2)
	s.False(res)
}
