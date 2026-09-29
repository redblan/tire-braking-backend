package repository

import "time"

type Request struct {
	ID          uint       `gorm:"primaryKey"`
	Status      string     `gorm:"column:status"`
	CreatorID   uint       `gorm:"column:creator_id"`
	ModeratorID *uint      `gorm:"column:moderator_id"`
	SpeedKmh    *int       `gorm:"column:speed_kmh"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	FormedAt    *time.Time `gorm:"column:formed_at"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
}

func (Request) TableName() string { return "requests" }

type RequestItem struct {
	ID              uint     `gorm:"primaryKey"`
	RequestID       uint     `gorm:"column:request_id"`
	TireConditionID uint     `gorm:"column:tire_condition_id"`
	Position        int      `gorm:"column:position"`
	BrakingDistance *float64 `gorm:"column:braking_distance"`
}

func (RequestItem) TableName() string { return "request_items" }
