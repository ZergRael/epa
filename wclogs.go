package main

import (
	"epa/wclogs"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

var trackedCharacters map[string][]int
var characterTrackTicker map[string]*time.Ticker
var timerStopper map[string]chan bool

const characterTrackTickerDuration = time.Minute

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

	// Setup tracking timer
	setupWCLogsTicker(guildID)
}

func destroyWCLogsForGuild(guildID string) {
	// Remove tracking timer
	if characterTrackTicker[guildID] != nil {
		characterTrackTicker[guildID].Stop()
	}
	if timerStopper[guildID] != nil {
		timerStopper[guildID] <- true
	}

	trackedCharacters[guildID] = nil
	logs[guildID] = nil
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
	var err error
	trackedCharacters[guildID], err = fetchWCLogsTrackedCharacters(db, guildID)
	if err != nil {
		log.Warn().Err(err).Msg("No currently tracked characters")
		trackedCharacters[guildID] = make([]int, 0)
	}

	log.Info().Str("guildID", guildID).Msg("WCLogs instance successful")
	err = storeWCLogsCredentials(db, guildID, creds)
	if err != nil {
		log.Error().Str("guildID", guildID).Err(err).Msg("storeWCLogsCredentials failed")
		return "API credentials are valid, but I failed to store them"
	}

	// Setup tracking timer
	setupWCLogsTicker(guildID)

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

	// Don't record parses here as it may be too slow for discord response
	// Parses will be recorded on next check ticker

	return charSlug + " is now tracked"
}

func checkWCLogsForCharacterUpdates(guildID string, charID int) error {
	dbReport, err := fetchWCLogsLatestReportForCharacter(db, charID)
	if err != nil {
		// Missing latest report
		// Since we should have recorded at least one on track
		// Assume this is an error
		return err
	}

	report, err := logs[guildID].GetLatestReportMetadata(charID)
	if err != nil {
		return err
	}

	dbParses, err := fetchWCLogsParsesForCharacter(db, charID)
	if err != nil {
		log.Debug().Int("charID", charID).Msg("fetchWCLogsParsesForCharacter : missing parses")
		// Missing parses
		// This is expected and we should assume we just need to record them
		dbParses, err = logs[guildID].GetCurrentParsesForCharacter(charID)
		if err != nil {
			return err
		}

		err = storeWCLogsParsesForCharacter(db, charID, dbParses)
		if err != nil {
			return err
		}

		return nil
	}

	if report.EndTime == dbReport.EndTime {
		log.Debug().Int("charID", charID).
			Msg("checkWCLogsForCharacterUpdates : no latest report changes")
		return nil
	}

	log.Debug().Int("charID", charID).
		Float32("endTime", report.EndTime).Float32("dbEndTime", dbReport.EndTime).
		Msg("checkWCLogsForCharacterUpdates : latest report changes")

	parses, err := logs[guildID].GetCurrentParsesForCharacter(charID)
	if err != nil {
		return err
	}

	for metric, rankings := range *parses {
		for _, ranking := range rankings.Rankings {
			for _, dbRanking := range (*dbParses)[metric].Rankings {
				if ranking.Encounter == dbRanking.Encounter {
					if ranking.RankPercent != dbRanking.RankPercent {
						log.Info().
							Str("metric", metric).
							Int("charID", charID).Float32("oldParse", dbRanking.RankPercent).
							Float32("newParse", ranking.RankPercent).Msg("New parse !")
						// TODO: Send message & emote on changes
						// TODO: Find out which channel to use
					}
				}
			}
		}
	}

	return nil
}

func setupWCLogsTicker(guildID string) {
	if characterTrackTicker == nil {
		characterTrackTicker = make(map[string]*time.Ticker)
	}
	if timerStopper == nil {
		timerStopper = make(map[string]chan bool)
	}
	characterTrackTicker[guildID] = time.NewTicker(characterTrackTickerDuration)
	timerStopper[guildID] = make(chan bool)

	for {
		select {
		case <-timerStopper[guildID]:
			return
		case <-characterTrackTicker[guildID].C:
			for _, charID := range trackedCharacters[guildID] {
				err := checkWCLogsForCharacterUpdates(guildID, charID)
				if err != nil {
					log.Error().Err(err).Msg("checkWCLogsForCharacterUpdates")
				}
			}
		}
	}
}
