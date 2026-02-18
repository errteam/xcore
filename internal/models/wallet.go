package models

import (
	"time"

	"gorm.io/gorm"
)

type Wallet struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   uint    `gorm:"uniqueIndex;not null" json:"user_id"`
	Balance  float64 `gorm:"type:decimal(15,2);default:0.00" json:"balance"`
	Currency string  `gorm:"size:3;default:USD" json:"currency"`

	User         *User         `gorm:"foreignKey:UserID" json:"-"`
	Transactions []Transaction `gorm:"foreignKey:WalletID" json:"transactions,omitempty"`
}

func (Wallet) TableName() string {
	return "wallets"
}
