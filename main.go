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
	s.AddHandler(commandsHandler)

	s.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	err := s.Open()
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot open the session")
	}
	defer s.Close()

	log.Debug().Msgf("Session opened for bot ID : %s", s.State.User.ID)

	log.Info().Msg("Adding commands...")
	for _, v := range commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, "", v)
		if err != nil {
			log.Fatal().Err(err).Msgf("Cannot create : %v", v.Name)
		}
		log.Debug().Msgf("Added command : %s [%s]", cmd.Name, cmd.ID)
	}

	log.Info().Msg("Invite the bot to your server with https://discordapp.com/oauth2/authorize?client_id=" + s.State.User.ID + "&scope=bot%20applications.commands")

	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt)
	<-stop

	log.Info().Msg("Removing commands...")
	registeredCommands, err := s.ApplicationCommands(s.State.User.ID, "")
	for _, v := range registeredCommands {
		err := s.ApplicationCommandDelete(s.State.User.ID, "", v.ID)
		if err != nil {
			log.Fatal().Err(err).Msgf("Cannot delete : %v", v.Name)
		}
	}

	log.Info().Msg("Graceful shutdown")
}
