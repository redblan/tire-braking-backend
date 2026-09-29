package repository

import "sync"

var (
	currentUserOnce sync.Once
	currentUserID   uint
)

// CurrentUser — синглтон: пользователь-создатель зафиксирован до реальной авторизации в ЛР4
func CurrentUser() uint {
	currentUserOnce.Do(func() {
		currentUserID = 1
	})
	return currentUserID
}
