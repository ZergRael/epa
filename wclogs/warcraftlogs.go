// Package wclogs contains most of the WarcraftLogs API service communication
package wclogs

import (
	"context"
	"github.com/machinebox/graphql"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

//https://www.warcraftlogs.com/api/docs
//https://www.warcraftlogs.com/v2-api-docs/warcraft/

const (
	// authorizationUri = "https://www.warcraftlogs.com/oauth/authorize"
	tokenUri = "https://www.warcraftlogs.com/oauth/token"
)

const minSizeTrackedEncounter = 10

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

// RateLimitData contains WarcraftLogs API rate limits results, usually 3600 points per hour
type RateLimitData struct {
	LimitPerHour        int
	PointsSpentThisHour float64
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

	// TODO: check context value
	client := graphql.NewClient(flavor.Uri(), graphql.WithHTTPClient(c.Client(context.Background())))
	if debugLogsFunc != nil {
		client.Log = debugLogsFunc
	}

	w := WCLogs{client: client, flavor: flavor}

	return &w
}

// Connect tries to connect to WarcraftLogs API, mostly used to validate credentials
// TODO: it could be useful to also check rate limits here
func (w *WCLogs) Connect() bool {
	_, err := w.GetRateLimits()
	if err != nil {
		return false
	}

	err = w.cacheZones()
	if err != nil {
		return false
	}

	return true
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
