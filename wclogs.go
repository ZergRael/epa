package main

import (
	"fmt"
	"math/rand"
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
	*wclogs.Character
	ChannelID string
}

var goodParse = []string{
	":partying_face:",
	":muscle:",
	":chart_with_upwards_trend:",
	":trophy:",
	":clap:",
	":star_struck:",
	":crown:",
}
var badParse = []string{
	"Nice try, but you suck",
	"That was nice. You should try harder ?",
	":pleading_face:",
	"At some point, you might have more than a green parse",
	"This is bad, but it could be worse",
	"Better luck next time",
	"Here, have a participation trophy :trophy:",
}

// instantiateWCLogsForGuild tries to fetch wclogs.Credentials from database and validate them before starting ticker
func instantiateWCLogsForGuild(guildID string) {
	// WCLogs credentials
	creds, err := fetchWCLogsCredentials(db, guildID)
	if err != nil || creds == nil {
		log.Warn().Err(err).Str("guildID", guildID).Msg("Cannot read WCLogs credentials for guild")
		return
	}

	// Check WCLogs credentials
	if creds.ClientID == "" && creds.ClientSecret == "" {
		log.Warn().Str("guildID", guildID).Msg("Missing credentials for guild")
		return
	}

	w := wclogs.New(creds, wclogs.Classic, nil)
	if !w.Connect() {
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

// destroyWCLogsForGuild unregisters WCLogs credentials, deletes live tracks and remove timers & tickers
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

// registerWarcraftLogs instantiates a new WCLogs with credentials for a specific guildID
func registerWarcraftLogs(clientID, clientSecret, guildID string) string {
	creds := &wclogs.Credentials{ClientID: clientID, ClientSecret: clientSecret}
	w := wclogs.New(creds, wclogs.Classic, nil)
	if !w.Connect() {
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

// unregisterWarcraftLogs destroys WCLogs instance
func unregisterWarcraftLogs(guildID string) string {
	if logs[guildID] == nil {
		return "No stored credentials"
	}

	destroyWCLogsForGuild(guildID)

	return "Unregister successful"
}

// trackCharacter tries to add a regular performance track on a specific character
func trackCharacter(name, server, region, guildID, channelID string) string {
	if logs[guildID] == nil {
		return "Missing WarcraftLogs credentials setup"
	}

	char, err := logs[guildID].GetCharacter(name, server, region)
	if err != nil || char == nil {
		log.Error().Str("slug", char.Slug()).Err(err).Msg("GetCharacterID failed")
		return "Failed to track " + char.Slug() + " : character not found !"
	}

	for idx, c := range *trackedCharacters[guildID] {
		if c.Character.ID == char.ID {
			// Remove currently tracked character, it will be added back again later
			// hackish way to allow update to announce channel
			*trackedCharacters[guildID] = append((*trackedCharacters[guildID])[:idx], (*trackedCharacters[guildID])[idx+1:]...)
			log.Warn().Str("slug", char.Slug()).Int("charID", char.ID).Err(err).Msg("Already tracked")
		}
	}

	reportMetadata, err := logs[guildID].GetLatestReportMetadata(char)
	if err != nil {
		log.Error().Str("slug", char.Slug()).Int("charID", char.ID).
			Err(err).Msg("GetLatestReportMetadata failed")
		return "Failed to track " + char.Slug() + " : no recent report"
	}

	err = storeWCLogsLatestReportForCharacterID(db, char.ID, reportMetadata)
	if err != nil {
		log.Error().Str("slug", char.Slug()).Int("charID", char.ID).
			Err(err).Msg("storeWCLogsLatestReportForCharacterID failed")
		return "Failed to track " + char.Slug()
	}

	trackedChar := TrackedCharacter{Character: char, ChannelID: channelID}
	*trackedCharacters[guildID] = append(*trackedCharacters[guildID], trackedChar)
	err = storeWCLogsTrackedCharacters(db, guildID, trackedCharacters[guildID])
	if err != nil {
		log.Error().Str("slug", char.Slug()).Int("charID", char.ID).
			Err(err).Msg("storeWCLogsTrackedCharacters failed")
		return "Failed to track " + char.Slug()
	}

	// Don't record parses here as it may be too slow for discord response
	// ZoneParses will be recorded on next check ticker

	log.Info().Str("slug", char.Slug()).Err(err).Msg("Track successful")
	return char.Slug() + " is now tracked"
}

// untrackCharacter removes a character for current tracking
func untrackCharacter(name, server, region, guildID string) string {
	if logs[guildID] == nil {
		return "Missing WarcraftLogs credentials setup"
	}

	char, err := logs[guildID].GetCharacter(name, server, region)
	if err != nil {
		log.Error().Str("slug", char.Slug()).Err(err).Msg("GetCharacterID failed")
		return "Failed to untrack " + char.Slug() + " : character not found !"
	}

	for idx, c := range *trackedCharacters[guildID] {
		if c.Character.ID == char.ID {
			*trackedCharacters[guildID] = append((*trackedCharacters[guildID])[:idx], (*trackedCharacters[guildID])[idx+1:]...)
			log.Info().Str("slug", char.Slug()).Msg("Untrack successful")
			return char.Slug() + " is not tracked anymore"
		}
	}

	log.Warn().Str("slug", char.Slug()).Err(err).Msg("Not tracked")
	return char.Slug() + " was not tracked"
}

// currentParses returns cached performances for a character
// TODO: improve formatting
func currentParses(name, server, region, guildID string) string {
	if logs[guildID] == nil {
		return "Missing WarcraftLogs credentials setup"
	}

	char, err := logs[guildID].GetCharacter(name, server, region)
	if err != nil {
		log.Error().Str("slug", char.Slug()).Err(err).Msg("GetCharacterID failed")
		return "Failed to get parses for " + char.Slug() + " : character not found !"
	}

	dbParses, err := fetchWCLogsParsesForCharacterID(db, char.ID)
	if err != nil || dbParses == nil {
		log.Warn().Str("slug", char.Slug()).Err(err).Msg("Not tracked")
		return char.Slug() + " is not tracked"
	}

	content := ""
	for _, zoneParses := range *dbParses {
		for metric, parses := range zoneParses {
			content += "**" + string(metric) + "**\n"
			for _, ranking := range parses.Rankings {
				content += ranking.Encounter.Name + ": " +
					fmt.Sprintf("%.2f", ranking.RankPercent) + "\n"
			}
		}
	}

	log.Info().Str("slug", char.Slug()).Msg("Sent parses")
	return content
}

// listTrackedCharacters returns a list of all known and tracked characters
func listTrackedCharacters(guildID string) string {
	if logs[guildID] == nil {
		return "Missing WarcraftLogs credentials setup"
	}

	res := "Tracked characters :\n"
	for _, char := range *trackedCharacters[guildID] {
		// TODO: Add latest report EndTime from db
		res += char.Slug() + "\n"
	}

	return res
}

func checkWCLogsForCharacterUpdates(guildID string, char *TrackedCharacter) error {
	dbReport, err := fetchWCLogsLatestReportForCharacterID(db, char.ID)
	if err != nil {
		// Missing latest report, we should have recorded at least one from trackCharacter
		return err
	}

	report, err := logs[guildID].GetLatestReportMetadata(char.Character)
	if err != nil {
		return err
	}

	dbParses, err := fetchWCLogsParsesForCharacterID(db, char.ID)
	if err != nil || (*dbParses)[report.Zone.ID] == nil {
		log.Warn().Int("charID", char.ID).Msg("fetchWCLogsParsesForCharacterID : missing parses")
		// Missing parses from database for this character
		zoneParses, err := logs[guildID].GetCurrentZoneParsesForCharacter(char.Character, report.Zone.ID)
		if err != nil {
			return err
		}

		parses := make(wclogs.Parses)
		parses[report.Zone.ID] = *zoneParses
		err = storeWCLogsParsesForCharacterID(db, char.ID, &parses)
		if err != nil {
			return err
		}

		return nil
	}

	if report.Code != dbReport.Code {
		log.Info().
			Int("charID", char.ID).Str("code", report.Code).
			Msg("checkWCLogsForCharacterUpdates: new report code")

		// Current report has to be older than stored one, anything else might indicate wclogs deletion
		if report.EndTime > dbReport.EndTime {
			announceNewReport(report, char)
		}
	}

	if report.EndTime == dbReport.EndTime {
		log.Debug().Int("charID", char.ID).Str("code", report.Code).
			Msg("checkWCLogsForCharacterUpdates : no latest report changes")
		// TODO: Check if EndTime is too old and ask if we should continue tracking ?
		return nil
	}

	log.Info().Int("charID", char.ID).Str("code", report.Code).
		Float64("endTime", report.EndTime).Float64("dbEndTime", dbReport.EndTime).
		Msg("checkWCLogsForCharacterUpdates : latest report changes")

	// Store metadata now, too bad if we err later
	err = storeWCLogsLatestReportForCharacterID(db, char.ID, report)
	if err != nil {
		return err
	}

	zoneParses, err := logs[guildID].GetCurrentZoneParsesForCharacter(char.Character, report.Zone.ID)
	if err != nil {
		return err
	}

	newParses := compareParsesAndAnnounce(zoneParses, dbParses, report, char)

	if newParses {
		(*dbParses)[report.Zone.ID] = *zoneParses
		err = storeWCLogsParsesForCharacterID(db, char.ID, dbParses)
		if err != nil {
			return err
		}
	}

	return nil
}

func announceNewReport(report *wclogs.Report, char *TrackedCharacter) {
	link := "https://classic.warcraftlogs.com/reports/" + report.Code
	content := "New report detected for " + char.Slug() + " : " + link

	_, err := s.ChannelMessageSend(char.ChannelID, content)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send message")
	}
}

func compareParsesAndAnnounce(zoneParses *wclogs.ZoneParses, dbParses *wclogs.Parses, report *wclogs.Report, char *TrackedCharacter) bool {
	newParses := false

	for metric, rankings := range *zoneParses {
		for _, ranking := range rankings.Rankings {
			for _, dbRanking := range (*dbParses)[report.Zone.ID][metric].Rankings {
				if ranking.Encounter.ID == dbRanking.Encounter.ID {
					if ranking.RankPercent > dbRanking.RankPercent {
						log.Info().
							Str("metric", string(metric)).
							Int("charID", char.ID).Float64("oldParse", dbRanking.RankPercent).
							Float64("newParse", ranking.RankPercent).Msg("New parse !")

						// TODO: Get player spec and fight ID for proper link
						link := "https://classic.warcraftlogs.com/reports/" + report.Code
						reaction := goodParse[rand.Intn(len(goodParse))]
						if ranking.RankPercent < 50 {
							reaction = badParse[rand.Intn(len(badParse))]
						}

						content := "New parse for " + char.Slug() + " on " + ranking.Encounter.Name + " !\n" +
							fmt.Sprintf("%.2f", dbRanking.RankPercent) + " :arrow_right: " +
							"**" + fmt.Sprintf("%.2f", ranking.RankPercent) + "** [" + string(metric) + "] " +
							reaction + "\n" + link

						_, err := s.ChannelMessageSend(char.ChannelID, content)
						if err != nil {
							log.Error().Err(err).Msg("Failed to send message")
						}

						newParses = true
					} else if ranking.RankPercent != dbRanking.RankPercent {
						newParses = true
					}
				}
			}
		}
	}

	return newParses
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
