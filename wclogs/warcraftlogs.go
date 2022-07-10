// Package wclogs contains most of the WarcraftLogs API service communication
package wclogs

import (
	"context"
	"errors"

	"github.com/machinebox/graphql"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

//https://www.warcraftlogs.com/api/docs
//https://www.warcraftlogs.com/v2-api-docs/warcraft/

const (
	// authorizationUri = "https://www.warcraftlogs.com/oauth/authorize"
	tokenUri      = "https://www.warcraftlogs.com/oauth/token"
	retailApiUri  = "https://www.warcraftlogs.com/api/v2/client"
	classicApiUri = "https://classic.warcraftlogs.com/api/v2/client"
	vanillaApiUri = "https://vanilla.warcraftlogs.com/api/v2/client"
)

// WCLogs is the WarcraftLogs graphql API client holder
type WCLogs struct {
	client *graphql.Client
	flavor Flavor
}

// Credentials represents WarcraftLogs credentials used to read from API
type Credentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// Zone represents a WoW zone
type Zone struct {
	ID   int
	Name string
}

// Ranking contains a RankPercent for a specific Encounter
type Ranking struct {
	Encounter struct {
		ID   int
		Name string
	}
	RankPercent float32
}

// ZoneRankings contains a collection of Rankings for a specific Partition
type ZoneRankings struct {
	Partition int
	Rankings  []Ranking
}

type Metric string

// ZoneParses contains ZoneRankings for multiple Metric
type ZoneParses map[Metric]ZoneRankings

// Parses contains ZoneParses for multiple zones
type Parses map[int]ZoneParses

// Report represents WarcraftLogs report metadata
type Report struct {
	Code    string
	EndTime float32
	Zone    struct {
		ID int
	}
}

// RateLimitData contains WarcraftLogs API rate limits results, usually 3600 points per hour
type RateLimitData struct {
	LimitPerHour        int
	PointsSpentThisHour float32
	PointsResetIn       int
}

// New instantiates a new WCLogs graphql client
func New(creds *Credentials, flavor Flavor, debugLogsFunc func(string)) *WCLogs {
	c := clientcredentials.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		TokenURL:     tokenUri,
		AuthStyle:    oauth2.AuthStyleInHeader,
	}

	uri := retailApiUri
	switch flavor {
	case Classic:
		uri = classicApiUri
	case Vanilla:
		uri = vanillaApiUri
	}

	// TODO: check context value
	client := graphql.NewClient(uri, graphql.WithHTTPClient(c.Client(context.Background())))
	if debugLogsFunc != nil {
		client.Log = debugLogsFunc
	}

	w := WCLogs{client: client, flavor: flavor}

	return &w
}

// Check tries to connect to WarcraftLogs API, mostly used to validate credentials
// TODO: it could be useful to also check rate limits here
func (w *WCLogs) Check() bool {
	_, err := w.GetRateLimits()
	return err == nil
}

// GetRateLimits queries RateLimitData from WarcraftLogs API
func (w *WCLogs) GetRateLimits() (*RateLimitData, error) {
	req := graphql.NewRequest(`
    query {
        rateLimitData {
            limitPerHour
            pointsSpentThisHour
            pointsResetIn
        }
    }
`)

	var resp struct {
		RateLimitData RateLimitData
	}

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		return nil, err
	}

	return &resp.RateLimitData, nil
}

// GetCharacter queries WarcraftLogs character info based on character name, server and server region
func (w *WCLogs) GetCharacter(name, server, region string) (*Character, error) {
	req := graphql.NewRequest(`
    query ($name: String!, $server: String!, $region: String!) {
		characterData {
			character(name: $name, serverSlug: $server, serverRegion: $region) {
				id
				name
				server {
					name
					region {
						slug
					}
				}
			}
		}
    }
`)

	req.Var("name", name)
	req.Var("server", server)
	req.Var("region", region)

	var resp struct {
		CharacterData struct {
			Character struct {
				ID      int
				Name    string
				ClassID int
				Server  struct {
					Name   string
					Region struct {
						Slug string
					}
				}
			}
		}
	}

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		return nil, err
	}

	return &Character{
		ID:      resp.CharacterData.Character.ID,
		Name:    resp.CharacterData.Character.Name,
		Server:  resp.CharacterData.Character.Server.Name,
		Region:  resp.CharacterData.Character.Server.Region.Slug,
		ClassID: resp.CharacterData.Character.ClassID,
	}, nil
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
	req.Var("expansion", w.flavor.LatestExpansion())

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

// GetCurrentZoneParsesForCharacter queries HPS and DPS ZoneParses for a specific Character and zone ID
func (w *WCLogs) GetCurrentZoneParsesForCharacter(char *Character, zoneID int) (*ZoneParses, error) {
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

	return &parses, nil
}

// GetLatestReportMetadata queries latest Report for a specific Character
func (w *WCLogs) GetLatestReportMetadata(char *Character) (*Report, error) {
	req := graphql.NewRequest(`
    query ($id: Int!) {
		characterData {
			character(id: $id) {
				recentReports(limit: 1) {
					data {
						endTime
						code
						zone {
							id
						}
					}
				}
			}
		}
    }
`)

	req.Var("id", char.ID)

	var resp struct {
		CharacterData struct {
			Character struct {
				RecentReports struct {
					Data []Report
				}
			}
		}
	}

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		return nil, err
	}

	if len(resp.CharacterData.Character.RecentReports.Data) < 1 {
		return nil, errors.New("no recent report")
	}

	return &resp.CharacterData.Character.RecentReports.Data[0], nil
}
