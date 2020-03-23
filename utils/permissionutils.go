package utils

import (
	"fmt"
	"github.com/TicketsBot/GoPanel/config"
	"github.com/TicketsBot/GoPanel/database/table"
	"github.com/TicketsBot/GoPanel/utils/discord/endpoints/guild"
	"github.com/TicketsBot/GoPanel/utils/discord/objects"
	"github.com/gin-gonic/contrib/sessions"
	"github.com/robfig/go-cache"
	"strconv"
	"time"
)

var roleCache = cache.New(time.Minute, time.Minute)

func IsAdmin(store sessions.Session, guild objects.Guild, guildId, userId int64, res chan bool) {
	if Contains(config.Conf.Admins, strconv.Itoa(int(userId))) {
		res <- true
	}

	if guild.Owner {
		res <- true
	}

	if table.IsAdmin(guildId, userId) {
		res <- true
	}

	if guild.Permissions & 0x8 != 0 {
		res <- true
	}

	userRoles := GetRoles(store, guildId, userId)

	adminRolesChan := make(chan []int64)
	go table.GetAdminRoles(strconv.Itoa(int(guildId)), adminRolesChan)
	adminRoles := <- adminRolesChan

	hasAdminRole := false
	for _, userRole := range userRoles {
		for _, adminRole := range adminRoles {
			if userRole == adminRole {
				hasAdminRole = true
				break
			}
		}
	}

	if hasAdminRole {
		res <- true
	}

	res <- false
}

func GetRoles(store sessions.Session, guildId, userId int64) []int64 {
	key := fmt.Sprintf("%d-%d", guildId, userId)
	if cached, ok := roleCache.Get(key); ok {
		return cached.([]int64)
	}

	var member objects.Member
	endpoint := guild.GetGuildMember(int(guildId), int(userId))

	if err, _ := endpoint.Request(store, nil, nil, &member); err != nil {
		return nil
	}

	roleCache.Set(key, &member.Roles, time.Minute)

	return member.Roles
}
