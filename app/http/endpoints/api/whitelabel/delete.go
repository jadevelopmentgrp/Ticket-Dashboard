package api

import (
	"github.com/jadevelopmentgrp/Ticket-Dashboard/app"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/database"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/redis"
	"github.com/jadevelopmentgrp/Ticket-Utilities/whitelabeldelete"
	"github.com/gin-gonic/gin"
	"net/http"
)

func WhitelabelDelete(c *gin.Context) {
	userId := c.Keys["userid"].(uint64)

	// Check if this is a different token
	botId, err := database.Client.Whitelabel.Delete(c, userId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	if botId != nil {
		// TODO: Kafka
		go whitelabeldelete.Publish(redis.Client.Client, *botId)

	}

	c.Status(http.StatusNoContent)
}
