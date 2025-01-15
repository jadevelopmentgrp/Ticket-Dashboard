package api

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	dbclient "github.com/jadevelopmentgrp/Tickets-Dashboard/database"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/utils"
	database "github.com/jadevelopmentgrp/Tickets-Database"
)

type integrationCreateBody struct {
	Name             string  `json:"name" validate:"required,min=1,max=32"`
	Description      string  `json:"description" validate:"required,min=1,max=255"`
	ImageUrl         *string `json:"image_url" validate:"omitempty,url,max=255,startswith=https://"`
	PrivacyPolicyUrl *string `json:"privacy_policy_url" validate:"omitempty,url,max=255,startswith=https://"`

	Method        string  `json:"http_method" validate:"required,oneof=GET POST"`
	WebhookUrl    string  `json:"webhook_url" validate:"required,webhook,max=255,startsnotwith=https://discord.com,startsnotwith=https://discord.gg"`
	ValidationUrl *string `json:"validation_url" validate:"omitempty,url,max=255,startsnotwith=https://discord.com,startsnotwith=https://discord.gg"`

	Secrets []struct {
		Name        string  `json:"name" validate:"required,min=1,max=32,excludesall=% "`
		Description *string `json:"description" validate:"omitempty,max=255"`
	} `json:"secrets" validate:"dive,omitempty,min=0,max=5"`

	Headers []struct {
		Name  string `json:"name" validate:"required,min=1,max=32,excludes= "`
		Value string `json:"value" validate:"required,min=1,max=255"`
	} `json:"headers" validate:"dive,omitempty,min=0,max=5"`

	Placeholders []struct {
		Placeholder string `json:"name" validate:"required,min=1,max=32,excludesall=% "`
		JsonPath    string `json:"json_path" validate:"required,min=1,max=255"`
	} `json:"placeholders" validate:"dive,omitempty,min=0,max=15"`
}

var validate = newIntegrationValidator()

func CreateIntegrationHandler(ctx *gin.Context) {
	userId := ctx.Keys["userid"].(uint64)

	ownedCount, err := dbclient.Client.CustomIntegrations.GetOwnedCount(ctx, userId)
	if err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	if ownedCount >= 5 {
		ctx.JSON(403, utils.ErrorStr("You have reached the integration limit (5/5)"))
		return
	}

	var data integrationCreateBody
	if err := ctx.BindJSON(&data); err != nil {
		ctx.JSON(400, utils.ErrorJson(err))
		return
	}

	if err := validate.Struct(data); err != nil {
		validationErrors, ok := err.(validator.ValidationErrors)
		if !ok {
			ctx.JSON(500, utils.ErrorStr("An error occurred while validating the integration"))
			return
		}

		formatted := "Your input contained the following errors:\n" + utils.FormatValidationErrors(validationErrors)
		ctx.JSON(400, utils.ErrorStr(formatted))
		return
	}

	if data.ValidationUrl != nil {
		sameHost, err := isSameValidationUrlHost(data.WebhookUrl, *data.ValidationUrl)
		if err != nil {
			ctx.JSON(500, utils.ErrorJson(err))
			return
		}

		if !sameHost {
			ctx.JSON(400, utils.ErrorStr("Validation URL must be on the same host as the webhook URL"))
			return
		}
	}

	integration, err := dbclient.Client.CustomIntegrations.Create(ctx, userId, data.WebhookUrl, data.ValidationUrl, data.Method, data.Name, data.Description, data.ImageUrl, data.PrivacyPolicyUrl)
	if err != nil {
		ctx.JSON(500, utils.ErrorJson(err))
		return
	}

	// Store secrets
	if len(data.Secrets) > 0 {
		secrets := make([]database.CustomIntegrationSecret, len(data.Secrets))
		for i, secret := range data.Secrets {
			secrets[i] = database.CustomIntegrationSecret{
				Name:        secret.Name,
				Description: secret.Description,
			}
		}

		if _, err := dbclient.Client.CustomIntegrationSecrets.CreateOrUpdate(ctx, integration.Id, secrets); err != nil {
			ctx.JSON(500, utils.ErrorJson(err))
			return
		}
	}

	// Store headers
	if len(data.Headers) > 0 {
		headers := make([]database.CustomIntegrationHeader, len(data.Headers))
		for i, header := range data.Headers {
			headers[i] = database.CustomIntegrationHeader{
				Name:  header.Name,
				Value: header.Value,
			}
		}

		if _, err := dbclient.Client.CustomIntegrationHeaders.CreateOrUpdate(ctx, integration.Id, headers); err != nil {
			ctx.JSON(500, utils.ErrorJson(err))
			return
		}
	}

	// Store placeholders
	if len(data.Placeholders) > 0 {
		placeholders := make([]database.CustomIntegrationPlaceholder, len(data.Placeholders))
		for i, placeholder := range data.Placeholders {
			placeholders[i] = database.CustomIntegrationPlaceholder{
				Name:     placeholder.Placeholder,
				JsonPath: placeholder.JsonPath,
			}
		}

		if _, err := dbclient.Client.CustomIntegrationPlaceholders.Set(ctx, integration.Id, placeholders); err != nil {
			ctx.JSON(500, utils.ErrorJson(err))
			return
		}
	}

	ctx.JSON(200, integration)
}

func isSameValidationUrlHost(webhookUrl, validationUrl string) (bool, error) {
	webhookStripped := utils.GetUrlHost(strings.ReplaceAll(webhookUrl, "%", ""))
	validationStripped := utils.GetUrlHost(strings.ReplaceAll(validationUrl, "%", ""))

	if webhookStripped == "Invalid URL" || validationStripped == "Invalid URL" {
		return false, errors.New("invalid webhook or validation URL")
	}

	return strings.ToLower(utils.SecondLevelDomain(webhookStripped)) == strings.ToLower(utils.SecondLevelDomain(validationStripped)), nil
}
