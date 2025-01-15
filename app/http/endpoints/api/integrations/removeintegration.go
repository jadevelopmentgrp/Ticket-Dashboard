package api

import (
	"github.com/gin-gonic/gin"
	dbclient "github.com/jadevelopmentgrp/Tickets-Dashboard/database"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/utils"
	"strconv"
)

func RemoveIntegrationHandler(ctx *gin.Context) {
	guildId := ctx.Keys["guildid"].(uint64)

	integrationId, err := strconv.Atoi(ctx.Param("integrationid"))
	if err != nil {
		ctx.JSON(400, utils.ErrorStr("Invalid integration ID"))
		return
	}

	if err := dbclient.Client.CustomIntegrationGuilds.RemoveFromGuild(ctx, integrationId, guildId); err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	ctx.Status(204)
}
