package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/app"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/botcontext"
	dbclient "github.com/jadevelopmentgrp/Tickets-Dashboard/database"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/utils"
	database "github.com/jadevelopmentgrp/Tickets-Database"
	"github.com/rxdn/gdl/rest"
	"github.com/rxdn/gdl/rest/request"
	"golang.org/x/sync/errgroup"
)

func MultiPanelUpdate(c *gin.Context) {
	guildId := c.Keys["guildid"].(uint64)

	// parse body
	var data multiPanelCreateData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(400, utils.ErrorStr("Invalid request body"))
		return
	}

	// parse panel ID
	panelId, err := strconv.Atoi(c.Param("panelid"))
	if err != nil {
		c.JSON(400, utils.ErrorStr("Missing panel ID"))
		return
	}

	// retrieve panel from DB
	multiPanel, ok, err := dbclient.Client.MultiPanels.Get(c, panelId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	// check panel exists
	if !ok {
		c.JSON(404, utils.ErrorJson(errors.New("No panel with the provided ID found")))
		return
	}

	// check panel is in the same guild
	if guildId != multiPanel.GuildId {
		c.JSON(403, utils.ErrorJson(errors.New("Guild ID doesn't match")))
		return
	}

	if err := validate.Struct(data); err != nil {
		var validationErrors validator.ValidationErrors
		if ok := errors.As(err, &validationErrors); !ok {
			_ = c.AbortWithError(http.StatusInternalServerError, app.NewError(err, "An error occurred while validating the panel"))
			return
		}

		formatted := "Your input contained the following errors:\n" + utils.FormatValidationErrors(validationErrors)
		c.JSON(400, utils.ErrorStr(formatted))
		return
	}

	// validate body & get sub-panels
	panels, err := data.doValidations(guildId)
	if err != nil {
		c.JSON(400, utils.ErrorJson(err))
		return
	}

	for _, panel := range panels {
		if panel.CustomId == "" {
			panel.CustomId, err = utils.RandString(30)
			if err != nil {
				_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
				return
			}

			if err := dbclient.Client.Panel.Update(c, panel); err != nil {
				_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
				return
			}
		}
	}

	// get bot context
	botContext, err := botcontext.ContextForGuild(guildId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	// delete old message
	ctx, cancel := app.DefaultContext()
	defer cancel()

	if err := rest.DeleteMessage(ctx, botContext.Token, botContext.RateLimiter, multiPanel.ChannelId, multiPanel.MessageId); err != nil {
		var unwrapped request.RestError
		if !errors.As(err, &unwrapped) || !unwrapped.IsClientError() {
			_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
			return
		}
	}
	cancel()

	// send new message
	messageData := data.IntoMessageData()
	messageId, err := messageData.send(botContext, panels)
	if err != nil {
		var unwrapped request.RestError
		if errors.As(err, &unwrapped) && unwrapped.StatusCode == 403 {
			c.JSON(http.StatusBadRequest, utils.ErrorJson(errors.New("I do not have permission to send messages in the provided channel")))
		} else {
			_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		}

		return
	}

	// update DB
	dbEmbed, dbEmbedFields := data.Embed.IntoDatabaseStruct()
	updated := database.MultiPanel{
		Id:                    multiPanel.Id,
		MessageId:             messageId,
		ChannelId:             data.ChannelId,
		GuildId:               guildId,
		SelectMenu:            data.SelectMenu,
		SelectMenuPlaceholder: data.SelectMenuPlaceholder,
		Embed: &database.CustomEmbedWithFields{
			CustomEmbed: dbEmbed,
			Fields:      dbEmbedFields,
		},
	}

	if err = dbclient.Client.MultiPanels.Update(c, multiPanel.Id, updated); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	// TODO: one query for ACID purposes
	// delete old targets
	if err := dbclient.Client.MultiPanelTargets.DeleteAll(c, multiPanel.Id); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	// insert new targets
	group, _ := errgroup.WithContext(context.Background())
	for _, panel := range panels {
		panel := panel

		group.Go(func() error {
			return dbclient.Client.MultiPanelTargets.Insert(c, multiPanel.Id, panel.PanelId)
		})
	}

	if err := group.Wait(); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"data":    multiPanel,
	})
}
