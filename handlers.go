package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func ready(s *discordgo.Session, r *discordgo.Ready) {
	err := s.UpdateGameStatus(0, commandPrefix+"help")
	if err != nil {
		log.Error().Err(err).Msg("Unable to set game status")
	}
	log.Info().Msg("Bot is up!")
}

const commandPrefix = "!"

func discordMessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	log.Debug().Str("message", m.Content).Send()

	if m.Author.Bot {
		//log.Debug().Msg("User is a bot and being ignored.")
		return
	}

	var err error
	var channel *discordgo.Channel
	log.Debug().Str("channelID", m.ChannelID).Send()
	channel, err = s.State.Channel(m.ChannelID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read channel details")
		channel, err = s.UserChannelCreate(m.Author.ID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create channel details")
			return
		}
	}

	// Commands start with "."
	if strings.Index(m.Content, commandPrefix) == 0 {
		cmdArgs := strings.SplitN(m.Content, " ", 2)
		switch cmdArgs[0] {
		case commandPrefix + "help":
			writeHelp(channel)
		case commandPrefix + "zg":
			if len(cmdArgs) > 1 {
				addReminder("ZG", parseTime(cmdArgs[1]), 5, channel)
			}
		case commandPrefix + "ony":
			if len(cmdArgs) > 1 {
				addReminder("ONY", parseTime(cmdArgs[1]), 5, channel)
			}
		case commandPrefix + "rem":
			if len(cmdArgs) > 1 {
				addReminder("REMINDER", parseTime(cmdArgs[1]), 0, channel)
			}
		}
	}
}

func addReminder(remindType string, t *time.Time, diffMinutes int, channel *discordgo.Channel) {
	if t == nil {
		_, err := s.ChannelMessageSend(channel.ID, "Failed to parse time")
		if err != nil {
			log.Error().Err(err).Msg("Failed to send message")
		}
		return
	}

	at := t.Add(time.Duration(-diffMinutes) * time.Minute)
	in := at.Sub(time.Now())
	if in < 0 {
		_, err := s.ChannelMessageSend(channel.ID, "Cannot set a reminder in the past")
		if err != nil {
			log.Error().Err(err).Msg("Failed to send message")
		}
		return
	}

	log.Info().Str("remindType", remindType).Time("t", *t).Dur("in", in).Int("diffMinutes", diffMinutes).Msg("Add reminder")

	time.AfterFunc(in, func() {
		log.Info().Str("remindType", remindType).Time("t", *t).Msg("Timer done")
		_, err := s.ChannelMessageSend(channel.ID, fmt.Sprintf("@here Reminder %s (%s)", remindType, t.String()))
		if err != nil {
			log.Error().Err(err).Msg("Failed to send message")
		}
	})

	_, err := s.ChannelMessageSend(channel.ID, fmt.Sprintf("Reminder set for %s", at))
	if err != nil {
		log.Error().Err(err).Msg("Failed to send message")
	}
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

func writeHelp(channel *discordgo.Channel) {
	content := fmt.Sprintf(`Available commands :
%[1]shelp
%[1]szg 17:53
%[1]sony 18:55`, commandPrefix)
	_, err := s.ChannelMessageSend(channel.ID, content)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send message")
	}
}
