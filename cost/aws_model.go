package cost

import (
	"github.com/pkg/errors"
)

// AWSItem is information for an item for a particular Name and AWSItemType
type AWSItem struct {
	Product    string
	Launched   bool
	Terminated bool
	Price      float64
	FixedPrice float64
	Uptime     int //stored in number of hours
	Count      int
}

// AWSItemKey is used together with AWSItem to create a hashtable from AWSItemKey to []AWSItem
type AWSItemKey struct {
	Service      string
	Name         string
	ItemType     string
	Duration     int64
	OfferingType string
}

type AWSServices struct {
	m map[AWSItemKey][]AWSItem
}

func NewAWSServices() *AWSServices {
	return &AWSServices{
		m: make(map[AWSItemKey][]AWSItem),
	}
}

func (s *AWSServices) Len() int                              { return len(s.m) }
func (s *AWSServices) Get(i AWSItemKey) ([]AWSItem, bool)    { out, ok := s.m[i]; return out, ok }
func (s *AWSServices) Put(k AWSItemKey, v []AWSItem)         { s.m[k] = v }
func (s *AWSServices) Append(k AWSItemKey, items ...AWSItem) { s.Extend(k, items) }
func (s *AWSServices) Extend(k AWSItemKey, items []AWSItem) {
	if v, ok := s.m[k]; ok {
		v = append(v, items...)
		s.m[k] = v
		return
	}

	s.m[k] = items
}

func (s *AWSServices) PutSafe(k AWSItemKey, v []AWSItem) error {
	if _, ok := s.m[k]; ok {
		return errors.Errorf("an item with the key '%+v' exists, not overwriting", k)
	}

	s.m[k] = v

	return nil
}

type AWSItemPair struct {
	Key   AWSItemKey
	Value []AWSItem
}

func (s *AWSServices) Iter() <-chan AWSItemPair {
	out := make(chan AWSItemPair)
	go func() {
		for k, v := range s.m {
			out <- AWSItemPair{
				Key:   k,
				Value: v,
			}
		}
		close(out)
	}()
	return out
}
