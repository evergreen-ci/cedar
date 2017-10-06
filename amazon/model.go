package amazon

import (
	"github.com/pkg/errors"
)

// Item is information for an item for a particular Name and ItemType
type Item struct {
	Product    string
	Launched   bool
	Terminated bool
	Price      float64
	FixedPrice float64
	Uptime     int //stored in number of hours
	Count      int
}

// ItemKey is used together with Item to create a hashtable from ItemKey to []Item
type ItemKey struct {
	Service      string
	Name         string
	ItemType     string
	Duration     int64
	OfferingType string
}

type Services struct {
	m map[ItemKey][]Item
}

func NewServices() *Services {
	return &Services{
		m: make(map[ItemKey][]Item),
	}
}

func (s *Services) Len() int                        { return len(s.m) }
func (s *Services) Get(i ItemKey) ([]Item, bool)    { out, ok := s.m[i]; return out, ok }
func (s *Services) Put(k ItemKey, v []Item)         { s.m[k] = v }
func (s *Services) Append(k ItemKey, items ...Item) { s.Extend(k, items) }
func (s *Services) Extend(k ItemKey, items []Item) {
	if v, ok := s.m[k]; ok {
		v = append(v, items...)
		s.m[k] = v
		return
	}

	s.m[k] = items
}

func (s *Services) PutSafe(k ItemKey, v []Item) error {
	if _, ok := s.m[k]; ok {
		return errors.Errorf("an item with the key '%+v' exists, not overwriting", k)
	}

	s.m[k] = v

	return nil
}

type ItemPair struct {
	Key   ItemKey
	Value []Item
}

func (s *Services) Iter() <-chan ItemPair {
	out := make(chan ItemPair)
	go func() {
		for k, v := range s.m {
			out <- ItemPair{
				Key:   k,
				Value: v,
			}
		}
		close(out)
	}()
	return out
}
