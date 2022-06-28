package main

import (
	"epa/wclogs"
	"strconv"

	"github.com/rs/zerolog/log"
)

var trackedCharacters map[string][]int

func instantiateWCLogsForGuild(guildID string) {
	// WCLogs credentials
	creds, err := fetchWCLogsCredentials(db, guildID)
	if err != nil {
		log.Debug().Err(err).Str("guildID", guildID).Msg("Cannot read WCLogs credentials for guild")
	}

	// Check WCLogs credentials
	if creds.ClientID == "" && creds.ClientSecret == "" {
		log.Warn().Str("guildID", guildID).Msg("Missing credentials for guild")
		return
	}

	w := wclogs.NewWCLogs(creds)
	if !w.Check() {
		log.Warn().Str("guildID", guildID).Msg("Failed to reuse credentials for guild")
	}

	log.Info().Str("guildID", guildID).Msg("WCLogs instance successful")
	logs[guildID] = w

	if trackedCharacters == nil {
		trackedCharacters = make(map[string][]int)
	}
	trackedCharacters[guildID], err = fetchWCLogsTrackedCharacters(db, guildID)
	if err != nil {
		log.Warn().Err(err).Msg("No currently tracked characters")
		trackedCharacters[guildID] = make([]int, 0)
	}
}

func handleRegisterWarcraftLogs(clientID, clientSecret, guildID string) string {
	creds := &wclogs.Credentials{ClientID: clientID, ClientSecret: clientSecret}
	w := wclogs.NewWCLogs(creds)
	if !w.Check() {
		return "These API credentials cannot be used"
	}

	log.Info().Str("guildID", guildID).Msg("WCLogs instance successful")
	logs[guildID] = w

	if trackedCharacters == nil {
		trackedCharacters = make(map[string][]int)
	}
	trackedCharacters[guildID] = make([]int, 0)
	log.Info().Str("guildID", guildID).Msg("WCLogs instance successful")
	err := storeWCLogsCredentials(db, guildID, creds)
	if err != nil {
		log.Error().Str("guildID", guildID).Err(err).Msg("storeWCLogsCredentials failed")
		return "API credentials are valid, but I failed to store them"
	}

	return "Congrats, API credentials are valid"
}

func handleTrackCharacter(name, server, region, guildID string) string {
	if logs[guildID] == nil {
		return "Missing WarcraftLogs credentials setup"
	}

	charSlug := name + "-" + server + "[" + region + "]"
	charID, err := logs[guildID].GetCharacterID(name, server, region)
	if err != nil {
		log.Error().Str("slug", charSlug).Err(err).Msg("GetCharacterID failed")
		return "Failed to track " + charSlug + " : character not found !"
	}

	charSlug += " (" + strconv.Itoa(charID) + ")"
	for _, id := range trackedCharacters[guildID] {
		if id == charID {
			log.Warn().Str("slug", charSlug).Err(err).Msg("Already tracked")
			return charSlug + " is already tracked"
		}
	}

	reportMetadata, err := logs[guildID].GetLatestReportMetadata(charID)
	if err != nil {
		log.Error().Str("slug", charSlug).Err(err).Msg("GetLatestReportMetadata failed")
		return "Failed to track " + charSlug + " : no recent report"
	}

	err = storeWCLogsLatestReportForCharacter(db, charID, reportMetadata)
	if err != nil {
		log.Error().Str("slug", charSlug).Err(err).Msg("storeWCLogsLatestReportForCharacter failed")
		return "Failed to track " + charSlug
	}

	trackedCharacters[guildID] = append(trackedCharacters[guildID], charID)
	err = storeWCLogsTrackedCharacters(db, guildID, trackedCharacters[guildID])
	if err != nil {
		log.Error().Str("slug", charSlug).Err(err).Msg("storeWCLogsTrackedCharacters failed")
		return "Failed to track " + charSlug
	}

	return charSlug + " is now tracked"
}

// TODO: Run on timer
func checkWCLogsForCharacterUpdates(guildID string, charID int) error {
	dbReport, err := fetchWCLogsLatestReportForCharacter(db, charID)
	if err != nil {
		return err
	}

	report, err := logs[guildID].GetLatestReportMetadata(charID)
	if err != nil {
		return err
	}

	if report.EndTime == dbReport.EndTime {
		return nil
	}

	// Get parses and diff them
	return nil
}
