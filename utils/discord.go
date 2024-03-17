package utils

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type DiscordResponse interface {
	Respond() error
	Edit() (*discordgo.Message, error)
	SetTitle(string) DiscordResponse
	SetTitlef(string, ...interface{}) DiscordResponse
	SetInfo(string) DiscordResponse
	SetWarning(string) DiscordResponse
	SetSuccess(string) DiscordResponse
	SetError(error) DiscordResponse
	SetDescription(string) DiscordResponse
}

type discordResponse struct {
	embed *discordgo.MessageEmbed
	s     *discordgo.Session
	i     *discordgo.InteractionCreate
}

func NewDiscordResponse(s *discordgo.Session, i *discordgo.InteractionCreate) DiscordResponse {
	return &discordResponse{
		s:     s,
		i:     i,
		embed: &discordgo.MessageEmbed{},
	}
}

func (dr *discordResponse) SetDescription(d string) DiscordResponse {
	dr.embed.Description = d

	return dr
}

func (dr *discordResponse) SetDescriptionf(d string, args ...interface{}) DiscordResponse {
	dr.embed.Description = fmt.Sprintf(d, args...)

	return dr
}

func (dr *discordResponse) SetTitle(t string) DiscordResponse {
	dr.embed.Title = t

	return dr
}

func (dr *discordResponse) SetTitlef(t string, args ...interface{}) DiscordResponse {
	dr.embed.Title = fmt.Sprintf(t, args...)

	return dr
}

func (dr *discordResponse) SetInfo(desc string) DiscordResponse {
	dr.embed.Color = 0x0c5460
	dr.SetDescription(desc)

	return dr
}

func (dr *discordResponse) SetSuccess(desc string) DiscordResponse {
	dr.embed.Color = 0x155724
	dr.SetDescription(desc)

	return dr
}

func (dr *discordResponse) SetWarning(desc string) DiscordResponse {
	dr.embed.Color = 0x856404
	dr.SetDescription(desc)

	return dr
}

func (dr *discordResponse) SetError(err error) DiscordResponse {
	dr.embed.Color = 0x721c24
	dr.SetDescriptionf("```%s```", err.Error())

	return dr
}

func (dr *discordResponse) Respond() error {
	return dr.s.InteractionRespond(dr.i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:  discordgo.MessageFlagsEphemeral,
			Embeds: []*discordgo.MessageEmbed{dr.embed},
		},
	})
}

func (dr *discordResponse) Edit() (*discordgo.Message, error) {
	return dr.s.InteractionResponseEdit(dr.i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{dr.embed},
	})
}
