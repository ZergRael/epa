package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"regexp"
	"strconv"
	"time"
)

func ready(s *discordgo.Session, _ *discordgo.Ready) {
	err := s.UpdateGameStatus(0, "/epa")
	if err != nil {
		log.Error().Err(err).Msg("Unable to set game status")
	}
	log.Info().Msg("Bot is up!")
}

func discordMessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		//log.Debug().Msg("User is a bot and being ignored.")
		return
	}

	log.Debug().Str("message", m.Content).Send()

	var err error
	//var channel *discordgo.Channel
	log.Debug().Str("channelID", m.ChannelID).Send()
	_, err = s.State.Channel(m.ChannelID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read channel details")
		_, err = s.UserChannelCreate(m.Author.ID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create channel details")
			return
		}
	}
}

func commandsHandler(s *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if commandFunc, ok := commandsHandlers[interaction.ApplicationCommandData().Name]; ok {
		commandFunc(s, interaction)
	}
}

func addReminder(remindType string, t *time.Time, diffMinutes int, channelID string) error {
	if t == nil {
		_, err := s.ChannelMessageSend(channelID, "Failed to parse time")
		if err != nil {
			return err
		}
		return nil
	}

	at := t.Add(time.Duration(-diffMinutes) * time.Minute)
	in := at.Sub(time.Now())
	if in < 0 {
		_, err := s.ChannelMessageSend(channelID, "Cannot set a reminder in the past")
		if err != nil {
			return err
		}
		return nil
	}

	log.Info().Str("remindType", remindType).Time("t", *t).Dur("in", in).Int("diffMinutes", diffMinutes).Msg("Add reminder")

	time.AfterFunc(in, func() {
		log.Info().Str("remindType", remindType).Time("t", *t).Msg("Timer done")
		_, err := s.ChannelMessageSend(channelID, fmt.Sprintf("@here Reminder %s (%s)", remindType, t.String()))
		if err != nil {
			log.Error().Err(err).Msg("Failed to send message")
		}
	})

	return nil
}

var regexTimeH = regexp.MustCompile(`(\d{1,2})(?:[hH:\-]?(\d{2}))?`)

func parseTime(t string) *time.Time {
	now := time.Now()
	matches := regexTimeH.FindStringSubmatch(t)
	log.Debug().Strs("matches", matches).Send()
	if len(matches) == 0 {
		return nil
	}
	hours, err := strconv.ParseInt(matches[1], 10, 8)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse hours")
		return nil
	}

	var minutes int64
	if matches[2] != "" {
		minutes, err = strconv.ParseInt(matches[2], 10, 8)
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse minutes")
			minutes = 0
		}
	}

	at := time.Date(now.Year(), now.Month(), now.Day(), int(hours), int(minutes), 00, 00, now.Location())
	if at.Before(now) {
		at = at.AddDate(0, 0, 1)
		if at.Before(now) {
			log.Error().Time("at", at).Msg("Failed to generate a proper future date")
			return nil
		}
	}

	return &at
}
