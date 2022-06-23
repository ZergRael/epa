package main

import (
	"encoding/json"
	"epa/wclogs"
	"flag"
	"github.com/rs/zerolog"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/buntdb"
)

// Application flags
var (
	BotToken = flag.String("token", "", "Bot access token")
)

// Global variables
var (
	s    *discordgo.Session
	db   *buntdb.DB
	logs map[string]*wclogs.WCLogs
)

// FIXME: Global commands register slowly, stick to guild specific commands for now
const globalCommands = false

func init() {
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	if *BotToken == "" {
		log.Fatal().Msg("Missing --token flag")
	}

	var err error
	s, err = discordgo.New("Bot " + *BotToken)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid bot parameters")
	}

	logs = make(map[string]*wclogs.WCLogs)
}

func main() {
	// Database
	var err error
	db, err = buntdb.Open("data.db")
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot open database")
	}
	defer func(db *buntdb.DB) {
		err := db.Close()
		if err != nil {
			log.Error().Err(err).Msg("Failed to properly close database")
		}
	}(db)

	// Discordgo handlers
	s.AddHandler(ready)
	s.AddHandler(guildCreate)
	s.AddHandler(guildDelete)
	s.AddHandler(discordMessageHandler)
	s.AddHandler(commandsHandler)

	s.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	// Discordgo startup
	err = s.Open()
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot open the session")
	}
	defer func(s *discordgo.Session) {
		err := s.Close()
		if err != nil {
			log.Error().Err(err).Msg("Failed to properly close session")
		}
	}(s)

	log.Debug().Str("id", s.State.User.ID).Msg("Session opened for bot")

	if globalCommands {
		addCommands("")
	}

	log.Info().Msg("Invite the bot to your server with https://discordapp.com/oauth2/authorize?client_id=" + s.State.User.ID + "&scope=bot%20applications.commands")

	// Bot run loop
	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt)
	<-stop

	// FIXME: Figure out if we need to remove commands on shutdown
	if globalCommands {
		removeCommands("")
	}

	log.Info().Msg("Graceful shutdown")
}

func instantiateWCLogsForGuild(guildID string) {
	// WCLogs credentials
	creds := &wclogs.Credentials{}
	err := db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("warcraft-logs-" + guildID)
		if err != nil {
			return err
		}

		err = json.Unmarshal([]byte(val), creds)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Debug().Err(err).Str("guildID", guildID).Msg("Cannot read WCLogs credentials for guild")
	}

	// Check WCLogs credentials
	if creds.ClientID != "" && creds.ClientSecret != "" {
		w := wclogs.NewWCLogs(creds)
		if w.Check() {
			log.Info().Str("guildID", guildID).Msg("WCLogs instance successful")
			logs[guildID] = w
			return
		}
	}

	log.Warn().Str("guildID", guildID).Msg("Failed to reuse credentials for guild")
}

func storeWCLogsCredentials(guildID string, creds *wclogs.Credentials) error {
	bytes, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	err = db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set("warcraft-logs-"+guildID, string(bytes), nil)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func trackWCLogsCharacter(guildID string, charID int) {
	_, err := logs[guildID].CheckParsesForCharacter(charID)
	if err != nil {
		return
	}

	// TODO: store parses
	// TODO: set regular check timer

	//err = db.Update(func(tx *buntdb.Tx) error {
	//
	//	val, err := tx.Get("warcraft-logs-characters-" + guildID)
	//	if err == nil {
	//
	//	}
	//
	//	_, _, err := tx.Set("warcraft-logs-characters-"+guildID, string(bytes), nil)
	//	if err != nil {
	//		return err
	//	}
	//	return nil
	//})
	//if err != nil {
	//	return err
	//}
}
