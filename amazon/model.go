package amazon

type itemType string
type serviceType string

// Maps the ItemKey to an array of Items
type itemHash map[ItemKey][]*Item

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
	Service      serviceType
	Name         string
	ItemType     itemType
	offeringType string
	duration     int64
}
