package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/chatreplica"
	dbclient "github.com/jadevelopmentgrp/Ticket-Dashboard/database"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/utils"
)

func GetTranscriptRenderHandler(ctx *gin.Context) {
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
		hasPermission, err := utils.HasPermissionToViewTicket(ctx, guildId, userId, ticket)
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
	transcript, err := utils.ArchiverClient.Get(ctx, guildId, ticketId)
	if err != nil {
		if errors.Is(err, archiverclient.ErrNotFound) {
			ctx.JSON(404, utils.ErrorStr("Transcript not found"))
		} else {
			ctx.JSON(500, utils.ErrorJson(err))
		}

		return
	}

	// Render
	payload := chatreplica.FromTranscript(transcript, ticketId)
	html, err := chatreplica.Render(payload)
	if err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	ctx.Data(200, "text/html", html)
}
