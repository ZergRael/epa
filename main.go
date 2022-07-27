package main

import (
	"flag"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/buntdb"
	"github.com/zergrael/epa/wclogs"
)

// s is global discord session
var s *discordgo.Session

// db is global database handler
var db *buntdb.DB

// logs is WCLogs handler for each guildID
var logs map[string]*wclogs.WCLogs

// FIXME: Global commands register slowly, stick to guild specific commands for now
const globalCommands = false

func init() {
	// botToken is Discord bot access token
	var botToken string
	flag.StringVar(&botToken, "token", lookupEnvOrString("DISCORD_BOT_TOKEN", ""), "Bot discord access token")

	// debug forces debug messages output
	var debug bool
	flag.BoolVar(&debug, "debug", lookupEnvOrBool("DEBUG", false), "Output debug messages")

	flag.Parse()

	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).Level(level)

	if botToken == "" {
		log.Fatal().Msg("Missing --token flag / DISCORD_BOT_TOKEN env variable")
	}

	var err error
	s, err = discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid bot parameters")
	}

	logs = make(map[string]*wclogs.WCLogs)
}

func main() {
	// Database

	var err error
	db, err = buntdb.Open("storage/data.db")
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot open database")
	}
	defer func(db *buntdb.DB) {
		err := db.Close()
		if err != nil {
			log.Error().Err(err).Msg("Failed to properly close database")
		}
	}(db)

	err = upgradeDatabaseIfNecessary(db)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to apply database migrations")
	}

	// Discordgo handlers

	s.AddHandler(ready)
	s.AddHandler(guildCreate)
	s.AddHandler(guildDelete)
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

	if s.State == nil {
		log.Fatal().Msg("Failed to get session state")
	}

	log.Debug().Str("id", s.State.User.ID).Msg("Session opened for bot")

	if globalCommands {
		addCommands("")
	}

	log.Info().Msg("Invite the bot to your server with https://discordapp.com/oauth2/authorize?client_id=" + s.State.User.ID + "&scope=bot%20applications.commands")

	// Bot run loop

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	if globalCommands {
		removeCommands("")
	}

	log.Info().Msg("Graceful shutdown")
}

// lookupEnvOrString returns key environment variable or defaultVal
func lookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}

	return defaultVal
}

// lookupEnvOrBool returns key environment variable or defaultVal
func lookupEnvOrBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		return strings.ToLower(val) == "true" || val == "1"
	}

	return defaultVal
}
