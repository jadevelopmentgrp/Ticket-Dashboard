package api

import (
	"github.com/gin-gonic/gin"
	dbclient "github.com/jadevelopmentgrp/Ticket-Dashboard/database"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/utils"
	database "github.com/jadevelopmentgrp/Tickets-Database"
)

func CreateTeam(ctx *gin.Context) {
	type body struct {
		Name string `json:"name"`
	}

	guildId := ctx.Keys["guildid"].(uint64)

	var data body
	if err := ctx.BindJSON(&data); err != nil {
		ctx.JSON(400, utils.ErrorJson(err))
		return
	}

	if len(data.Name) == 0 || len(data.Name) > 32 {
		ctx.JSON(400, utils.ErrorStr("Team name must be between 1 and 32 characters"))
		return
	}

	_, exists, err := dbclient.Client.SupportTeam.GetByName(ctx, guildId, data.Name)
	if err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	if exists {
		ctx.JSON(400, utils.ErrorStr("Team already exists"))
		return
	}

	id, err := dbclient.Client.SupportTeam.Create(ctx, guildId, data.Name)
	if err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	ctx.JSON(200, database.SupportTeam{
		Id:      id,
		GuildId: guildId,
		Name:    data.Name,
	})
}
