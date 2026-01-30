package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type BudgetRequest struct {
	ID                bson.ObjectID `bson:"_id,omitempty" json:"id"`
	RequestID         string        `bson:"request_id" json:"request_id"` // BR-YYYYMMDDNN
	ChannelID         string        `bson:"channel_id" json:"channel_id"`
	ApprovalChannelID string        `bson:"approval_channel_id" json:"approval_channel_id"`
	PostID            string        `bson:"post_id" json:"post_id"`
	ApprovalPostID    string        `bson:"approval_post_id" json:"approval_post_id"`
	CurrentStep       int           `bson:"current_step" json:"current_step"`
	Status            string        `bson:"status" json:"status"`

	// Step 1: Sale Info
	SaleUserID string  `bson:"sale_user_id" json:"sale_user_id"`
	Campaign   string  `bson:"campaign" json:"campaign"`
	Partner    string  `bson:"partner" json:"partner"`
	Amount     float64 `bson:"amount" json:"amount"`
	Purpose    string  `bson:"purpose" json:"purpose"`
	Deadline   string  `bson:"deadline" json:"deadline"`

	// Step 2: Partner Content
	PostContent string `bson:"post_content,omitempty" json:"post_content"`
	PostLink    string `bson:"post_link,omitempty" json:"post_link"`
	PageLink    string `bson:"page_link,omitempty" json:"page_link"`

	// Step 3: TLQC Confirmation
	AdAccountID   string `bson:"ad_account_id,omitempty" json:"ad_account_id"`
	TLQCUserID    string `bson:"tlqc_user_id,omitempty" json:"tlqc_user_id"`
	TLQCConfirmed bool   `bson:"tlqc_confirmed" json:"tlqc_confirmed"`

	// Step 4: Payment Info
	RecipientName string  `bson:"recipient_name,omitempty" json:"recipient_name"`
	BankAccount   string  `bson:"bank_account,omitempty" json:"bank_account"`
	BankName      string  `bson:"bank_name,omitempty" json:"bank_name"`
	PaymentAmount float64 `bson:"payment_amount,omitempty" json:"payment_amount"`

	// Step 5: Team Lead Approval
	TeamLeadID  string `bson:"team_lead_id,omitempty" json:"team_lead_id"`
	VoiceFileID string `bson:"voice_file_id,omitempty" json:"voice_file_id"`
	Approved    bool   `bson:"approved" json:"approved"`

	// Step 6: Bank Note
	BankNote string `bson:"bank_note,omitempty" json:"bank_note"`

	// Step 7: Finance
	BillFileID      string     `bson:"bill_file_id,omitempty" json:"bill_file_id"`
	TransactionCode string     `bson:"transaction_code,omitempty" json:"transaction_code"`
	CompletedAt     *time.Time `bson:"completed_at,omitempty" json:"completed_at"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}
