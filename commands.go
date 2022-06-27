package main

import (
	"epa/wclogs"
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

var falsePointer = false

var commands = []*discordgo.ApplicationCommand{
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
		},
	},
}

var commandsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"ping": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Pong !",
			},
		})
	},

	"register-warcraftlogs": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		clientID := i.ApplicationCommandData().Options[0].StringValue()
		clientSecret := i.ApplicationCommandData().Options[1].StringValue()

		creds := &wclogs.Credentials{ClientID: clientID, ClientSecret: clientSecret}
		w := wclogs.NewWCLogs(creds)
		response := "These API credentials cannot be used"
		if w.Check() {
			logs[i.GuildID] = w
			log.Info().Str("guildID", i.GuildID).Msg("WCLogs instance successful")
			response = "Congrats, API credentials are valid"
			err := storeWCLogsCredentials(i.GuildID, creds)
			if err != nil {
				response = "API credentials are valid, but I failed to store them"
			}
		}

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

		response := "Missing WarcraftLogs credentials setup"
		if logs[i.GuildID] != nil {
			response = "Failed to track " + char + "-" + server + "[" + region + "]"
			id, err := logs[i.GuildID].GetCharacterID(char, server, region)
			if err == nil {
				response = char + "-" + server + "[" + region + "] (" + strconv.Itoa(id) + ") is now tracked"
				trackWCLogsCharacter(i.GuildID, id)
			}
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   uint64(discordgo.MessageFlagsEphemeral),
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
