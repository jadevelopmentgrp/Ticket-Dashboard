package api

import (
	"errors"
	"fmt"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/botcontext"
	dbclient "github.com/jadevelopmentgrp/Ticket-Dashboard/database"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/rpc"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/utils"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/utils/types"
	"github.com/jadevelopmentgrp/Ticket-Utilities/premium"
	"github.com/jadevelopmentgrp/Ticket-Database"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/rxdn/gdl/objects/interaction"
	"github.com/rxdn/gdl/rest"
	"regexp"
	"strings"
)

type tag struct {
	Id              string             `json:"id" validate:"required,min=1,max=16"`
	UseGuildCommand bool               `json:"use_guild_command"`
	Content         *string            `json:"content" validate:"omitempty,min=1,max=4096"`
	UseEmbed        bool               `json:"use_embed"`
	Embed           *types.CustomEmbed `json:"embed" validate:"omitempty,dive"`
}

var (
	validate          = validator.New()
	slashCommandRegex = regexp.MustCompile(`^[-_a-zA-Z0-9]{1,32}$`)
)

func CreateTag(ctx *gin.Context) {
	guildId := ctx.Keys["guildid"].(uint64)

	// Max of 200 tags
	count, err := dbclient.Client.Tag.GetTagCount(ctx, guildId)
	if err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	if count >= 200 {
		ctx.JSON(400, utils.ErrorStr("Tag limit (200) reached"))
		return
	}

	var data tag
	if err := ctx.BindJSON(&data); err != nil {
		ctx.JSON(400, utils.ErrorJson(err))
		return
	}

	data.Id = strings.ToLower(data.Id)

	if !data.UseEmbed {
		data.Embed = nil
	}

	// TODO: Limit command amount
	if err := validate.Struct(data); err != nil {
		var validationErrors validator.ValidationErrors
		if ok := errors.As(err, &validationErrors); !ok {
			ctx.JSON(500, utils.ErrorStr("An error occurred while validating the integration"))
			return
		}

		formatted := "Your input contained the following errors:\n" + utils.FormatValidationErrors(validationErrors)
		ctx.JSON(400, utils.ErrorStr(formatted))
		return
	}

	if !data.verifyId() {
		ctx.JSON(400, utils.ErrorStr("Tag IDs must be alphanumeric (including hyphens and underscores), and be between 1 and 16 characters long"))
		return
	}

	if !data.verifyContent() {
		ctx.JSON(400, utils.ErrorStr("You have not provided any content for the tag"))
		return
	}

	botContext, err := botcontext.ContextForGuild(guildId)
	if err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	if data.UseGuildCommand {
		premiumTier, err := rpc.PremiumClient.GetTierByGuildId(ctx, guildId, true, botContext.Token, botContext.RateLimiter)
		if err != nil {
			ctx.JSON(500, utils.ErrorJson(err))
			return
		}

		if premiumTier < premium.Premium {
			ctx.JSON(400, utils.ErrorStr("Premium is required to use custom commands"))
			return
		}
	}

	var embed *database.CustomEmbedWithFields
	if data.Embed != nil {
		customEmbed, fields := data.Embed.IntoDatabaseStruct()
		embed = &database.CustomEmbedWithFields{
			CustomEmbed: customEmbed,
			Fields:      fields,
		}
	}

	var applicationCommandId *uint64
	if data.UseGuildCommand {
		cmd, err := botContext.CreateGuildCommand(ctx, guildId, rest.CreateCommandData{
			Name:        data.Id,
			Description: fmt.Sprintf("Alias for /tag %s", data.Id),
			Options:     nil,
			Type:        interaction.ApplicationCommandTypeChatInput,
		})

		if err != nil {
			ctx.JSON(500, utils.ErrorJson(err))
			return
		}

		applicationCommandId = &cmd.Id
	}

	wrapped := database.Tag{
		Id:                   data.Id,
		GuildId:              guildId,
		Content:              data.Content,
		Embed:                embed,
		ApplicationCommandId: applicationCommandId,
	}

	if err := dbclient.Client.Tag.Set(ctx, wrapped); err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	ctx.Status(204)
}

func (t *tag) verifyId() bool {
	if len(t.Id) == 0 || len(t.Id) > 16 || strings.Contains(t.Id, " ") {
		return false
	}

	if t.UseGuildCommand {
		return slashCommandRegex.MatchString(t.Id)
	} else {
		return true
	}
}

func (t *tag) verifyContent() bool {
	if t.Content != nil { // validator ensures that if this is not nil, > 0 length
		return true
	}

	if t.Embed != nil {
		if t.Embed.Description != nil || len(t.Embed.Fields) > 0 || t.Embed.ImageUrl != nil || t.Embed.ThumbnailUrl != nil {
			return true
		}
	}

	return false
}
