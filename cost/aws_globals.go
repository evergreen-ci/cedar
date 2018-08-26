package cost

import "github.com/aws/aws-sdk-go/service/ec2"

const (
	// layouts use reference Mon Jan 2 15:04:05 -0700 MST 2006
	ec2Service = "ec2"
	ebsService = "ebs"
	// s3Service      = "s3"
	tagLayout      = "20060102150405"
	utcLayout      = "2006-01-02T15:04:05.000Z"
	ondemandLayout = "2006-01-02 15:04:05 MST"

	spot     = ec2.InstanceLifecycleTypeSpot
	reserved = "reserved"
	onDemand = "on-demand"
	marked   = "marked-for-termination"
)

var (
	ignoreCodes = []string{
		"canceled-before-fulfillment",
		"schedule-expired",
		"bad-parameters",
		"system-error",
	}
	amazonTerminated = []string{
		"instance-terminated-by-price",
		"instance-terminated-no-capacity",
		"instance-terminated-capacity-oversubscribed",
		"instance-terminated-launch-group-constraint",
	}
)
a
