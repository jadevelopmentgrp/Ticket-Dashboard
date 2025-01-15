package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v4"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/app"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/app/http/validation"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/botcontext"
	dbclient "github.com/jadevelopmentgrp/Ticket-Dashboard/database"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/utils"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/utils/types"
	"github.com/rxdn/gdl/objects/guild/emoji"
	"github.com/rxdn/gdl/objects/interaction/component"
	"github.com/rxdn/gdl/rest/request"
)

const freePanelLimit = 3

type panelBody struct {
	ChannelId         uint64                            `json:"channel_id,string"`
	MessageId         uint64                            `json:"message_id,string"`
	Title             string                            `json:"title"`
	Content           string                            `json:"content"`
	Colour            uint32                            `json:"colour"`
	CategoryId        uint64                            `json:"category_id,string"`
	Emoji             types.Emoji                       `json:"emote"`
	WelcomeMessage    *types.CustomEmbed                `json:"welcome_message" validate:"omitempty,dive"`
	Mentions          []string                          `json:"mentions"`
	WithDefaultTeam   bool                              `json:"default_team"`
	Teams             []int                             `json:"teams"`
	ImageUrl          *string                           `json:"image_url,omitempty"`
	ThumbnailUrl      *string                           `json:"thumbnail_url,omitempty"`
	ButtonStyle       component.ButtonStyle             `json:"button_style,string"`
	ButtonLabel       string                            `json:"button_label"`
	FormId            *int                              `json:"form_id"`
	NamingScheme      *string                           `json:"naming_scheme"`
	Disabled          bool                              `json:"disabled"`
	ExitSurveyFormId  *int                              `json:"exit_survey_form_id"`
	AccessControlList []database.PanelAccessControlRule `json:"access_control_list"`
	PendingCategory   *uint64                           `json:"pending_category,string"`
}

func (p *panelBody) IntoPanelMessageData(customId string) panelMessageData {
	return panelMessageData{
		ChannelId:      p.ChannelId,
		Title:          p.Title,
		Content:        p.Content,
		CustomId:       customId,
		Colour:         int(p.Colour),
		ImageUrl:       p.ImageUrl,
		ThumbnailUrl:   p.ThumbnailUrl,
		Emoji:          p.getEmoji(),
		ButtonStyle:    p.ButtonStyle,
		ButtonLabel:    p.ButtonLabel,
		ButtonDisabled: p.Disabled,
		IsPremium:      true,
	}
}

var validate = validator.New()

