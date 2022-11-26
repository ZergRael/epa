package wclogs

import (
	"context"
	"github.com/machinebox/graphql"
	"math"
)

// ZoneID represents the raid cluster zone identifier
type ZoneID int

// Parses contains SizeRankings for multiple ZoneID
type Parses map[ZoneID]SizeRankings

// RaidSize is 10/25/40 raid group size
type RaidSize int

// SizeRankings contains MetricRankings for multiple RaidSize
type SizeRankings map[RaidSize]MetricRankings

// Metric is either dps or hps
type Metric string

// MetricRankings contains Rankings for multiple Metric
type MetricRankings map[Metric]PartitionRankings

// PartitionRankings contains a collection of Rankings for a specific Partition
type PartitionRankings struct {
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

// GetMetricRankingsForCharacter queries HPS and DPS ZoneParses for a specific Character, zone ID and raid size
func (w *WCLogs) GetMetricRankingsForCharacter(char *Character, zoneID ZoneID, size RaidSize) (*MetricRankings, error) {
	req := graphql.NewRequest(`
    query ($id: Int!, $zoneID: Int!, $size: Int!, $withHps: Boolean!) {
		characterData {
			character(id: $id) {
				hpsZoneRankings: zoneRankings(metric: hps, zoneID: $zoneID, size: $size) @include(if: $withHps)
				dpsZoneRankings: zoneRankings(metric: dps, zoneID: $zoneID, size: $size)
			}
		}
    }
`)

	req.Var("id", char.ID)
	req.Var("zoneID", zoneID)
	req.Var("size", size)
	req.Var("withHps", char.CanHeal())

	var resp struct {
		CharacterData struct {
			Character struct {
				HpsZoneRankings PartitionRankings
				DpsZoneRankings PartitionRankings
			}
		}
	}

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		return nil, err
	}

	parses := make(MetricRankings)
	parses["dps"] = resp.CharacterData.Character.DpsZoneRankings
	if char.CanHeal() {
		parses["hps"] = resp.CharacterData.Character.HpsZoneRankings
	}

	// HACK: Lower float resolution to help mitigate precision issues
	for metric, rankings := range parses {
		for idx, ranking := range rankings.Rankings {
			parses[metric].Rankings[idx].RankPercent = math.Round(ranking.RankPercent*1000) / 1000
		}
	}

	return &parses, nil
}
