package api

import (
	"context"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	dbclient "github.com/jadevelopmentgrp/Tickets-Dashboard/database"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/utils"
)

func GetTranscriptHandler(ctx *gin.Context) {
	guildId := ctx.Keys["guildid"].(uint64)
	userId := ctx.Keys["userid"].(uint64)

	// format ticket ID
	ticketId, err := strconv.Atoi(ctx.Param("ticketId"))
	if err != nil {
		ctx.JSON(400, utils.ErrorStr("Invalid ticket ID"))
		return
	}

	// get ticket object
	ticket, err := dbclient.Client.Tickets.Get(ctx, ticketId, guildId)
	if err != nil {
		ctx.JSON(500, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Verify this is a valid ticket and it is closed
	if ticket.UserId == 0 || ticket.Open {
		ctx.JSON(404, utils.ErrorStr("Transcript not found"))
		return
	}

	// Verify the user has permissions to be here
	// ticket.UserId cannot be 0
	if ticket.UserId != userId {
		hasPermission, err := utils.HasPermissionToViewTicket(context.Background(), guildId, userId, ticket)
		if err != nil {
			ctx.JSON(err.StatusCode, utils.ErrorJson(err))
			return
		}

		if !hasPermission {
			ctx.JSON(403, utils.ErrorStr("You do not have permission to view this transcript"))
			return
		}
	}

	// retrieve ticket messages from bucket
	messages, err := utils.ArchiverClient.Get(ctx, guildId, ticketId)
	if err != nil {
		if errors.Is(err, archiverclient.ErrNotFound) {
			ctx.JSON(404, utils.ErrorStr("Transcript not found"))
		} else {
			ctx.JSON(500, utils.ErrorJson(err))
		}

		return
	}

	ctx.JSON(200, messages)
}
