package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	StatusWorking   = "working"
	StatusBreak     = "break"
	StatusCompleted = "completed"
)

type AttendanceRecord struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID     string        `bson:"user_id" json:"user_id"`
	Username   string        `bson:"username" json:"username"`
	ChannelID  string        `bson:"channel_id" json:"channel_id"`
	Date       string        `bson:"date" json:"date"` // YYYY-MM-DD
	CheckIn    *time.Time    `bson:"check_in,omitempty" json:"check_in"`
	BreakStart *time.Time    `bson:"break_start,omitempty" json:"break_start"`
	BreakEnd   *time.Time    `bson:"break_end,omitempty" json:"break_end"`
	CheckOut   *time.Time    `bson:"check_out,omitempty" json:"check_out"`
	Status     string        `bson:"status" json:"status"`
	CreatedAt  time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time     `bson:"updated_at" json:"updated_at"`
}
