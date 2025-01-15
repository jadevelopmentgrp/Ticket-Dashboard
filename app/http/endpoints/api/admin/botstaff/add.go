package botstaff

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/database"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/utils"
)

func AddBotStaffHandler(ctx *gin.Context) {
	userId, err := strconv.ParseUint(ctx.Param("userid"), 10, 64)
	if err != nil {
		ctx.JSON(400, utils.ErrorJson(err))
		return
	}

	if err := database.Client.BotStaff.Add(ctx, userId); err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	ctx.Status(204)
}
