package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/app"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/botcontext"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/database"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/utils"
	"github.com/rxdn/gdl/rest"
	"github.com/rxdn/gdl/rest/request"
)

func DeletePanel(c *gin.Context) {
	guildId := c.Keys["guildid"].(uint64)

	botContext, err := botcontext.ContextForGuild(guildId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	panelId, err := strconv.Atoi(c.Param("panelid"))
	if err != nil {
		c.JSON(400, utils.ErrorStr("Missing panel ID"))
		return
	}

	panel, err := database.Client.Panel.GetById(c, panelId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	if panel.PanelId == 0 {
		c.JSON(404, utils.ErrorStr("Panel not found"))
		return
	}

	// verify panel belongs to guild
	if panel.GuildId != guildId {
		c.JSON(403, utils.ErrorStr("Guild ID doesn't match"))
		return
	}

	// Get any multi panels this panel is part of to use later
	multiPanels, err := database.Client.MultiPanelTargets.GetMultiPanels(c, panelId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	// Delete welcome message embed
	if panel.WelcomeMessageEmbed != nil {
		if err := database.Client.Embeds.Delete(c, *panel.WelcomeMessageEmbed); err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
			return
		}
	}

	if err := database.Client.Panel.Delete(c, panelId); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	// TODO: Set timeout on context
	if err := rest.DeleteMessage(c, botContext.Token, botContext.RateLimiter, panel.ChannelId, panel.MessageId); err != nil {
		var unwrapped request.RestError
		if !errors.As(err, &unwrapped) || unwrapped.StatusCode != 404 {
			_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
			return
		}
	}

	// Update all multi panels messages to remove the button
	for i, multiPanel := range multiPanels {
		// Only update 5 multi-panels maximum: Prevent DoS
		if i >= 5 {
			break
		}

		panels, err := database.Client.MultiPanelTargets.GetPanels(c, multiPanel.Id)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
			return
		}

		messageData := multiPanelIntoMessageData(multiPanel, true)
		messageId, err := messageData.send(botContext, panels)
		if err != nil {
			var unwrapped request.RestError
			if !errors.As(err, &unwrapped) || !unwrapped.IsClientError() {
				_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
				return
			}
			// TODO: nil message ID?
		} else {
			if err := database.Client.MultiPanels.UpdateMessageId(c, multiPanel.Id, messageId); err != nil {
				_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
				return
			}

			// Delete old panel
			// TODO: Use proper context
			_ = rest.DeleteMessage(c, botContext.Token, botContext.RateLimiter, multiPanel.ChannelId, multiPanel.MessageId)
		}
	}

	c.JSON(200, utils.SuccessResponse)
}
