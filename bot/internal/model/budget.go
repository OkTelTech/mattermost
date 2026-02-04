package model

import (
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// BudgetStep represents the current step in the budget workflow.
type BudgetStep int

const (
	BudgetStepSaleCreated    BudgetStep = 1 // Sale created request
	BudgetStepPartnerContent BudgetStep = 2 // Partner submitted content
	BudgetStepTLQCConfirmed  BudgetStep = 3 // TLQC confirmed
	BudgetStepPaymentInfo    BudgetStep = 4 // Partner submitted payment info
	BudgetStepApproved       BudgetStep = 5 // Approver approved
	BudgetStepCompleted      BudgetStep = 6 // Finance completed
)

const (
	BudgetSaleChannel     = "budget-sale"
	BudgetTLQCChannel     = "budget-tlqc"
	BudgetApprovalChannel = "budget-approval"
	BudgetFinanceChannel  = "budget-finance"
)

// PartnerChannelName returns the channel name for a given partner, e.g. "budget-partner-facebook".
func PartnerChannelName(partner string) string {
	name := strings.ToLower(strings.TrimSpace(partner))
	name = strings.ReplaceAll(name, " ", "-")
	return "budget-partner-" + name
}

type BudgetRequest struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	CurrentStep BudgetStep `bson:"current_step" json:"current_step"`

	// Channel IDs (resolved at creation, stored for later updates)
	SaleChannelID     string `bson:"sale_channel_id" json:"sale_channel_id"`
	PartnerChannelID  string `bson:"partner_channel_id" json:"partner_channel_id"`
	TLQCChannelID     string `bson:"tlqc_channel_id" json:"tlqc_channel_id"`
	ApprovalChannelID string `bson:"approval_channel_id" json:"approval_channel_id"`
	FinanceChannelID  string `bson:"finance_channel_id" json:"finance_channel_id"`

	// Post IDs (stored to update posts across channels)
	SalePostID     string `bson:"sale_post_id" json:"sale_post_id"`
	PartnerPostID  string `bson:"partner_post_id" json:"partner_post_id"`
	TLQCPostID     string `bson:"tlqc_post_id" json:"tlqc_post_id"`
	ApprovalPostID string `bson:"approval_post_id" json:"approval_post_id"`
	FinancePostID  string `bson:"finance_post_id" json:"finance_post_id"`

	// Step 1: Sale Info
	SaleUserID string `bson:"sale_user_id" json:"sale_user_id"`
	Name       string `bson:"name" json:"name"`
	Partner    string `bson:"partner" json:"partner"`
	Amount     string `bson:"amount" json:"amount"`
	Purpose    string `bson:"purpose" json:"purpose"`
	Deadline   string `bson:"deadline" json:"deadline"`

	// Step 2: Partner Content
	PostContent   string     `bson:"post_content,omitempty" json:"post_content"`
	PostLink      string     `bson:"post_link,omitempty" json:"post_link"`
	PageLink      string     `bson:"page_link,omitempty" json:"page_link"`
	ContentAt     *time.Time `bson:"content_at,omitempty" json:"content_at"`

	// Step 3: TLQC Confirmation
	TLQCUserID    string     `bson:"tlqc_user_id,omitempty" json:"tlqc_user_id"`
	TLQCConfirmedAt *time.Time `bson:"tlqc_confirmed_at,omitempty" json:"tlqc_confirmed_at"`

	// Step 4: Payment Info
	RecipientName string     `bson:"recipient_name,omitempty" json:"recipient_name"`
	BankAccount   string     `bson:"bank_account,omitempty" json:"bank_account"`
	BankName      string     `bson:"bank_name,omitempty" json:"bank_name"`
	PaymentAmount string     `bson:"payment_amount,omitempty" json:"payment_amount"`
	PaymentAt     *time.Time `bson:"payment_at,omitempty" json:"payment_at"`

	// Step 5: Approval
	ApproverID string     `bson:"approver_id,omitempty" json:"approver_id"`
	ApprovedAt *time.Time `bson:"approved_at,omitempty" json:"approved_at"`

	// Step 6: Finance Completion
	FinanceUserID   string     `bson:"finance_user_id,omitempty" json:"finance_user_id"`
	TransactionCode string     `bson:"transaction_code,omitempty" json:"transaction_code"`
	BillURL         string     `bson:"bill_url,omitempty" json:"bill_url"`
	CompletedAt     *time.Time `bson:"completed_at,omitempty" json:"completed_at"`

	// Rejection
	RejectedAt *time.Time `bson:"rejected_at,omitempty" json:"rejected_at"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}
