package api

import (
	"github.com/jadevelopmentgrp/Ticket-Dashboard/botcontext"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/redis"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/rpc/cache"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/utils"
	"github.com/gin-gonic/gin"
	"github.com/rxdn/gdl/objects/channel"
	"github.com/rxdn/gdl/rest"
	"sort"
)

func ChannelsHandler(ctx *gin.Context) {
	guildId := ctx.Keys["guildid"].(uint64)

	botContext, err := botcontext.ContextForGuild(guildId)
	if err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	var channels []channel.Channel
	if ctx.Query("refresh") == "true" {
		hasToken, err := redis.Client.TakeChannelRefreshToken(ctx, guildId)
		if err != nil {
			ctx.JSON(500, utils.ErrorJson(err))
			return
		}

		if hasToken {
			channels, err = rest.GetGuildChannels(ctx, botContext.Token, botContext.RateLimiter, guildId)
			if err != nil {
				ctx.JSON(500, utils.ErrorJson(err))
				return
			}

			if err := cache.Instance.StoreChannels(ctx, channels); err != nil {
				ctx.JSON(500, utils.ErrorJson(err))
				return
			}
		} else {
			channels, err = cache.Instance.GetGuildChannels(ctx, guildId)
			if err != nil {
				ctx.JSON(500, utils.ErrorJson(err))
				return
			}
		}
	} else {
		var err error
		channels, err = botContext.GetGuildChannels(ctx, guildId)
		if err != nil {
			ctx.JSON(500, utils.ErrorJson(err))
			return
		}
	}

	filtered := make([]channel.Channel, 0, len(channels))
	for _, ch := range channels {
		// Filter out threads
		if ch.Type == channel.ChannelTypeGuildNewsThread ||
			ch.Type == channel.ChannelTypeGuildPrivateThread ||
			ch.Type == channel.ChannelTypeGuildPublicThread {
			continue
		}

		filtered = append(filtered, ch)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Position < filtered[j].Position
	})

	ctx.JSON(200, filtered)
}
