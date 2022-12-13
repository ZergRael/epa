package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zergrael/epa/wclogs"
)

var trackedCharacters map[string][]*TrackedCharacter
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
	log.Debug().Str("guildID", guildID).Msg("instantiateWCLogsForGuild")
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
		trackedCharacters = make(map[string][]*TrackedCharacter)
	}
	trackedCharacters[guildID], err = fetchWCLogsTrackedCharacters(db, guildID)
	if err != nil {
		log.Warn().Err(err).Msg("No currently tracked characters")
		characters := make([]*TrackedCharacter, 0)
		trackedCharacters[guildID] = characters
	}

	// Setup tracking timer
	setupWCLogsTicker(guildID)
}

// destroyWCLogsForGuild unregisters WCLogs credentials, deletes live tracks and remove timers & tickers
func destroyWCLogsForGuild(guildID string) {
	log.Debug().Str("guildID", guildID).Msg("destroyWCLogsForGuild")
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
	log.Debug().Str("guildID", guildID).Msg("registerWarcraftLogs")
	creds := &wclogs.Credentials{ClientID: clientID, ClientSecret: clientSecret}
	w := wclogs.New(creds, wclogs.Classic, nil)
	if !w.Connect() {
		return "These API credentials cannot be used"
	}

	log.Info().Str("guildID", guildID).Msg("WCLogs instance successful")
	logs[guildID] = w

	if trackedCharacters == nil {
		trackedCharacters = make(map[string][]*TrackedCharacter)
	}
	var err error
	trackedCharacters[guildID], err = fetchWCLogsTrackedCharacters(db, guildID)
	if err != nil {
		log.Warn().Err(err).Msg("No currently tracked characters")
		characters := make([]*TrackedCharacter, 0)
		trackedCharacters[guildID] = characters
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
	log.Debug().Str("guildID", guildID).Msg("unregisterWarcraftLogs")
	if logs[guildID] == nil {
		return "No stored credentials"
	}

	destroyWCLogsForGuild(guildID)

	return "Unregister successful"
}

// trackCharacter tries to add a regular performance track on a specific character
func trackCharacter(name, server, region, guildID, channelID string) string {
	log.Debug().Str("name", name).Str("server", server).Str("region", "region").
		Str("guildID", guildID).Str("channelID", channelID).Msg("trackCharacter")
	if logs[guildID] == nil {
		return "Missing WarcraftLogs credentials setup"
	}

	char, err := logs[guildID].GetCharacter(name, server, region)
	if err != nil || char == nil {
		log.Error().Str("slug", char.Slug()).Err(err).Msg("GetCharacterID failed")
		return "Failed to track " + char.Slug() + " : character not found !"
	}

	for idx, c := range trackedCharacters[guildID] {
		if c.Character.ID == char.ID {
			// Remove currently tracked character, it will be added back again later
			// hackish way to allow update to announce channel
			trackedCharacters[guildID] = append(trackedCharacters[guildID][:idx], trackedCharacters[guildID][idx+1:]...)
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

	trackedChar := &TrackedCharacter{Character: char, ChannelID: channelID}
	trackedCharacters[guildID] = append(trackedCharacters[guildID], trackedChar)
	err = storeWCLogsTrackedCharacters(db, guildID, trackedCharacters[guildID])
	if err != nil {
		log.Error().Str("slug", char.Slug()).Int("charID", char.ID).
			Err(err).Msg("storeWCLogsTrackedCharacters failed")
		return "Failed to track " + char.Slug()
	}

	// Record parses in goroutine as it may be too slow for discord response
	go func() {
		_, err := getAndStoreAllWCLogsParsesForCharacter(guildID, trackedChar)
		if err != nil {
			log.Error().Err(err).Str("slug", char.Slug()).Msg("Failed to get all parses")
		}
	}()

	log.Info().Str("slug", char.Slug()).Msg("Track successful")
	return char.Slug() + " is now tracked"
}

// untrackCharacter removes a character for current tracking
func untrackCharacter(name, server, region, guildID string) string {
	log.Debug().Str("name", name).Str("server", server).Str("region", "region").
		Str("guildID", guildID).Msg("untrackCharacter")
	if logs[guildID] == nil {
		return "Missing WarcraftLogs credentials setup"
	}

	char, err := logs[guildID].GetCharacter(name, server, region)
	if err != nil {
		log.Error().Str("slug", char.Slug()).Err(err).Msg("GetCharacterID failed")
		return "Failed to untrack " + char.Slug() + " : character not found !"
	}

	for idx, c := range trackedCharacters[guildID] {
		if c.Character.ID == char.ID {
			trackedCharacters[guildID] = append(trackedCharacters[guildID][:idx], trackedCharacters[guildID][idx+1:]...)
			log.Info().Str("slug", char.Slug()).Msg("Untrack successful")
			return char.Slug() + " is not tracked anymore"
		}
	}

	log.Warn().Str("slug", char.Slug()).Err(err).Msg("Not tracked")
	return char.Slug() + " was not tracked"
}

// currentParses returns cached performances for a character
func currentParses(name, server, region, guildID string) (map[string][]string, string) {
	log.Debug().Str("name", name).Str("server", server).Str("region", "region").
		Str("guildID", guildID).Msg("currentParses")
	if logs[guildID] == nil {
		return nil, "Missing WarcraftLogs credentials setup"
	}

	char, err := logs[guildID].GetCharacter(name, server, region)
	if err != nil {
		log.Error().Str("slug", char.Slug()).Err(err).Msg("GetCharacterID failed")
		return nil, "Failed to get parses for " + char.Slug() + " : character not found !"
	}

	dbParses, err := fetchWCLogsParsesForCharacterID(db, char.ID)
	if err != nil || dbParses == nil {
		log.Warn().Str("slug", char.Slug()).Err(err).Msg("Not tracked")
		return nil, char.Slug() + " is not tracked"
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
	return content, ""
}

// getTrackedCharacters returns an array of all known and tracked characters for a guildID
func getTrackedCharacters(guildID string) ([]*TrackedCharacter, string) {
	log.Debug().Str("guildID", guildID).Msg("listTrackedCharacters")
	if logs[guildID] == nil {
		return nil, "Missing WarcraftLogs credentials setup"
	}

	return trackedCharacters[guildID], ""
}

// getAndStoreAllWCLogsParsesForCharacter gets all available parses for a character and stores them in db
func getAndStoreAllWCLogsParsesForCharacter(guildID string, char *TrackedCharacter) (*wclogs.Parses, error) {
	log.Debug().Str("guildID", guildID).Msg("getAndStoreAllWCLogsParsesForCharacter")
	parses, err := logs[guildID].GetParsesForCharacter(char.Character)
	if err != nil {
		return nil, err
	}

	err = storeWCLogsParsesForCharacterID(db, char.ID, parses)
	if err != nil {
		return nil, err
	}

	return parses, nil
}

// checkWCLogsForCharacterUpdates gets the latest report metadata and updates parses if necessary
func checkWCLogsForCharacterUpdates(guildID string, char *TrackedCharacter) error {
	log.Debug().Int("charID", char.ID).Str("slug", char.Slug()).Msg("checkWCLogsForCharacterUpdates")
	// Get the latest report metadata from DB
	dbReport, err := fetchWCLogsLatestReportForCharacterID(db, char.ID)
	if err != nil || dbReport == nil {
		// Missing initial report
		log.Debug().Int("charID", char.ID).Str("slug", char.Slug()).Msg("Missing initial report")
		report, err := logs[guildID].GetLatestReportMetadata(char.Character)
		if err != nil {
			return err
		}
		return storeWCLogsLatestReportForCharacterID(db, char.ID, report)
	}

	// Get the latest report metadata from WCLogs
	report, err := logs[guildID].GetLatestReportMetadata(char.Character)
	if err != nil {
		return err
	}

	// Bail if end times are equal at second precision
	if report.EndTime.Unix() == dbReport.EndTime.Unix() {
		log.Debug().Int("charID", char.ID).Str("slug", char.Slug()).
			Str("code", report.Code).Int64("endTime", report.EndTime.UnixMilli()).
			Msg("No end time report changes")
		// TODO: Check if EndTime is too old and ask if we should continue tracking ?
		return nil
	}

	// Get parses from DB
	dbParses, err := fetchWCLogsParsesForCharacterID(db, char.ID)
	if err != nil || dbParses == nil {
		// Missing initial parses
		log.Debug().Int("charID", char.ID).Str("slug", char.Slug()).Msg("Missing initial parses")
		dbParses, err = getAndStoreAllWCLogsParsesForCharacter(guildID, char)
		if err != nil {
			return err
		}
	}

	// Announce new report if code diff and end time is later than DB end time
	if report.Code != dbReport.Code {
		log.Info().
			Int("charID", char.ID).Str("slug", char.Slug()).Str("code", report.Code).
			Msg("New report code")

		// Current report has to be older than stored one, anything else might indicate wclogs deletion
		if report.EndTime.After(dbReport.EndTime) {
			announceNewReport(report, char)
		}
	}

	// Get the latest full report from WCLogs
	fullReport, err := logs[guildID].GetLatestReport(char.Character)
	if err != nil {
		return err
	}

	log.Info().Int("charID", char.ID).Str("slug", char.Slug()).Str("code", report.Code).
		Int64("endTime", fullReport.EndTime.UnixMilli()).Int64("dbEndTime", dbReport.EndTime.UnixMilli()).
		Int("zoneID", int(fullReport.ZoneID)).Int("size", int(fullReport.Size)).
		Msg("Latest report changes")

	// Store new report in DB
	err = storeWCLogsLatestReportForCharacterID(db, char.ID, report)
	if err != nil {
		return err
	}

	// TODO: Scan report for more tracked characters

	// Get report zone/size specific parses from WCLogs
	metricRankings, err := logs[guildID].GetMetricRankingsForCharacter(char.Character, fullReport.ZoneID, fullReport.Size)
	if err != nil {
		return err
	}

	// Compare and announce if necessary
	compareParsesAndAnnounce(metricRankings, dbParses, fullReport, char)

	// Merge parses into DB
	dbParses.MergeMetricRankings(fullReport.ZoneID, fullReport.Size, metricRankings)
	return storeWCLogsParsesForCharacterID(db, char.ID, dbParses)
}

// announceNewReport formats and sends a new report announcement
func announceNewReport(report *wclogs.ReportMetadata, char *TrackedCharacter) {
	log.Debug().Str("code", report.Code).Str("slug", char.Slug()).Msg("announceNewReport")
	link := "https://classic.warcraftlogs.com/reports/" + report.Code

	_, err := s.ChannelMessageSendEmbed(char.ChannelID, &discordgo.MessageEmbed{
		Type:  discordgo.EmbedTypeRich,
		URL:   link,
		Title: "New report for " + char.Slug(),
		Color: 0x904400,
		Footer: &discordgo.MessageEmbedFooter{
			Text: link,
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send message")
	}
}

// compareParsesAndAnnounce iterates over rankings to find a new parse and announce it there is an improvement
func compareParsesAndAnnounce(metricRankings *wclogs.MetricRankings, dbParses *wclogs.Parses, report *wclogs.Report, char *TrackedCharacter) {
	log.Debug().Str("code", report.Code).Str("slug", char.Slug()).Msg("compareParsesAndAnnounce")
	if (*dbParses)[report.ZoneID] == nil || (*dbParses)[report.ZoneID][report.Size] == nil {
		return
	}

	for metric, rankings := range *metricRankings {
		for _, ranking := range rankings.Rankings {
			for _, dbRanking := range (*dbParses)[report.ZoneID][report.Size][metric].Rankings {
				if ranking.Encounter.ID == dbRanking.Encounter.ID {
					if ranking.RankPercent-dbRanking.RankPercent > 0.1 {
						log.Info().
							Str("metric", string(metric)).
							Int("charID", char.ID).Float64("oldParse", dbRanking.RankPercent).
							Float64("newParse", ranking.RankPercent).Msg("New parse !")

						announceParse(&ranking, &dbRanking, report, metric, char)
					}
				}
			}
		}
	}
}

// announceParse formats and sends a new parse announcement
func announceParse(ranking *wclogs.Ranking, dbRanking *wclogs.Ranking, report *wclogs.Report, metric wclogs.Metric, char *TrackedCharacter) {
	log.Debug().Str("code", report.Code).Str("slug", char.Slug()).Msg("announceParse")
	// TODO: Get player spec and fight ID for proper link
	link := "https://classic.warcraftlogs.com/reports/" + report.Code
	reaction := goodParse[rand.Intn(len(goodParse))]
	if ranking.RankPercent < 50 {
		reaction = badParse[rand.Intn(len(badParse))]
	}

	content := fmt.Sprintf("New parse for %s on %s(%d) : %s :arrow_right: **%s** [%s] %s\n"+
		"%s", char.Slug(), ranking.Encounter.Name, report.Size,
		fmt.Sprintf("%.2f", dbRanking.RankPercent), fmt.Sprintf("%.2f", ranking.RankPercent),
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
				log.Debug().Str("guildID", guildID).Msg("Tick")
				for _, char := range trackedCharacters[guildID] {
					err := checkWCLogsForCharacterUpdates(guildID, char)
					if err != nil {
						log.Error().Err(err).Msg("Failed to checkWCLogsForCharacterUpdates in wclogs ticker")
					}
				}
			}
		}
	}(guildID)
}
