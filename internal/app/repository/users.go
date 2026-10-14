package repository

import (
	"errors"

	"gorm.io/gorm"
)

var ErrUsernameTaken = errors.New("username already taken")

type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"column:username"`
	PasswordHash string `gorm:"column:password_hash"`
	IsModerator  bool   `gorm:"column:is_moderator"`
}

func (User) TableName() string { return "users" }

// CreateUser — ORM, создаёт пользователя с уже захешированным паролем; занятый username -> ErrUsernameTaken
func (r *Repository) CreateUser(username, passwordHash string) (User, error) {
	var existing User
	err := r.db.Where("username = ?", username).First(&existing).Error
	if err == nil {
		return User{}, ErrUsernameTaken
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, err
	}

	user := User{Username: username, PasswordHash: passwordHash}
	err = r.db.Create(&user).Error
	return user, err
}

// GetUserByUsername — ORM; не найден -> gorm.ErrRecordNotFound
func (r *Repository) GetUserByUsername(username string) (User, error) {
	var user User
	err := r.db.Where("username = ?", username).First(&user).Error
	return user, err
}
