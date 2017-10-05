package model

import (
	"github.com/mongodb/anser/bsonutil"
)

type CostConfigAmazon struct {
	Accounts  []string            `bson:"accounts" json:"accounts" yaml:"accounts"`
	S3Info    CostConfigAmazonS3  `bson:"s3" json:"s3" yaml:"s3"`
	EBSPrices CostConfigAmazonEBS `bson:"ebs_pricing" json:"ebs_pricing" yaml:"ebs_pricing"`
}

var (
	costConfigAmazonAccountsKey  = bsonutil.MustHaveTag(CostConfigAmazon{}, "Accounts")
	costConfigAmazonS3InfoKey    = bsonutil.MustHaveTag(CostConfigAmazon{}, "S3Info")
	costConfigAmazonEBSPricesKey = bsonutil.MustHaveTag(CostConfigAmazon{}, "EBSPrices")
)

type CostConfigAmazonS3 struct {
	KeyStart string `bson:"s3key_start" json:"s3key_start" yaml:"s3key_start"`
	Bucket   string `bson:"s3bucket" json:"s3bucket" yaml:"s3bucket"`
	Owner    string `bson:"owner" json:"owner" yaml:"owner"`
}

var (
	costConfigAmazonS3StartKey  = bsonutil.MustHaveTag(CostConfigAmazonS3{}, "KeyStart")
	costConfigAmazonS3BucketKey = bsonutil.MustHaveTag(CostConfigAmazonS3{}, "Bucket")
	costConfigAmazonS3OwnerKey  = bsonutil.MustHaveTag(CostConfigAmazonS3{}, "Owner")
)

// IsValid checks that a bucket name and key start are given in the S3Info struct.
func (s *CostConfigAmazonS3) IsValid() bool {
	if s == nil {
		return false
	}
	if s.Bucket == "" || s.KeyStart == "" {
		return false
	}
	return true
}

type CostConfigAmazonEBS struct {
	GP2      float64 `bson:"gp2" json:"gp2" yaml:"gp2"`
	IO1      float64 `bson:"io1GB" json:"io1GB" yaml:"io1GB"`
	IO1IOPS  float64 `bson:"io1IOPS" json:"io1IOPS" yaml:"io1IOPS"`
	ST1      float64 `bson:"st1" json:"st1" yaml:"st1"`
	SC1      float64 `bson:"sc1" json:"sc1" yaml:"sc1"`
	Standard float64 `bson:"standard" json:"standard" yaml:"standard"`
	Snapshot float64 `bson:"snapshot" json:"snapshot" yaml:"snapshot"`
}

var (
	costConfigAwsEbsGP2Key   = bsonutil.MustHaveTag(CostConfigAmazonEBS{}, "GP2")
	costConfigAwsEbsIO1      = bsonutil.MustHaveTag(CostConfigAmazonEBS{}, "IO1")
	costConfigAwsEbsIO1IOPS  = bsonutil.MustHaveTag(CostConfigAmazonEBS{}, "IO1IOPS")
	costConfigAwsEbsST1      = bsonutil.MustHaveTag(CostConfigAmazonEBS{}, "ST1")
	costConfigAwsEbsSC1      = bsonutil.MustHaveTag(CostConfigAmazonEBS{}, "SC1")
	costConfigAwsEbsStandard = bsonutil.MustHaveTag(CostConfigAmazonEBS{}, "Standard")
	costConfigAwsEbsSnapshot = bsonutil.MustHaveTag(CostConfigAmazonEBS{}, "Snapshot")
)

func (p *CostConfigAmazonEBS) IsValid() bool {
	for _, val := range []float64{p.GP2, p.IO1, p.IO1IOPS, p.ST1, p.SC1, p.Standard, p.Snapshot} {
		if val != 0.0 {
			return true
		}
	}

	return false
}
