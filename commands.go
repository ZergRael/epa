package main

import (
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

var falsePointer = false

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "epa",
		Description: "Display configuration & information about the bot",
	},
	{
		Name:              "register-warcraftlogs",
		Description:       "Setup credentials for WarcraftLogs API",
		DefaultPermission: &falsePointer,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "client-id",
				Description: "Client ID from WCLogs API",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "client-secret",
				Description: "Client secret from WCLogs API",
				Required:    true,
			},
		},
	},
	{
		Name:              "unregister-warcraftlogs",
		Description:       "Erase WarcraftLogs API credentials",
		DefaultPermission: &falsePointer,
	},
	{
		Name:        "track-character",
		Description: "Add WCLogs parses tracking on a specific character",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "character",
				Description: "Character name",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "server",
				Description: "Character server",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "region",
				Description: "Character server region (EU/US)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Channel used to announce updates",
				Required:    false,
			},
		},
	},
	{
		Name:        "untrack-character",
		Description: "Remove WCLogs parses tracking on a specific character",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "character",
				Description: "Character name",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "server",
				Description: "Character server",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "region",
				Description: "Character server region (EU/US)",
				Required:    true,
			},
		},
	},
	{
		Name:        "list-tracked-characters",
		Description: "List WCLogs parses tracked characters",
	},
}

var commandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"epa": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		response := "Hello there\n"
		if logs[i.GuildID] != nil {
			response += "WarcraftLogs engine is running, currently tracking " +
				strconv.Itoa(len(*trackedCharacters[i.GuildID])) +
				" characters, see /track-character command to add more."
		} else {
			response += "WarcraftLogs is disabled, see /register-warcraftlogs command as an admin"
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   uint64(discordgo.MessageFlagsEphemeral),
			},
		})
	},

	"register-warcraftlogs": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		clientID := i.ApplicationCommandData().Options[0].StringValue()
		clientSecret := i.ApplicationCommandData().Options[1].StringValue()

		response := handleRegisterWarcraftLogs(clientID, clientSecret, i.GuildID)

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   uint64(discordgo.MessageFlagsEphemeral),
			},
		})
	},
	"unregister-warcraftlogs": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		response := handleUnregisterWarcraftLogs(i.GuildID)

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   uint64(discordgo.MessageFlagsEphemeral),
			},
		})
	},

	"track-character": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		char := i.ApplicationCommandData().Options[0].StringValue()
		server := i.ApplicationCommandData().Options[1].StringValue()
		region := i.ApplicationCommandData().Options[2].StringValue()
		channel := i.ChannelID
		if len(i.ApplicationCommandData().Options) > 3 {
			channel = i.ApplicationCommandData().Options[3].ChannelValue(s).ID
		}

		response := handleTrackCharacter(char, server, region, i.GuildID, channel)

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
			},
		})
	},
	"untrack-character": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		char := i.ApplicationCommandData().Options[0].StringValue()
		server := i.ApplicationCommandData().Options[1].StringValue()
		region := i.ApplicationCommandData().Options[2].StringValue()

		response := handleUntrackCharacter(char, server, region, i.GuildID)

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
			},
		})
	},
	"list-tracked-characters": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		response := handleListTrackedCharacters(i.GuildID)

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
			},
		})
	},
}

func addCommands(guildID string) {
	log.Debug().Str("guildID", guildID).Msg("Adding commands...")

	cmds, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, guildID, commands)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot create commands")
	}
	log.Debug().Str("guild", guildID).Interface("commands", cmds).Msg("Added commands")
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
