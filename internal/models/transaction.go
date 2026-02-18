package models

import (
	"time"

	"gorm.io/gorm"
)

type TransactionType string

const (
	TransactionTypeDeposit  TransactionType = "deposit"
	TransactionTypeWithdraw TransactionType = "withdraw"
	TransactionTypeTransfer TransactionType = "transfer"
)

type Transaction struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	WalletID     uint            `gorm:"not null;index" json:"wallet_id"`
	Type         TransactionType `gorm:"type:varchar(20);not null" json:"type"`
	Amount       float64         `gorm:"type:decimal(15,2);not null" json:"amount"`
	BalanceAfter float64         `gorm:"type:decimal(15,2);not null" json:"balance_after"`
	Description  string          `gorm:"size:500" json:"description"`
	Reference    string          `gorm:"uniqueIndex;size:100" json:"reference"`

	Wallet *Wallet `gorm:"foreignKey:WalletID" json:"-"`
}

func (Transaction) TableName() string {
	return "transactions"
}
