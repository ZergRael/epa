package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "epa",
		Description: "Bot status",
	},
	{
		Name:        "ping",
		Description: "Send a ping to the bot",
	},
	{
		Name:        "reminder",
		Description: "Add a reminder for a specific time",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "time",
				Description: "Example: 21:00",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "reason",
				Description: "Reminder reason",
				Required:    false,
			},
		},
	},
}

var commandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"epa": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "That's me alright",
				Flags:   uint64(discordgo.MessageFlagsEphemeral),
			},
		})
	},

	"ping": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Pong !",
			},
		})
	},

	"reminder": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		remindAt := parseTime(i.ApplicationCommandData().Options[0].StringValue())
		remindReason := ""

		if len(i.ApplicationCommandData().Options) >= 2 {
			remindReason = i.ApplicationCommandData().Options[1].StringValue()
		}

		responseContent := fmt.Sprintf("I will remind you at %s", remindAt)

		err := addReminder(remindReason, remindAt, 0, i.ChannelID)
		if err != nil {
			log.Err(err)
			responseContent = fmt.Sprintf("Failed to set reminder : %v", err)
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			// Ignore type for now, we'll discuss them in "responses" part
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: responseContent,
			},
		})
	},
}

func addCommands(guildID string) {
	log.Debug().Str("guildID", guildID).Msg("Adding commands...")

	for _, v := range commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, v)
		if err != nil {
			log.Fatal().Err(err).Msgf("Cannot create : %v", v.Name)
		}
		log.Debug().Str("name", cmd.Name).Str("id", cmd.ID).Str("guild", guildID).Msg("Added command")
	}
}

func removeCommands(guildID string) {
	log.Debug().Str("guildID", guildID).Msg("Removing commands...")

	registeredCommands, err := s.ApplicationCommands(s.State.User.ID, guildID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get current commands")
		return
	}

	for _, v := range registeredCommands {
		err := s.ApplicationCommandDelete(s.State.User.ID, guildID, v.ID)
		if err != nil {
			log.Fatal().Err(err).Msgf("Cannot delete : %v", v.Name)
		}
	}
}
