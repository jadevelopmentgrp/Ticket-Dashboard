package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/app"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/database"
	"github.com/jadevelopmentgrp/Tickets-Dashboard/rpc/cache"
	"github.com/rxdn/gdl/objects/user"
	"net/http"
	"time"
)

type (
	listTicketsResponse struct {
		Tickets       []ticketData         `json:"tickets"`
		PanelTitles   map[int]string       `json:"panel_titles"`
		ResolvedUsers map[uint64]user.User `json:"resolved_users"`
		SelfId        uint64               `json:"self_id,string"`
	}

	ticketData struct {
		TicketId            int        `json:"id"`
		PanelId             *int       `json:"panel_id"`
		UserId              uint64     `json:"user_id,string"`
		ClaimedBy           *uint64    `json:"claimed_by,string"`
		OpenedAt            time.Time  `json:"opened_at"`
		LastResponseTime    *time.Time `json:"last_response_time"`
		LastResponseIsStaff *bool      `json:"last_response_is_staff"`
	}
)

func GetTickets(c *gin.Context) {
	userId := c.Keys["userid"].(uint64)
	guildId := c.Keys["guildid"].(uint64)

	tickets, err := database.Client.Tickets.GetGuildOpenTicketsWithMetadata(c, guildId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	panels, err := database.Client.Panel.GetByGuild(c, guildId)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	panelTitles := make(map[int]string)
	for _, panel := range panels {
		panelTitles[panel.PanelId] = panel.Title
	}

	// Get user objects
	userIds := make([]uint64, 0, int(float32(len(tickets))*1.5))
	for _, ticket := range tickets {
		userIds = append(userIds, ticket.Ticket.UserId)

		if ticket.ClaimedBy != nil {
			userIds = append(userIds, *ticket.ClaimedBy)
		}
	}

	users, err := cache.Instance.GetUsers(c, userIds)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, app.NewServerError(err))
		return
	}

	data := make([]ticketData, len(tickets))
	for i, ticket := range tickets {
		data[i] = ticketData{
			TicketId:            ticket.Id,
			PanelId:             ticket.PanelId,
			UserId:              ticket.Ticket.UserId,
			ClaimedBy:           ticket.ClaimedBy,
			OpenedAt:            ticket.OpenTime,
			LastResponseTime:    ticket.LastMessageTime,
			LastResponseIsStaff: ticket.UserIsStaff,
		}
	}

	c.JSON(200, listTicketsResponse{
		Tickets:       data,
		PanelTitles:   panelTitles,
		ResolvedUsers: users,
		SelfId:        userId,
	})
}
