package model

import "time"

type Token struct {
	Key      string    `json:"key" gorm:"primaryKey" binding:"required"` // unique key
	Value    string    `json:"value"`                                    // value
	Modified time.Time `json:"modified"`
}
