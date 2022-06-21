package main

import (
	"flag"
	"github.com/rs/zerolog"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/buntdb"
)

var (
	BotToken = flag.String("token", "", "Bot access token")
)

var s *discordgo.Session

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
}

func main() {
	// Database
	db, err := buntdb.Open("data.db")
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
