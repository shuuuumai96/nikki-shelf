package auth

import (
	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/logx"
)

const contextUserKey = "authUser"

func SetUser(c echo.Context, user User) {
	c.Set(contextUserKey, user)
	logx.SetUserID(c, user.ID)
}

func UserFromContext(c echo.Context) (User, bool) {
	user, ok := c.Get(contextUserKey).(User)
	return user, ok
}

func UserID(c echo.Context) (int64, bool) {
	user, ok := UserFromContext(c)
	return user.ID, ok
}
