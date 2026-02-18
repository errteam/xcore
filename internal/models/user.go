package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Email    string `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Username string `gorm:"uniqueIndex;size:100;not null" json:"username"`
	Password string `gorm:"size:255;not null" json:"-"`
	Name     string `gorm:"size:100" json:"name"`

	Wallet *Wallet `gorm:"foreignKey:UserID" json:"wallet,omitempty"`
}

func (User) TableName() string {
	return "users"
}
