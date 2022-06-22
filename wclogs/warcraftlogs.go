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
	authorizationUri = "https://www.warcraftlogs.com/oauth/authorize"
	tokenUri         = "https://www.warcraftlogs.com/oauth/token"
	retailApiUri     = "https://www.warcraftlogs.com/api/v2/client"
	classicApiUri    = "https://classic.warcraftlogs.com/api/v2/client"
)

type WCLogs struct {
	client *graphql.Client
}

type Credentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type rateLimit struct {
	RateLimitData struct {
		LimitPerHour        int
		PointsSpentThisHour int
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
		return false
	}

	log.Debug().Interface("rateLimitData", resp.RateLimitData).Msg("WCLogs checked")

	return true
}
