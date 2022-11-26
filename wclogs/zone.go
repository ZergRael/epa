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
		Sizes []RaidSize
	}
	Encounters []struct {
		ID   int
		Name string
	}
}

func (z Zone) IsRelevant() bool {
	for _, difficulty := range z.Difficulties {
		for _, size := range difficulty.Sizes {
			if size >= minSizeTrackedEncounter {
				return true
			}
		}
	}

	return false
}

type Zones []Zone

func (z Zones) GetZoneIDForEncounter(encounterID int) ZoneID {
	for _, zone := range z {
		for _, encounter := range zone.Encounters {
			if encounter.ID == encounterID {
				return zone.ID
			}
		}
	}

	return 0
}

var cachedZones Zones

// getZones queries a collection of Zone, this is static data for each expansion
func (w *WCLogs) getZones() (Zones, error) {
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

	var zones Zones
	for _, zone := range resp.WorldData.Zones {
		if zone.IsRelevant() {
			zones = append(zones, zone)
		}
	}

	return zones, nil
}

func (w *WCLogs) cacheZones() error {
	if cachedZones != nil {
		return nil
	}

	var err error
	cachedZones, err = w.getZones()
	return err
}
