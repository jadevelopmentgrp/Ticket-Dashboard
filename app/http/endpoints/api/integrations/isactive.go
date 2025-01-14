package api

import (
	dbclient "github.com/jadevelopmentgrp/Ticket-Dashboard/database"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/utils"
	"github.com/gin-gonic/gin"
	"strconv"
)

func IsIntegrationActiveHandler(ctx *gin.Context) {
	guildId := ctx.Keys["guildid"].(uint64)

	integrationId, err := strconv.Atoi(ctx.Param("integrationid"))
	if err != nil {
		ctx.JSON(400, utils.ErrorStr("Invalid integration ID"))
		return
	}

	active, err := dbclient.Client.CustomIntegrationGuilds.IsActive(ctx, integrationId, guildId)
	if err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	ctx.JSON(200, gin.H{
		"active": active,
	})
}
