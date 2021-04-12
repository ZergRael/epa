package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

func discordMessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	log.Debug().Str("message", m.Content).Send()

	if m.Author.Bot {
		log.Debug().Msg("User is a bot and being ignored.")
		return
	}

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read channel details")
		return
	}

	if channel.Type == discordgo.ChannelTypeDM {
		log.Debug().Msg("DirectMessage")
	}
}
