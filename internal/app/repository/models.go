package repository

type Like struct {
	ID              uint `gorm:"primaryKey"`
	UserID          uint `gorm:"column:user_id"`
	TireConditionID uint `gorm:"column:tire_condition_id"`
}

func (Like) TableName() string { return "likes" }
