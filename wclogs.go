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
const characterTrackTickerDuration = 1 * time.Minute

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
	"That was nice. Maybe you should try harder ?",
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

	// Record parses in goroutine as it may be too slow for discord response
	go getAndStoreAllWCLogsParsesForCharacter(guildID, &trackedChar)

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
func currentParses(name, server, region, guildID string) map[string][]string {
	if logs[guildID] == nil {
		//return "Missing WarcraftLogs credentials setup"
		return nil
	}

	char, err := logs[guildID].GetCharacter(name, server, region)
	if err != nil {
		log.Error().Str("slug", char.Slug()).Err(err).Msg("GetCharacterID failed")
		//return "Failed to get parses for " + char.Slug() + " : character not found !"
		return nil
	}

	dbParses, err := fetchWCLogsParsesForCharacterID(db, char.ID)
	if err != nil || dbParses == nil {
		log.Warn().Str("slug", char.Slug()).Err(err).Msg("Not tracked")
		//return char.Slug() + " is not tracked"
		return nil
	}

	var content = make(map[string][]string)
	content["Boss"] = []string{}

	for _, sizeRankings := range *dbParses {
		for size, zoneRankings := range sizeRankings {
			for metric, parses := range zoneRankings {
				header := fmt.Sprintf("%d-%s", size, metric)
				for _, ranking := range parses.Rankings {
					if !arrayContains(content["Boss"], ranking.Encounter.Name) {
						content["Boss"] = append(content["Boss"], ranking.Encounter.Name)
					}

					content[header] = append(content[header], fmt.Sprintf("%.2f", ranking.RankPercent))
				}
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

func getAndStoreAllWCLogsParsesForCharacter(guildID string, char *TrackedCharacter) error {
	parses, err := logs[guildID].GetParsesForCharacter(char.Character)
	if err != nil {
		return err
	}

	return storeWCLogsParsesForCharacterID(db, char.ID, parses)
}

// checkWCLogsForCharacterUpdates gets the latest report metadata and updates parses if necessary
func checkWCLogsForCharacterUpdates(guildID string, char *TrackedCharacter) error {
	// Get the latest report metadata from DB
	dbReport, err := fetchWCLogsLatestReportForCharacterID(db, char.ID)
	if err != nil {
		// Missing latest report, we should have recorded at least one from trackCharacter
		return err
	}

	// Get the latest report metadata from WCLogs
	report, err := logs[guildID].GetLatestReportMetadata(char.Character)
	if err != nil {
		return err
	}

	// Announce new report if code diff and end time is later than DB end time
	// TODO: Check end time diff first instead of code, threshold of 10s is probably fine
	if report.Code != dbReport.Code {
		log.Info().
			Int("charID", char.ID).Str("code", report.Code).
			Msg("checkWCLogsForCharacterUpdates : new report code")

		// Current report has to be older than stored one, anything else might indicate wclogs deletion
		if report.EndTime > dbReport.EndTime {
			announceNewReport(report, char)
		}
	}

	// Bail if end times are equal
	if report.EndTime == dbReport.EndTime {
		log.Debug().Int("charID", char.ID).Str("code", report.Code).Float64("endTime", report.EndTime).
			Msg("checkWCLogsForCharacterUpdates : no latest report changes")
		// TODO: Check if EndTime is too old and ask if we should continue tracking ?
		return nil
	}

	// Get the latest full report from WCLogs
	fullReport, err := logs[guildID].GetLatestReport(char.Character)
	if err != nil {
		return err
	}

	log.Info().Int("charID", char.ID).Str("code", report.Code).
		Float64("endTime", fullReport.EndTime).Float64("dbEndTime", dbReport.EndTime).
		Int("zoneID", int(fullReport.ZoneID)).Int("size", int(fullReport.Size)).
		Msg("checkWCLogsForCharacterUpdates : latest report changes")

	// Store new report in DB
	err = storeWCLogsLatestReportForCharacterID(db, char.ID, report)
	if err != nil {
		return err
	}

	// TODO: Scan report for more tracked characters

	// Get parses from DB
	dbParses, err := fetchWCLogsParsesForCharacterID(db, char.ID)
	if err != nil || dbParses == nil {
		dbParses = &wclogs.Parses{}
	}

	// Get report zone/size specific parses from WCLogs
	metricRankings, err := logs[guildID].GetMetricRankingsForCharacter(char.Character, fullReport.ZoneID, fullReport.Size)
	if err != nil {
		return err
	}

	// Compare and announce if necessary
	newParses := compareParsesAndAnnounce(metricRankings, dbParses, fullReport, char)

	// Merge parses into DB
	if newParses {
		dbParses.MergeMetricRankings(fullReport.ZoneID, fullReport.Size, metricRankings)
		err = storeWCLogsParsesForCharacterID(db, char.ID, dbParses)
		if err != nil {
			return err
		}
	}

	return nil
}

// announceNewReport formats and sends a new report announcement
func announceNewReport(report *wclogs.ReportMetadata, char *TrackedCharacter) {
	link := "https://classic.warcraftlogs.com/reports/" + report.Code
	content := "New report detected for " + char.Slug() + " : " + link

	_, err := s.ChannelMessageSend(char.ChannelID, content)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send message")
	}
}

// compareParsesAndAnnounce iterates over rankings to find a new parse and announce it there is an improvement
func compareParsesAndAnnounce(metricRankings *wclogs.MetricRankings, dbParses *wclogs.Parses, report *wclogs.Report, char *TrackedCharacter) bool {
	if (*dbParses)[report.ZoneID] == nil || (*dbParses)[report.ZoneID][report.Size] == nil {
		return true
	}

	newParses := false
	for metric, rankings := range *metricRankings {
		for _, ranking := range rankings.Rankings {
			for _, dbRanking := range (*dbParses)[report.ZoneID][report.Size][metric].Rankings {
				if ranking.Encounter.ID == dbRanking.Encounter.ID {
					if ranking.RankPercent > dbRanking.RankPercent {
						log.Info().
							Str("metric", string(metric)).
							Int("charID", char.ID).Float64("oldParse", dbRanking.RankPercent).
							Float64("newParse", ranking.RankPercent).Msg("New parse !")

						announceParse(&ranking, &dbRanking, report, metric, char)

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

// announceParse formats and sends a new parse announcement
func announceParse(ranking *wclogs.Ranking, dbRanking *wclogs.Ranking, report *wclogs.Report, metric wclogs.Metric, char *TrackedCharacter) {
	// TODO: Get player spec and fight ID for proper link
	link := "https://classic.warcraftlogs.com/reports/" + report.Code
	reaction := goodParse[rand.Intn(len(goodParse))]
	if ranking.RankPercent < 50 {
		reaction = badParse[rand.Intn(len(badParse))]
	}

	content := fmt.Sprintf("New parse for %s on %s[%d] !\n"+
		"%s :arrow_right: **%s** [%s] %s\n"+
		"%s", char.Slug(), ranking.Encounter.Name, report.Size,
		fmt.Sprintf("%.2f", dbRanking.RankPercent),
		fmt.Sprintf("%.2f", ranking.RankPercent),
		string(metric), reaction, link)

	_, err := s.ChannelMessageSend(char.ChannelID, content)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send message")
	}
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
