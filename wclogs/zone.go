package wclogs

import (
	"context"
	"github.com/machinebox/graphql"
)

// Zone represents a WoW zone
type Zone struct {
	ID   int
	Name string
}

// getZones queries a collection of Zone, this is static data for each expansion
func (w *WCLogs) getZones() ([]Zone, error) {
	req := graphql.NewRequest(`
    query ($expansion: Int!) {
		worldData {
			zones (expansion_id: $expansion) {
				id
				name
			}
		}
    }
`)
	req.Var("expansion", w.flavor.Expansion())

	var resp struct {
		WorldData struct {
			Zones []Zone
		}
	}

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		return nil, err
	}

	return resp.WorldData.Zones, nil
}
