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
