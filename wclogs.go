package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zergrael/epa/wclogs"
)

var trackedCharacters map[string]*[]TrackedCharacter
var characterTrackTicker map[string]*time.Ticker
var timerStopper map[string]chan bool

// TODO: Adjust ticker duration based on latest report age
const characterTrackTickerDuration = 2 * time.Minute

type TrackedCharacter struct {
	CharID    int
	Slug      string
	ChannelID string
}

var winEmojis = []string{
	":partying_face:",
	":muscle:",
	":bangbang:",
	":chart_with_upwards_trend:",
}

// instantiateWCLogsForGuild tries to fetch wclogs.Credentials from database and validate them before starting ticker
func instantiateWCLogsForGuild(guildID string) {
	// WCLogs credentials
	creds, err := fetchWCLogsCredentials(db, guildID)
	if err != nil || creds == nil {
		log.Debug().Err(err).Str("guildID", guildID).Msg("Cannot read WCLogs credentials for guild")
		return
	}

	// Check WCLogs credentials
	if creds.ClientID == "" && creds.ClientSecret == "" {
		log.Warn().Str("guildID", guildID).Msg("Missing credentials for guild")
		return
	}

	w := wclogs.New(creds, true, nil)
	if !w.Check() {
		log.Warn().Str("guildID", guildID).Msg("Failed to reuse credentials for guild")
	}

	log.Info().Str("guildID", guildID).Msg("WCLogs instance successful")
	logs[guildID] = w

	if trackedCharacters == nil {
		trackedCharacters = make(map[string]*[]TrackedCharacter)
	}
	trackedCharacters[guildID], err = fetchWCLogsTrackedCharacters(db, guildID)
	if err != nil {
		log.Warn().Err(err).Msg("No currently tracked characters")
		characters := make([]TrackedCharacter, 0)
		trackedCharacters[guildID] = &characters
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
	w := wclogs.New(creds, true, nil)
	if !w.Check() {
		return "These API credentials cannot be used"
	}

	log.Info().Str("guildID", guildID).Msg("WCLogs instance successful")
	logs[guildID] = w

	if trackedCharacters == nil {
		trackedCharacters = make(map[string]*[]TrackedCharacter)
	}
	var err error
	trackedCharacters[guildID], err = fetchWCLogsTrackedCharacters(db, guildID)
	if err != nil {
		log.Warn().Err(err).Msg("No currently tracked characters")
		characters := make([]TrackedCharacter, 0)
		trackedCharacters[guildID] = &characters
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

func handleUnregisterWarcraftLogs(guildID string) string {
	if logs[guildID] == nil {
		return "No stored credentials"
	}

	destroyWCLogsForGuild(guildID)

	return "Unregister successful"
}

func handleTrackCharacter(name, server, region, guildID, channelID string) string {
	if logs[guildID] == nil {
		return "Missing WarcraftLogs credentials setup"
	}

	charSlug := name + "-" + server + "[" + region + "]"
	charID, err := logs[guildID].GetCharacterID(name, server, region)
	if err != nil || charID == 0 {
		log.Error().Str("slug", charSlug).Err(err).Msg("GetCharacterID failed")
		return "Failed to track " + charSlug + " : character not found !"
	}

	for _, char := range *trackedCharacters[guildID] {
		if char.CharID == charID {
			// TODO: handle already tracked as update tracking
			log.Warn().Str("slug", charSlug).Int("charID", charID).Err(err).Msg("Already tracked")
			return charSlug + " is already tracked"
		}
	}

	reportMetadata, err := logs[guildID].GetLatestReportMetadata(charID)
	if err != nil {
		log.Error().Str("slug", charSlug).Int("charID", charID).
			Err(err).Msg("GetLatestReportMetadata failed")
		return "Failed to track " + charSlug + " : no recent report"
	}

	err = storeWCLogsLatestReportForCharacter(db, charID, reportMetadata)
	if err != nil {
		log.Error().Str("slug", charSlug).Int("charID", charID).
			Err(err).Msg("storeWCLogsLatestReportForCharacter failed")
		return "Failed to track " + charSlug
	}

	char := TrackedCharacter{CharID: charID, ChannelID: channelID, Slug: charSlug}
	*trackedCharacters[guildID] = append(*trackedCharacters[guildID], char)
	err = storeWCLogsTrackedCharacters(db, guildID, trackedCharacters[guildID])
	if err != nil {
		log.Error().Str("slug", charSlug).Int("charID", charID).
			Err(err).Msg("storeWCLogsTrackedCharacters failed")
		return "Failed to track " + charSlug
	}

	// Don't record parses here as it may be too slow for discord response
	// Parses will be recorded on next check ticker

	return charSlug + " is now tracked"
}

func handleUntrackCharacter(name, server, region, guildID string) string {
	if logs[guildID] == nil {
		return "Missing WarcraftLogs credentials setup"
	}

	charSlug := name + "-" + server + "[" + region + "]"
	charID, err := logs[guildID].GetCharacterID(name, server, region)
	if err != nil {
		log.Error().Str("slug", charSlug).Err(err).Msg("GetCharacterID failed")
		return "Failed to untrack " + charSlug + " : character not found !"
	}

	charSlug += " (" + strconv.Itoa(charID) + ")"
	for idx, char := range *trackedCharacters[guildID] {
		if char.CharID == charID {
			*trackedCharacters[guildID] = append((*trackedCharacters[guildID])[:idx], (*trackedCharacters[guildID])[idx+1:]...)
			log.Debug().Str("slug", charSlug).Err(err).Msg("Untracked")
			return charSlug + " is already tracked"
		}
	}

	log.Warn().Str("slug", charSlug).Err(err).Msg("Not tracked")
	return charSlug + " is not tracked"
}

func handleListTrackedCharacters(guildID string) string {
	if logs[guildID] == nil {
		return "Missing WarcraftLogs credentials setup"
	}

	res := "Tracked characters :\n"
	for _, char := range *trackedCharacters[guildID] {
		// TODO: Add latest report EndTime from db
		res += char.Slug + "\n"
	}

	return res
}

func checkWCLogsForCharacterUpdates(guildID string, char *TrackedCharacter) error {
	dbReport, err := fetchWCLogsLatestReportForCharacter(db, char.CharID)
	if err != nil {
		// Missing latest report
		// Since we should have recorded at least one on track
		// Assume this is an error
		return err
	}

	report, err := logs[guildID].GetLatestReportMetadata(char.CharID)
	if err != nil {
		return err
	}

	dbParses, err := fetchWCLogsParsesForCharacter(db, char.CharID)
	if err != nil {
		log.Debug().Int("charID", char.CharID).Msg("fetchWCLogsParsesForCharacter : missing parses")
		// Missing parses
		// This is expected, and we should assume we just need to record them
		dbParses, err = logs[guildID].GetCurrentParsesForCharacter(char.CharID)
		if err != nil {
			return err
		}

		err = storeWCLogsParsesForCharacter(db, char.CharID, dbParses)
		if err != nil {
			return err
		}

		return nil
	}

	if report.EndTime == dbReport.EndTime {
		log.Debug().Int("charID", char.CharID).Str("code", report.Code).
			Msg("checkWCLogsForCharacterUpdates : no latest report changes")
		// TODO: Check if EndTime is too old and ask if we should continue tracking ?
		return nil
	}

	log.Debug().Int("charID", char.CharID).Str("code", report.Code).
		Float32("endTime", report.EndTime).Float32("dbEndTime", dbReport.EndTime).
		Msg("checkWCLogsForCharacterUpdates : latest report changes")

	// Store metadata now, too bad if we err later
	err = storeWCLogsLatestReportForCharacter(db, char.CharID, report)
	if err != nil {
		return err
	}

	parses, err := logs[guildID].GetCurrentParsesForCharacter(char.CharID)
	if err != nil {
		return err
	}

	newParses := false
	for metric, rankings := range *parses {
		for _, ranking := range rankings.Rankings {
			for _, dbRanking := range (*dbParses)[metric].Rankings {
				if ranking.Encounter.ID == dbRanking.Encounter.ID {
					if ranking.RankPercent > dbRanking.RankPercent {
						log.Info().
							Str("metric", metric).
							Int("charID", char.CharID).Float32("oldParse", dbRanking.RankPercent).
							Float32("newParse", ranking.RankPercent).Msg("New parse !")
						link := "https://classic.warcraftlogs.com/reports/" + report.Code
						content := "New parse for " + char.Slug + " on " + ranking.Encounter.Name + " !\n" +
							fmt.Sprintf("%.2f", dbRanking.RankPercent) + " :arrow_right: " +
							"**" + fmt.Sprintf("%.2f", ranking.RankPercent) + "** [" + metric + "] " +
							winEmojis[rand.Intn(len(winEmojis))] + "\n" + link
						_, err := s.ChannelMessageSend(char.ChannelID, content)
						newParses = true
						if err != nil {
							return err
						}
					} else if ranking.RankPercent != dbRanking.RankPercent {
						newParses = true
					}
				}
			}
		}
	}

	if newParses {
		err = storeWCLogsParsesForCharacter(db, char.CharID, parses)
		if err != nil {
			return err
		}
	}

	return nil
}

// setupWCLogsTicker starts the periodic check ticker, including character parses updates
func setupWCLogsTicker(guildID string) {
	if characterTrackTicker == nil {
		characterTrackTicker = make(map[string]*time.Ticker)
	}
	if timerStopper == nil {
		timerStopper = make(map[string]chan bool)
	}
	characterTrackTicker[guildID] = time.NewTicker(characterTrackTickerDuration)
	timerStopper[guildID] = make(chan bool)

	log.Info().Str("guildID", guildID).Msg("Started ticker")

	go func(guildID string) {
		for {
			select {
			case <-timerStopper[guildID]:
				return
			case <-characterTrackTicker[guildID].C:
				for _, char := range *trackedCharacters[guildID] {
					err := checkWCLogsForCharacterUpdates(guildID, &char)
					if err != nil {
						log.Error().Err(err).Msg("checkWCLogsForCharacterUpdates")
					}
				}
			}
		}
	}(guildID)
}
