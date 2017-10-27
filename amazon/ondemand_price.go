package amazon

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
)

const (
	linuxOS       = "Linux"
	windowsOS     = "Windows"
	east1         = "us-east-1"
	west1         = "us-west-1"
	west2         = "us-west-2"
	east1Region   = "US East (N. Virginia)"
	west1Region   = "US West (N. California)"
	west2Region   = "US West (Oregon)"
	priceEndpoint = "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/index.json"
)

//NOTE: most of this code is taken from evergreen-ci/evergreen/cloud/providers/ec2/ec2_util.go

// Terms is an internal type for loading price API results into.
type terms struct {
	OnDemand map[string]map[string]struct {
		PriceDimensions map[string]struct {
			PricePerUnit struct {
				USD string
			}
		}
	}
}

// Product is an internal type for loading product API results into.
type product struct {
	SKU           string
	ProductFamily string
	Attributes    struct {
		Location        string
		InstanceType    string
		PreInstalledSW  string
		OperatingSystem string
		Tenancy         string
		LicenseModel    string
	}
}

// priceInfo is an internal type that holds price and product API results
type priceInfo struct {
	Terms    terms
	Products map[string]product
}

// odInfo is an internal type for keying hosts by the attributes that affect billing.
type odInfo struct {
	os       string
	instance string
	region   string
}

type prices map[odInfo]float64

// skuPrice digs through the incredibly verbose Amazon price data format
// for the USD dollar amount of an SKU. The for loops are for traversing
// maps of size one with an unknown key, which is simple to do in a
// language like python but really ugly here.
func (t terms) skuPrice(sku string) float64 {
	for _, v := range t.OnDemand[sku] {
		for _, p := range v.PriceDimensions {
			// parse -- ignoring errors
			val, _ := strconv.ParseFloat(p.PricePerUnit.USD, 64)
			return val
		}
	}
	return 0
}

// osBillingName returns the os name in the same format as the API.
func osBillingName(pd string) string {
	if strings.Contains(strings.ToLower(pd), "windows") {
		return windowsOS
	}
	return linuxOS
}

// regionFullname takes the API ID of amazon region and returns the
// full region name. For instance, "us-west-1" becomes "US West (N. California)".
// This is necessary as the On Demand pricing endpoint uses the full name, unlike
// the rest of the API. THIS FUNCTION ONLY HANDLES U.S. REGIONS.
func regionFullName(region string) (string, error) {
	switch region {
	case east1:
		return east1Region, nil
	case west1:
		return west1Region, nil
	case west2:
		return west2Region, nil
	}
	return "", errors.Errorf("region %v not supported for On Demand cost calculation", region)
}

// azToRegion takes an availability zone and returns the region id.
func azToRegion(az string) string {
	// an amazon region is just the availability zone minus the final letter
	return az[:len(az)-1]
}

// isValid return true if the product receiver meets the restrictions previously
// defined in evergreen-ci/evergreen.
func (p *product) isValid() bool {
	return (p.ProductFamily == "Compute Instance" &&
		p.Attributes.PreInstalledSW == "NA" &&
		p.Attributes.Tenancy == "Shared" &&
		p.Attributes.LicenseModel != "Bring your own license")
}

// getOnDemandPriceInformation gets json from the ec2 on-demand price endpoint,
// iterates through and populates the prices struct,
// which is returned.
func getOnDemandPriceInformation() (*prices, error) {
	grip.Debugln("Loading On Demand pricing from", priceEndpoint)
	resp, err := http.Get(priceEndpoint)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("No response body")
	}
	defer resp.Body.Close()
	details := priceInfo{}
	pricing := &prices{}
	err = json.NewDecoder(resp.Body).Decode(&details)
	if err != nil {
		return nil, errors.Wrap(err, "Problem parsing response body")
	}
	for _, p := range details.Products {
		if p.isValid() {
			// the product description does not include pricing information,
			// so we must look up the SKU in the "Terms" section.
			price := details.Terms.skuPrice(p.SKU)
			info := odInfo{
				os:       p.Attributes.OperatingSystem,
				instance: p.Attributes.InstanceType,
				region:   p.Attributes.Location,
			}
			(*pricing)[info] = price
		}
	}
	return pricing, nil
}

// fetchPrice uses the given instance information to retrieve its price from the prices receiver.
//If the information is invalid or the price doesn't exist, we return 0.
func (pricing *prices) fetchPrice(productDesc string, instanceType string, availZone string) float64 {
	region, err := regionFullName(azToRegion(availZone))
	if err != nil {
		return 0.0
	}
	info := odInfo{
		os:       osBillingName(productDesc),
		instance: instanceType,
		region:   region,
	}
	return (*pricing)[info]
}
