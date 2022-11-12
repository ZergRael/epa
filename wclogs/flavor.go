package wclogs

import (
	"context"
	"github.com/machinebox/graphql"
)

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
	Vanilla
)

// String returns printable Flavor
func (f Flavor) String() string {
	return [...]string{"Retail", "Classic", "Vanilla"}[f]
}

func (f Flavor) Uri() string {
	uri := retailApiUri
	switch f {
	case Classic:
		uri = classicApiUri
	case Vanilla:
		uri = vanillaApiUri
	}

	return uri
}

// Expansion returns the current expansion ID for a Flavor
// TODO: This really shouldn't be hardcoded
func (f Flavor) Expansion() int {
	return [...]int{4, 1002, 2000}[f]
}

// getLatestExpansion the latest expansion ID for a specific Flavor
func (w *WCLogs) getLatestExpansion() (int, error) {
	req := graphql.NewRequest(`
    query {
		worldData {
			expansions {
				id
				name
			}
		}
    }
`)

	var resp struct {
		WorldData struct {
			Expansions []struct {
				ID   int
				Name string
			}
		}
	}

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		return 0, err
	}

	return resp.WorldData.Expansions[0].ID, nil
}
