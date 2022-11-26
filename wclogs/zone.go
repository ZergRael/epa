package wclogs

import (
	"context"
	"github.com/machinebox/graphql"
)

// Zone represents a WoW zone
type Zone struct {
	ID           ZoneID
	Name         string
	Difficulties []struct {
		ID    int
		Name  string
		Sizes []int
	}
	Encounters []struct {
		ID   int
		Name string
	}
}

type Zones []Zone

func (Z Zones) GetZoneIDForEncounter(encounterID int) ZoneID {
	for _, z := range Z {
		for _, e := range z.Encounters {
			if e.ID == encounterID {
				return z.ID
			}
		}
	}

	return 0
}

var cachedZones Zones

// getZones queries a collection of Zone, this is static data for each expansion
func (w *WCLogs) getZones() ([]Zone, error) {
	req := graphql.NewRequest(`
    query ($expansion: Int!) {
		worldData {
			zones (expansion_id: $expansion) {
				id
				name
				difficulties {
					id
					name
					sizes
				}
				encounters {
					id
					name
				}
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

func (w *WCLogs) cacheZones() error {
	if cachedZones != nil {
		return nil
	}

	var err error
	cachedZones, err = w.getZones()
	return err
}
