package main

import (
	"flag"
	"github.com/rs/zerolog"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

var (
	BotToken = flag.String("token", "", "Bot access token")
)

var s *discordgo.Session

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
	s.AddHandler(ready)

	s.AddHandler(discordMessageHandler)

	s.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	err := s.Open()
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot open the session")
	}
	defer s.Close()

	bot, err := s.User("@me")
	if err != nil {
		log.Error().Err(err).Msg("Error obtaining @me account details")
	}

	log.Info().Msg("Invite the bot to your server with https://discordapp.com/oauth2/authorize?client_id=" + bot.ID + "&scope=bot")

	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Info().Msg("Graceful shutdown")
}