func CreatePanel(c *gin.Context) {
	guildId := c.Keys["guildid"].(uint64)

	botContext, err := botcontext.ContextForGuild(guildId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	var data panelBody

	if err := c.BindJSON(&data); err != nil {
		c.JSON(400, utils.ErrorStr("Invalid request body"))
		return
	}

	data.MessageId = 0

	// Apply defaults
	ApplyPanelDefaults(&data)

	ctx, cancel := app.DefaultContext()
	defer cancel()

	channels, err := botContext.GetGuildChannels(ctx, guildId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	roles, err := botContext.GetGuildRoles(ctx, guildId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	// Do custom validation
	validationContext := PanelValidationContext{
		Data:       data,
		GuildId:    guildId,
		IsPremium:  true,
		BotContext: botContext,
		Channels:   channels,
		Roles:      roles,
	}

	if err := ValidatePanelBody(validationContext); err != nil {
		var validationError *validation.InvalidInputError
		if errors.As(err, &validationError) {
			c.JSON(400, utils.ErrorStr(validationError.Error()))
		} else {
			_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		}

		return
	}

	// Do tag validation
	if err := validate.Struct(data); err != nil {
		var validationErrors validator.ValidationErrors
		if ok := errors.As(err, &validationErrors); !ok {
			c.JSON(500, utils.ErrorStr("An error occurred while validating the panel"))
			return
		}

		formatted := "Your input contained the following errors:\n" + utils.FormatValidationErrors(validationErrors)
		c.JSON(400, utils.ErrorStr(formatted))
		return
	}

	customId, err := utils.RandString(30)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	messageData := data.IntoPanelMessageData(customId, true)
	msgId, err := messageData.send(botContext)
	if err != nil {
		var unwrapped request.RestError
		if errors.As(err, &unwrapped) {
			if unwrapped.StatusCode == http.StatusForbidden {
				c.JSON(400, utils.ErrorStr("I do not have permission to send messages in the specified channel"))
			} else {
				c.JSON(400, utils.ErrorStr("Error sending panel message: "+unwrapped.ApiError.Message))
			}
		} else {
			_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		}

		return
	}

	var emojiId *uint64
	var emojiName *string
	{
		emoji := data.getEmoji()
		if emoji != nil {
			emojiName = &emoji.Name

			if emoji.Id.Value != 0 {
				emojiId = &emoji.Id.Value
			}
		}
	}

	// Store welcome message embed first
	var welcomeMessageEmbed *int
	if data.WelcomeMessage != nil {
		embed, fields := data.WelcomeMessage.IntoDatabaseStruct()
		embed.GuildId = guildId

		id, err := dbclient.Client.Embeds.CreateWithFields(c, embed, fields)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
			return
		}

		welcomeMessageEmbed = &id
	}

	// Store in DB
	panel := database.Panel{
		MessageId:           msgId,
		ChannelId:           data.ChannelId,
		GuildId:             guildId,
		Title:               data.Title,
		Content:             data.Content,
		Colour:              int32(data.Colour),
		TargetCategory:      data.CategoryId,
		EmojiId:             emojiId,
		EmojiName:           emojiName,
		WelcomeMessageEmbed: welcomeMessageEmbed,
		WithDefaultTeam:     data.WithDefaultTeam,
		CustomId:            customId,
		ImageUrl:            data.ImageUrl,
		ThumbnailUrl:        data.ThumbnailUrl,
		ButtonStyle:         int(data.ButtonStyle),
		ButtonLabel:         data.ButtonLabel,
		FormId:              data.FormId,
		NamingScheme:        data.NamingScheme,
		ForceDisabled:       false,
		Disabled:            data.Disabled,
		ExitSurveyFormId:    data.ExitSurveyFormId,
		PendingCategory:     data.PendingCategory,
	}

	createOptions := panelCreateOptions{
		TeamIds:            data.Teams,             // Already validated
		AccessControlRules: data.AccessControlList, // Already validated
	}

	// insert role mention data
	// string is role ID or "user" to mention the ticket opener
	validRoles := utils.ToSet(utils.Map(roles, utils.RoleToId))

	var roleMentions []uint64
	for _, mention := range data.Mentions {
		if mention == "user" {
			createOptions.ShouldMentionUser = true
		} else {
			roleId, err := strconv.ParseUint(mention, 10, 64)
			if err != nil {
				c.JSON(400, utils.ErrorStr("Invalid role ID"))
				return
			}

			if validRoles.Contains(roleId) {
				createOptions.RoleMentions = append(roleMentions, roleId)
			}
		}
	}

	panelId, err := storePanel(c, panel, createOptions)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	c.JSON(200, gin.H{
		"success":  true,
		"panel_id": panelId,
	})
}

// DB functions

type panelCreateOptions struct {
	ShouldMentionUser  bool
	RoleMentions       []uint64
	TeamIds            []int
	AccessControlRules []database.PanelAccessControlRule
}

func storePanel(ctx context.Context, panel database.Panel, options panelCreateOptions) (int, error) {
	var panelId int
	err := dbclient.Client.Panel.BeginFunc(ctx, func(tx pgx.Tx) error {
		var err error
		panelId, err = dbclient.Client.Panel.CreateWithTx(ctx, tx, panel)
		if err != nil {
			return err
		}

		if err := dbclient.Client.PanelUserMention.SetWithTx(ctx, tx, panelId, options.ShouldMentionUser); err != nil {
			return err
		}

		if err := dbclient.Client.PanelRoleMentions.ReplaceWithTx(ctx, tx, panelId, options.RoleMentions); err != nil {
			return err
		}

		// Already validated, we are safe to insert
		if err := dbclient.Client.PanelTeams.ReplaceWithTx(ctx, tx, panelId, options.TeamIds); err != nil {
			return err
		}

		if err := dbclient.Client.PanelAccessControlRules.ReplaceWithTx(ctx, tx, panelId, options.AccessControlRules); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return panelId, nil
}

// Data must be validated before calling this function
func (p *panelBody) getEmoji() *emoji.Emoji {
	return p.Emoji.IntoGdl()
}
