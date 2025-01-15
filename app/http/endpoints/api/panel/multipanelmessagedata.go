package api

import (
	"context"
	"math"

	"github.com/jadevelopmentgrp/Tickets-Dashboard/botcontext"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/utils/types"
	database "github.com/jadevelopmentgrp/Tickets-Database"
	"github.com/rxdn/gdl/objects/channel/embed"
	"github.com/rxdn/gdl/objects/interaction/component"
	"github.com/rxdn/gdl/rest"
	"github.com/rxdn/gdl/utils"
)

type multiPanelMessageData struct {
	ChannelId uint64

	SelectMenu            bool
	SelectMenuPlaceholder *string

	Embed *embed.Embed
}

func multiPanelIntoMessageData(panel database.MultiPanel) multiPanelMessageData {
	return multiPanelMessageData{
		ChannelId: panel.ChannelId,

		SelectMenu:            panel.SelectMenu,
		SelectMenuPlaceholder: panel.SelectMenuPlaceholder,
		Embed:                 types.NewCustomEmbed(panel.Embed.CustomEmbed, panel.Embed.Fields).IntoDiscordEmbed(),
	}
}

func (d *multiPanelMessageData) send(ctx *botcontext.BotContext, panels []database.Panel) (uint64, error) {
	d.Embed.SetFooter("Tickets by jaDevelopment", "https://avatars.githubusercontent.com/u/142818403")

	var components []component.Component
	if d.SelectMenu {
		options := make([]component.SelectOption, len(panels))
		for i, panel := range panels {
			emoji := types.NewEmoji(panel.EmojiName, panel.EmojiId).IntoGdl()

			options[i] = component.SelectOption{
				Label: panel.ButtonLabel,
				Value: panel.CustomId,
				Emoji: emoji,
			}
		}

		var placeholder string
		if d.SelectMenuPlaceholder == nil {
			placeholder = "Select a topic..."
		} else {
			placeholder = *d.SelectMenuPlaceholder
		}

		components = []component.Component{
			component.BuildActionRow(
				component.BuildSelectMenu(
					component.SelectMenu{
						CustomId:    "multipanel",
						Options:     options,
						Placeholder: placeholder,
						MinValues:   utils.IntPtr(1),
						MaxValues:   utils.IntPtr(1),
						Disabled:    false,
					}),
			),
		}
	} else {
		buttons := make([]component.Component, len(panels))
		for i, panel := range panels {
			emoji := types.NewEmoji(panel.EmojiName, panel.EmojiId).IntoGdl()

			buttons[i] = component.BuildButton(component.Button{
				Label:    panel.ButtonLabel,
				CustomId: panel.CustomId,
				Style:    component.ButtonStyle(panel.ButtonStyle),
				Emoji:    emoji,
				Disabled: panel.Disabled,
			})
		}

		var rows []component.Component
		for i := 0; i <= int(math.Ceil(float64(len(buttons)/5))); i++ {
			lb := i * 5
			ub := lb + 5

			if ub >= len(buttons) {
				ub = len(buttons)
			}

			if lb >= ub {
				break
			}

			row := component.BuildActionRow(buttons[lb:ub]...)
			rows = append(rows, row)
		}

		components = rows
	}

	data := rest.CreateMessageData{
		Embeds:     []*embed.Embed{d.Embed},
		Components: components,
	}

	// TODO: Use proper context
	msg, err := rest.CreateMessage(context.Background(), ctx.Token, ctx.RateLimiter, d.ChannelId, data)
	if err != nil {
		return 0, err
	}

	return msg.Id, nil
}
