package forms

import (
	"context"
	"errors"
	"fmt"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/app"
	dbclient "github.com/jadevelopmentgrp/Ticket-Dashboard/database"
	"github.com/jadevelopmentgrp/Ticket-Dashboard/utils"
	"github.com/jadevelopmentgrp/Ticket-Database"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/rxdn/gdl/objects/interaction/component"
	"net/http"
	"sort"
	"strconv"
)

type (
	updateInputsBody struct {
		Create []inputCreateBody `json:"create" validate:"omitempty,dive"`
		Update []inputUpdateBody `json:"update" validate:"omitempty,dive"`
		Delete []int             `json:"delete" validate:"omitempty"`
	}

	inputCreateBody struct {
		Label       string                   `json:"label" validate:"required,min=1,max=45"`
		Placeholder *string                  `json:"placeholder,omitempty" validate:"omitempty,min=1,max=100"`
		Position    int                      `json:"position" validate:"required,min=1,max=5"`
		Style       component.TextStyleTypes `json:"style" validate:"required,min=1,max=2"`
		Required    bool                     `json:"required"`
		MinLength   uint16                   `json:"min_length" validate:"min=0,max=1024"` // validator interprets 0 as not set
		MaxLength   uint16                   `json:"max_length" validate:"min=0,max=1024"`
	}

	inputUpdateBody struct {
		Id              int `json:"id" validate:"required"`
		inputCreateBody `validate:"required,dive"`
	}
)

var validate = validator.New()

func UpdateInputs(c *gin.Context) {
	guildId := c.Keys["guildid"].(uint64)

	formId, err := strconv.Atoi(c.Param("form_id"))
	if err != nil {
		c.JSON(400, utils.ErrorStr("Invalid form ID"))
		return
	}

	var data updateInputsBody
	if err := c.BindJSON(&data); err != nil {
		c.JSON(400, utils.ErrorJson(err))
		return
	}

	if err := validate.Struct(data); err != nil {
		var validationErrors validator.ValidationErrors
		if !errors.As(err, &validationErrors) {
			_ = c.AbortWithError(http.StatusInternalServerError, app.NewError(err, "An error occurred while validating the integration"))
			return
		}

		formatted := "Your input contained the following errors:\n" + utils.FormatValidationErrors(validationErrors)
		c.JSON(400, utils.ErrorStr(formatted))
		return
	}

	fieldCount := len(data.Create) + len(data.Update)
	if fieldCount <= 0 || fieldCount > 5 {
		c.JSON(400, utils.ErrorStr("Forms must have between 1 and 5 inputs"))
		return
	}

	// Verify form exists and is from the right guild
	form, ok, err := dbclient.Client.Forms.Get(c, formId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	if !ok {
		c.JSON(404, utils.ErrorStr("Form not found"))
		return
	}

	if form.GuildId != guildId {
		c.JSON(403, utils.ErrorStr("Form does not belong to this guild"))
		return
	}

	existingInputs, err := dbclient.Client.FormInput.GetInputs(c, formId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	// Verify that the UPDATE inputs exist
	for _, input := range data.Update {
		if !utils.ExistsMap(existingInputs, input.Id, idMapper) {
			c.JSON(400, utils.ErrorStr("Input (to be updated) not found"))
			return
		}
	}

	// Verify that the DELETE inputs exist
	for _, id := range data.Delete {
		if !utils.ExistsMap(existingInputs, id, idMapper) {
			c.JSON(400, utils.ErrorStr("Input (to be deleted) not found"))
			return
		}
	}

	// Ensure no overlap between DELETE and UPDATE
	for _, id := range data.Delete {
		if utils.ExistsMap(data.Update, id, idMapperBody) {
			c.JSON(400, utils.ErrorStr("Delete and update overlap"))
			return
		}
	}

	// Verify that we are updating ALL inputs, excluding the ones to be deleted
	var remainingExisting []int
	for _, input := range existingInputs {
		if !utils.Exists(data.Delete, input.Id) {
			remainingExisting = append(remainingExisting, input.Id)
		}
	}

	// Now verify that the contents match exactly
	if len(remainingExisting) != len(data.Update) {
		c.JSON(400, utils.ErrorStr("All inputs must be included in the update array"))
		return
	}

	for _, input := range data.Update {
		if !utils.Exists(remainingExisting, input.Id) {
			c.JSON(400, utils.ErrorStr("All inputs must be included in the update array"))
			return
		}
	}

	// Verify that the positions are unique, and are in ascending order
	if !arePositionsCorrect(data) {
		c.JSON(400, utils.ErrorStr("Positions must be unique and in ascending order"))
		return
	}

	if err := saveInputs(c, formId, data, existingInputs); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	c.Status(204)
}

func idMapper(input database.FormInput) int {
	return input.Id
}

func idMapperBody(input inputUpdateBody) int {
	return input.Id
}

func arePositionsCorrect(body updateInputsBody) bool {
	var positions []int
	for _, input := range body.Create {
		positions = append(positions, input.Position)
	}

	for _, input := range body.Update {
		positions = append(positions, input.Position)
	}

	sort.Slice(positions, func(i, j int) bool {
		return positions[i] < positions[j]
	})

	for i, position := range positions {
		if i+1 != position {
			return false
		}
	}

	return true
}

func saveInputs(ctx context.Context, formId int, data updateInputsBody, existingInputs []database.FormInput) error {
	// We can now update in the database
	tx, err := dbclient.Client.BeginTx(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(context.Background())

	for _, id := range data.Delete {
		if err := dbclient.Client.FormInput.DeleteTx(ctx, tx, id, formId); err != nil {
			return err
		}
	}

	for _, input := range data.Update {
		existing := utils.FindMap(existingInputs, input.Id, idMapper)
		if existing == nil {
			return fmt.Errorf("input %d does not exist", input.Id)
		}

		wrapped := database.FormInput{
			Id:          input.Id,
			FormId:      formId,
			Position:    input.Position,
			CustomId:    existing.CustomId,
			Style:       uint8(input.Style),
			Label:       input.Label,
			Placeholder: input.Placeholder,
			Required:    input.Required,
			MinLength:   &input.MinLength,
			MaxLength:   &input.MaxLength,
		}

		if err := dbclient.Client.FormInput.UpdateTx(ctx, tx, wrapped); err != nil {
			return err
		}
	}

	for _, input := range data.Create {
		customId, err := utils.RandString(30)
		if err != nil {
			return err
		}

		if _, err := dbclient.Client.FormInput.CreateTx(ctx,
			tx,
			formId,
			customId,
			input.Position,
			uint8(input.Style),
			input.Label,
			input.Placeholder,
			input.Required,
			&input.MinLength,
			&input.MaxLength,
		); err != nil {
			return err
		}
	}

	return tx.Commit(context.Background())
}
