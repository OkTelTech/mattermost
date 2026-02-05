package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type LeaveType string

const (
	LeaveTypeAnnual         LeaveType = "leave"
	LeaveTypeEmergency      LeaveType = "emergency"
	LeaveTypeSick           LeaveType = "sick"
	LeaveTypeLateArrival    LeaveType = "late_arrival"
	LeaveTypeEarlyDeparture LeaveType = "early_departure"
)

type LeaveStatus string

const (
	LeaveStatusPending  LeaveStatus = "pending"
	LeaveStatusApproved LeaveStatus = "approved"
	LeaveStatusRejected LeaveStatus = "rejected"
)

type LeaveRequest struct {
	ID                bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID            string        `bson:"user_id" json:"user_id"`
	Username          string        `bson:"username" json:"username"`
	ChannelID         string        `bson:"channel_id" json:"channel_id"`
	ApprovalChannelID string        `bson:"approval_channel_id" json:"approval_channel_id"`
	PostID            string        `bson:"post_id" json:"post_id"`
	ApprovalPostID    string        `bson:"approval_post_id" json:"approval_post_id"`
	Type              LeaveType     `bson:"type" json:"type"`
	Dates             []string      `bson:"dates" json:"dates"` // list of YYYY-MM-DD
	Reason            string        `bson:"reason" json:"reason"`
	ExpectedTime      string        `bson:"expected_time,omitempty" json:"expected_time,omitempty"` // HH:MM for late arrival / early departure
	Status            LeaveStatus   `bson:"status" json:"status"`
	ApproverID        string        `bson:"approver_id,omitempty" json:"approver_id"`
	ApproverUsername  string        `bson:"approver_username,omitempty" json:"approver_username"`
	ApprovedAt        *time.Time    `bson:"approved_at,omitempty" json:"approved_at"`
	RejectReason      string        `bson:"reject_reason,omitempty" json:"reject_reason"`
	CreatedAt         time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt         time.Time     `bson:"updated_at" json:"updated_at"`
}
