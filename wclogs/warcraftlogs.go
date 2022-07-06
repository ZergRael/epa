package wclogs

import (
	"context"
	"errors"

	"github.com/machinebox/graphql"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

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

type Parses map[string]ZoneRankings

var ParsesMetrics = []string{"hps", "dps"}

type Report struct {
	Code    string
	EndTime float32
}

type RateLimitData struct {
	LimitPerHour        int
	PointsSpentThisHour float32
	PointsResetIn       int
}

func NewWCLogs(creds *Credentials, debugLogsFunc func(string)) *WCLogs {
	c := clientcredentials.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		TokenURL:     tokenUri,
		AuthStyle:    oauth2.AuthStyleInHeader,
	}

	// TODO: check context value
	client := graphql.NewClient(classicApiUri, graphql.WithHTTPClient(c.Client(context.Background())))
	if debugLogsFunc != nil {
		client.Log = debugLogsFunc
	}

	w := WCLogs{client: client}

	return &w
}

func (w *WCLogs) Check() bool {
	_, err := w.GetRateLimits()
	return err == nil
}

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

func (w *WCLogs) GetCurrentParsesForCharacter(charID int) (*Parses, error) {
	req := graphql.NewRequest(`
    query ($id: Int!, $metric: CharacterRankingMetricType!) {
		characterData {
			character(id: $id) {
				zoneRankings(metric: $metric)
			}
		}
    }
`)

	req.Var("id", charID)

	parses := make(Parses)

	for _, metric := range ParsesMetrics {
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

	return &parses, nil
}

func (w *WCLogs) GetLatestReportMetadata(charID int) (*Report, error) {
	req := graphql.NewRequest(`
    query ($id: Int!) {
		characterData {
			character(id: $id) {
				recentReports(limit: 1) {
					data {
						endTime
						code
					}
				}
			}
		}
    }
`)

	req.Var("id", charID)

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
