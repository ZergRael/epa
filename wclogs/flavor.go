package wclogs

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

// LatestExpansion returns latest expansion ID for a Flavor
// TODO: This really shouldn't be hardcoded
func (f Flavor) LatestExpansion() int {
	return [...]int{4, 1000, 1001, 1002, 2000}[f]
}
