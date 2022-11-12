package wclogs

import (
	"context"
	"github.com/machinebox/graphql"
	"math"
)

// ZoneID represents the raid cluster zone identifier
type ZoneID int

// Parses contains ZoneParses for multiple zones
type Parses map[ZoneID]ZoneParses

// Metric is either dps or hps
type Metric string

// ZoneParses contains ZoneRankings for multiple Metric
type ZoneParses map[Metric]ZoneRankings

// ZoneRankings contains a collection of Rankings for a specific Partition
type ZoneRankings struct {
	Partition int
	Rankings  []Ranking
}

// Ranking contains a RankPercent for a specific Encounter
type Ranking struct {
	Encounter struct {
		ID   int
		Name string
	}
	RankPercent float64
}

// GetCurrentZoneParsesForCharacter queries HPS and DPS ZoneParses for a specific Character and zone ID
func (w *WCLogs) GetCurrentZoneParsesForCharacter(char *Character, zoneID ZoneID) (*ZoneParses, error) {
	req := graphql.NewRequest(`
    query ($id: Int!, $zoneID: Int!, $withHps: Boolean!) {
		characterData {
			character(id: $id) {
				hpsZoneRankings: zoneRankings(metric: hps, zoneID: $zoneID) @include(if: $withHps)
				dpsZoneRankings: zoneRankings(metric: dps, zoneID: $zoneID)
			}
		}
    }
`)

	req.Var("id", char.ID)
	req.Var("zoneID", zoneID)
	req.Var("withHps", char.CanHeal())

	var resp struct {
		CharacterData struct {
			Character struct {
				HpsZoneRankings ZoneRankings
				DpsZoneRankings ZoneRankings
			}
		}
	}

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		return nil, err
	}

	parses := make(ZoneParses)
	parses["dps"] = resp.CharacterData.Character.DpsZoneRankings
	if char.CanHeal() {
		parses["hps"] = resp.CharacterData.Character.HpsZoneRankings
	}

	// HACK: Lower float resolution to help mitigate precision issues
	for metric, rankings := range parses {
		for idx, ranking := range rankings.Rankings {
			parses[metric].Rankings[idx].RankPercent = math.Round(ranking.RankPercent*10000) / 10000
		}
	}

	return &parses, nil
}
