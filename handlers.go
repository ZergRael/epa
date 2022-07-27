package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

func ready(s *discordgo.Session, ready *discordgo.Ready) {
	err := s.UpdateGameStatus(0, "/epa")
	if err != nil {
		log.Error().Err(err).Msg("Unable to set game status")
	}

	log.Info().Int("guilds", len(ready.Guilds)).Msg("Bot is up!")
}

func guildCreate(_ *discordgo.Session, guild *discordgo.GuildCreate) {
	log.Info().Str("guildID", guild.ID).Msg("Added to guild")

	if !globalCommands {
		// Add guild specific commands on guild join
		addCommands(guild.ID)
	}

	instantiateWCLogsForGuild(guild.ID)
}

func guildDelete(_ *discordgo.Session, guild *discordgo.GuildDelete) {
	log.Info().Str("guildID", guild.ID).Msg("Removed from guild")
	// Not necessary to remove commands here, we have already lost permissions

	destroyWCLogsForGuild(guild.ID)
}

func commandsHandler(s *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if commandFunc, ok := commandsHandlers[interaction.ApplicationCommandData().Name]; ok {
		commandFunc(s, interaction)
	}
}
