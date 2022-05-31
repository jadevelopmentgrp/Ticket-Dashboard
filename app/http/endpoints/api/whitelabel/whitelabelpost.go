package api

import (
	"encoding/base64"
	dbclient "github.com/TicketsBot/GoPanel/database"
	"github.com/TicketsBot/GoPanel/redis"
	"github.com/TicketsBot/GoPanel/utils"
	"github.com/TicketsBot/common/tokenchange"
	"github.com/TicketsBot/database"
	"github.com/gin-gonic/gin"
	"github.com/rxdn/gdl/rest"
	"strconv"
	"strings"
)

func WhitelabelPost(ctx *gin.Context) {
	userId := ctx.Keys["userid"].(uint64)

	// Get token
	var data map[string]interface{}
	if err := ctx.BindJSON(&data); err != nil {
		ctx.JSON(400, gin.H{
			"success": false,
			"error":   "Missing token",
		})
		return
	}

	token, ok := data["token"].(string)
	if !ok || token == "" {
		ctx.JSON(400, utils.ErrorStr("Missing token"))
		return
	}

	if !validateToken(token) {
		ctx.JSON(400, utils.ErrorStr("Invalid token"))
		return
	}

	// Validate token + get bot ID
	bot, err := rest.GetCurrentUser(token, nil)
	if err != nil {
		ctx.JSON(400, utils.ErrorJson(err))
		return
	}

	if !bot.Bot {
		ctx.JSON(400, utils.ErrorStr("Token is not of a bot user"))
		return
	}

	// Check if this is a different token
	existing, err := dbclient.Client.Whitelabel.GetByUserId(userId)
	if err != nil {
		ctx.JSON(500, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if err = dbclient.Client.Whitelabel.Set(database.WhitelabelBot{
		UserId: userId,
		BotId:  bot.Id,
		Token:  token,
	}); err != nil {
		ctx.JSON(500, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	tokenChangeData := tokenchange.TokenChangeData{
		Token: token,
		NewId: bot.Id,
		OldId: existing.BotId,
	}

	if err := tokenchange.PublishTokenChange(redis.Client.Client, tokenChangeData); err != nil {
		ctx.JSON(500, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	ctx.JSON(200, gin.H{
		"success": true,
		"bot":     bot,
	})
}

func validateToken(token string) bool {
	// Check for 2 dots
	if strings.Count(token, ".") != 2 {
		return false
	}

	split := strings.Split(token, ".")

	// Validate bot ID
	// TODO: We could check the date on the snowflake
	if _, err := strconv.ParseUint(utils.Base64Decode(split[0]), 10, 64); err != nil {
		return false
	}

	// Validate time
	timestamp, err := base64.RawURLEncoding.DecodeString(split[1])
	if err != nil {
		return false
	}

	if len(timestamp) != 4 {
		return false
	}

	return true
}
