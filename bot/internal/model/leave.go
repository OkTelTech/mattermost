package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	LeaveTypeAnnual    = "leave"
	LeaveTypeEmergency = "emergency"
	LeaveTypeSick      = "sick"

	LeaveStatusPending  = "pending"
	LeaveStatusApproved = "approved"
	LeaveStatusRejected = "rejected"
)

type LeaveRequest struct {
	ID                bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID            string        `bson:"user_id" json:"user_id"`
	Username          string        `bson:"username" json:"username"`
	ChannelID         string        `bson:"channel_id" json:"channel_id"`
	ApprovalChannelID string        `bson:"approval_channel_id" json:"approval_channel_id"`
	PostID            string        `bson:"post_id" json:"post_id"`
	ApprovalPostID    string        `bson:"approval_post_id" json:"approval_post_id"`
	Type              string        `bson:"type" json:"type"`
	StartDate         string        `bson:"start_date" json:"start_date"`
	EndDate           string        `bson:"end_date" json:"end_date"`
	Days              int           `bson:"days" json:"days"`
	Reason            string        `bson:"reason" json:"reason"`
	Status            string        `bson:"status" json:"status"`
	ApproverID        string        `bson:"approver_id,omitempty" json:"approver_id"`
	ApproverUsername  string        `bson:"approver_username,omitempty" json:"approver_username"`
	ApprovedAt        *time.Time    `bson:"approved_at,omitempty" json:"approved_at"`
	RejectReason      string        `bson:"reject_reason,omitempty" json:"reject_reason"`
	CreatedAt         time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt         time.Time     `bson:"updated_at" json:"updated_at"`
}
