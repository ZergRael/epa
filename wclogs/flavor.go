package wclogs

// Flavor represents WoW release
type Flavor int

const (
	Retail Flavor = iota
	Classic
	Vanilla
)

// String returns printable Flavor
func (f Flavor) String() string {
	return [...]string{"Retail", "Classic", "Vanilla"}[f]
}

// LatestExpansion returns latest expansion ID for a Flavor
// TODO: This really shouldn't be hardcoded
func (f Flavor) LatestExpansion() int {
	return [...]int{4, 1001, 2000}[f]
}
