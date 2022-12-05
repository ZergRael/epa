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
		Name:        "parses",
		Description: "Show current parses for a specific character",
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
				strconv.Itoa(len(trackedCharacters[i.GuildID])) +
				" characters, see /track-character command to add more."
		} else {
			response += "WarcraftLogs is disabled, see /register-warcraftlogs command as an admin"
		}

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

		if err != nil {
			log.Error().Err(err).Msg("/epa command response failed")
		}
	},

	"register-warcraftlogs": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		clientID := i.ApplicationCommandData().Options[0].StringValue()
		clientSecret := i.ApplicationCommandData().Options[1].StringValue()

		response := registerWarcraftLogs(clientID, clientSecret, i.GuildID)

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

		if err != nil {
			log.Error().Err(err).Msg("/register-warcraftlogs command response failed")
		}
	},
	"unregister-warcraftlogs": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		response := unregisterWarcraftLogs(i.GuildID)

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

		if err != nil {
			log.Error().Err(err).Msg("/unregister-warcraftlogs command response failed")
		}
	},

	"track-character": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		char := i.ApplicationCommandData().Options[0].StringValue()
		server := i.ApplicationCommandData().Options[1].StringValue()
		region := i.ApplicationCommandData().Options[2].StringValue()
		channel := i.ChannelID
		if len(i.ApplicationCommandData().Options) > 3 {
			channel = i.ApplicationCommandData().Options[3].ChannelValue(s).ID
		}

		response := trackCharacter(char, server, region, i.GuildID, channel)

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
			},
		})

		if err != nil {
			log.Error().Err(err).Msg("/track-character command response failed")
		}
	},
	"untrack-character": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		char := i.ApplicationCommandData().Options[0].StringValue()
		server := i.ApplicationCommandData().Options[1].StringValue()
		region := i.ApplicationCommandData().Options[2].StringValue()

		response := untrackCharacter(char, server, region, i.GuildID)

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
			},
		})

		if err != nil {
			log.Error().Err(err).Msg("/untrack-character command response failed")
		}
	},
	"parses": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		char := i.ApplicationCommandData().Options[0].StringValue()
		server := i.ApplicationCommandData().Options[1].StringValue()
		region := i.ApplicationCommandData().Options[2].StringValue()

		data := &discordgo.InteractionResponseData{
			Content: "Failed to get parses",
		}

		content := currentParses(char, server, region, i.GuildID)
		if content != nil {
			var fields []*discordgo.MessageEmbedField
			for header, rows := range content {
				var value string
				for _, row := range rows {
					value += row + "\n"
				}
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   header,
					Value:  value,
					Inline: true,
				})
			}
			data = &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Type:   discordgo.EmbedTypeRich,
						Fields: fields,
					},
				},
			}
		}

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: data,
		})

		if err != nil {
			log.Error().Err(err).Msg("/untrack-character command response failed")
		}
	},
	"list-tracked-characters": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		response := listTrackedCharacters(i.GuildID)

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
			},
		})

		if err != nil {
			log.Error().Err(err).Msg("/list-tracked-characters command response failed")
		}
	},
}

func addCommands(guildID string) {
	log.Debug().Str("guildID", guildID).Msg("Adding commands...")

	_, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, guildID, commands)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot create commands")
	}
	log.Debug().Str("guild", guildID).Msg("Added commands")
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
