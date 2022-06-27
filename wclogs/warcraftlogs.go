package wclogs

import (
	"context"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// Register playername
// Regular pull from GraphQL
// Emote on new parse

//https://www.warcraftlogs.com/api/docs
//https://www.warcraftlogs.com/v2-api-docs/warcraft/user.doc.html

const (
	authorizationUri   = "https://www.warcraftlogs.com/oauth/authorize"
	tokenUri           = "https://www.warcraftlogs.com/oauth/token"
	retailApiUri       = "https://www.warcraftlogs.com/api/v2/client"
	classicApiUri      = "https://classic.warcraftlogs.com/api/v2/client"
	classicExpansionID = 1001
)

type WCLogs struct {
	client *graphql.Client
}

type Credentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type Zone struct {
	ID   int
	Name string
}

type Ranking struct {
	Encounter struct {
		ID   int
		Name string
	}
	RankPercent float32
}

type ZoneRankings struct {
	Partition int
	Rankings  []Ranking
}

type rateLimit struct {
	RateLimitData struct {
		LimitPerHour        int
		PointsSpentThisHour float32
		PointsResetIn       int
	}
}

func NewWCLogs(creds *Credentials) *WCLogs {
	c := clientcredentials.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		TokenURL:     tokenUri,
		AuthStyle:    oauth2.AuthStyleInHeader,
	}

	// TODO: check context value
	client := graphql.NewClient(classicApiUri, graphql.WithHTTPClient(c.Client(context.Background())))
	client.Log = func(s string) { log.Debug().Msg(s) }

	w := WCLogs{client: client}

	return &w
}

func (w *WCLogs) Check() bool {
	req := graphql.NewRequest(`
    query {
        rateLimitData {
            limitPerHour
            pointsSpentThisHour
            pointsResetIn
        }
    }
`)

	var resp rateLimit

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		log.Error().Err(err).Msg("Check failed")
		return false
	}

	log.Debug().Interface("rateLimitData", resp.RateLimitData).Msg("WCLogs checked")

	return true
}

func (w *WCLogs) GetCharacterID(char, server, region string) (int, error) {
	req := graphql.NewRequest(`
    query ($name: String!, $server: String!, $region: String!) {
		characterData {
			character(name: $name, serverSlug: $server, serverRegion: $region) {
				id
			}
		}
    }
`)

	req.Var("name", char)
	req.Var("server", server)
	req.Var("region", region)

	var resp struct {
		CharacterData struct {
			Character struct {
				ID int
			}
		}
	}

	if err := w.client.Run(context.Background(), req, &resp); err != nil {
		return 0, err
	}

	return resp.CharacterData.Character.ID, nil
}

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
	req.Var("expansion", classicExpansionID)

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

func (w *WCLogs) CheckCurrentParsesForCharacter(id int) (map[string]ZoneRankings, error) {
	req := graphql.NewRequest(`
    query ($id: Int!, $metric: CharacterRankingMetricType!) {
		characterData {
			character(id: $id) {
				zoneRankings(metric: $metric)
			}
		}
    }
`)

	req.Var("id", id)

	parses := make(map[string]ZoneRankings)

	metrics := []string{"hps", "dps"}

	for _, metric := range metrics {
		req.Var("metric", metric)

		var resp struct {
			CharacterData struct {
				Character struct {
					ZoneRankings ZoneRankings
				}
			}
		}

		if err := w.client.Run(context.Background(), req, &resp); err != nil {
			return nil, err
		}

		parses[metric] = resp.CharacterData.Character.ZoneRankings
	}

	return parses, nil
}
