package wclogs

const (
	retailApiUri  = "https://www.warcraftlogs.com/api/v2/client"
	classicApiUri = "https://classic.warcraftlogs.com/api/v2/client"
	vanillaApiUri = "https://vanilla.warcraftlogs.com/api/v2/client"
)

// Flavor represents WoW release
type Flavor int

const (
	Retail Flavor = iota
	Classic
	TBC
	WOTLK
	Vanilla
)

// String returns printable Flavor
func (f Flavor) String() string {
	return [...]string{"Retail", "Classic", "TBC", "WOTLK", "Vanilla"}[f]
}

func (f Flavor) Uri() string {
	uri := retailApiUri
	switch f {
	case Classic:
		fallthrough
	case TBC:
		fallthrough
	case WOTLK:
		uri = classicApiUri
	case Vanilla:
		uri = vanillaApiUri
	}

	return uri
}

// Expansion returns the current expansion ID for a Flavor
// TODO: This really shouldn't be hardcoded
func (f Flavor) Expansion() int {
	return [...]int{4, 1000, 1001, 1002, 2000}[f]
}
