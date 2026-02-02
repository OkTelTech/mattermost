package service

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"oktel-bot/internal/mattermost"
	"oktel-bot/internal/model"
	"oktel-bot/internal/store"
)

type BudgetService struct {
	store  *store.BudgetStore
	mm     *mattermost.Client
	botURL string
}

func NewBudgetService(store *store.BudgetStore, mm *mattermost.Client, botURL string) *BudgetService {
	return &BudgetService{store: store, mm: mm, botURL: botURL}
}

// CreateRequest handles step 1: Sale creates a budget request.
func (s *BudgetService) CreateRequest(ctx context.Context, userID, channelID, campaign, partner, purpose, deadline string, amount float64) error {
	channelInfo, err := s.mm.GetChannel(channelID)
	if err != nil {
		return fmt.Errorf("get channel info: %w", err)
	}
	approvalChannelName := channelInfo.Name + approvalSuffix
	approvalChannelID, err := s.mm.GetChannelByName(channelInfo.TeamID, approvalChannelName)
	if err != nil {
		return fmt.Errorf("get approval channel '%s': %w", approvalChannelName, err)
	}

	// Create DB record first to get the ID
	req := &model.BudgetRequest{
		ChannelID:         channelID,
		ApprovalChannelID: approvalChannelID,
		CurrentStep:       1,
		Status:            "step1",
		SaleUserID:        userID,
		Campaign:          campaign,
		Partner:           partner,
		Amount:            amount,
		Purpose:           purpose,
		Deadline:          deadline,
	}
	if err := s.store.Create(ctx, req); err != nil {
		return fmt.Errorf("create budget request: %w", err)
	}

	idHex := req.ID.Hex()
	infoMsg := formatBudgetMsg(campaign, partner, amount, purpose, deadline, 1)

	infoPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: channelID,
		Message:   infoMsg,
	})
	if err != nil {
		return fmt.Errorf("post info message: %w", err)
	}

	approvalPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: approvalChannelID,
		Message:   infoMsg,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{{
					Name: "Fill Step 2 (Partner Content)",
					Type: "button",
					Integration: mattermost.Integration{
						URL:     s.botURL + "/api/budget/step2-form",
						Context: map[string]any{"request_id": idHex},
					},
				}},
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("post approval message: %w", err)
	}

	// Update record with post IDs
	req.PostID = infoPost.ID
	req.ApprovalPostID = approvalPost.ID
	return s.store.Update(ctx, req)
}

// SubmitStep2 handles Partner submitting content info.
func (s *BudgetService) SubmitStep2(ctx context.Context, requestID, postContent, postLink, pageLink string) error {
	req, err := s.getAndValidate(ctx, requestID, 1)
	if err != nil {
		return err
	}

	req.PostContent = postContent
	req.PostLink = postLink
	req.PageLink = pageLink
	req.CurrentStep = 2
	req.Status = "step2"

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	return s.updatePostWithNextStep(req, 2, "Confirm Step 3 (TLQC)", "/api/budget/step3")
}

// ConfirmStep3 handles TLQC confirmation.
func (s *BudgetService) ConfirmStep3(ctx context.Context, requestID, adAccountID, userID string) error {
	req, err := s.getAndValidate(ctx, requestID, 2)
	if err != nil {
		return err
	}

	req.AdAccountID = adAccountID
	req.TLQCUserID = userID
	req.TLQCConfirmed = true
	req.CurrentStep = 3
	req.Status = "step3"

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	return s.updatePostWithNextStep(req, 3, "Fill Step 4 (Payment Info)", "/api/budget/step4-form")
}

// SubmitStep4 handles Partner submitting payment info.
func (s *BudgetService) SubmitStep4(ctx context.Context, requestID, recipientName, bankAccount, bankName string, paymentAmount float64) error {
	req, err := s.getAndValidate(ctx, requestID, 3)
	if err != nil {
		return err
	}

	req.RecipientName = recipientName
	req.BankAccount = bankAccount
	req.BankName = bankName
	req.PaymentAmount = paymentAmount
	req.CurrentStep = 4
	req.Status = "step4"

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	return s.updatePostWithNextStep(req, 4, "Approve Step 5 (Team Lead)", "/api/budget/step5")
}

// ApproveStep5 handles Team Lead approval.
func (s *BudgetService) ApproveStep5(ctx context.Context, requestID, userID string) error {
	req, err := s.getAndValidate(ctx, requestID, 4)
	if err != nil {
		return err
	}

	req.TeamLeadID = userID
	req.Approved = true
	req.CurrentStep = 5
	req.Status = "step5"

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	return s.updatePostWithNextStep(req, 5, "Fill Step 6 (Bank Note)", "/api/budget/step6-form")
}

// SubmitStep6 handles TL Bank adding note.
func (s *BudgetService) SubmitStep6(ctx context.Context, requestID, bankNote string) error {
	req, err := s.getAndValidate(ctx, requestID, 5)
	if err != nil {
		return err
	}

	req.BankNote = bankNote
	req.CurrentStep = 6
	req.Status = "step6"

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	return s.updatePostWithNextStep(req, 6, "Complete Step 7 (Finance)", "/api/budget/step7-form")
}

// CompleteStep7 handles Finance completing the request.
func (s *BudgetService) CompleteStep7(ctx context.Context, requestID, transactionCode string) error {
	req, err := s.getAndValidate(ctx, requestID, 6)
	if err != nil {
		return err
	}

	now := time.Now()
	req.TransactionCode = transactionCode
	req.CompletedAt = &now
	req.CurrentStep = 7
	req.Status = "completed"

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	completedMsg := fmt.Sprintf("**BUDGET REQUEST - COMPLETED**\n| | |\n|:--|:--|\n| Campaign | %s |\n| Partner | %s |\n| Amount | %.0f |\n| Transaction | %s |\n| Status | **COMPLETED** |",
		req.Campaign, req.Partner, req.Amount, transactionCode)

	s.mm.UpdatePost(req.ApprovalPostID, &mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   completedMsg,
	})
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   completedMsg,
	})

	return nil
}

// RejectRequest rejects a budget request at any step.
func (s *BudgetService) RejectRequest(ctx context.Context, requestID, userID string) error {
	id, err := bson.ObjectIDFromHex(requestID)
	if err != nil {
		return fmt.Errorf("invalid request ID: %w", err)
	}

	req, err := s.store.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if req == nil {
		return fmt.Errorf("budget request not found")
	}
	if req.Status == "completed" || req.Status == "rejected" {
		return fmt.Errorf("request is already %s", req.Status)
	}

	req.Status = "rejected"
	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	rejectedMsg := fmt.Sprintf("**BUDGET REQUEST - REJECTED**\n| | |\n|:--|:--|\n| Campaign | %s |\n| Partner | %s |\n| Amount | %.0f |\n| Status | **REJECTED** at step %d |",
		req.Campaign, req.Partner, req.Amount, req.CurrentStep)

	s.mm.UpdatePost(req.ApprovalPostID, &mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   rejectedMsg,
	})
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   rejectedMsg,
	})

	return nil
}

func (s *BudgetService) getAndValidate(ctx context.Context, requestID string, expectedStep int) (*model.BudgetRequest, error) {
	id, err := bson.ObjectIDFromHex(requestID)
	if err != nil {
		return nil, fmt.Errorf("invalid request ID: %w", err)
	}

	req, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, fmt.Errorf("budget request not found")
	}
	if req.CurrentStep != expectedStep {
		return nil, fmt.Errorf("request is at step %d, expected step %d", req.CurrentStep, expectedStep)
	}
	return req, nil
}

func (s *BudgetService) updatePostWithNextStep(req *model.BudgetRequest, completedStep int, nextButtonLabel, nextURL string) error {
	msg := formatBudgetMsg(req.Campaign, req.Partner, req.Amount, req.Purpose, req.Deadline, completedStep+1)
	idHex := req.ID.Hex()

	// Update info post (no buttons)
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   msg,
	})

	// Update approval post with next step button
	s.mm.UpdatePost(req.ApprovalPostID, &mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   msg,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: nextButtonLabel,
						Type: "button",
						Integration: mattermost.Integration{
							URL:     s.botURL + nextURL,
							Context: map[string]any{"request_id": idHex},
						},
					},
					{
						Name: "Reject",
						Type: "button",
						Integration: mattermost.Integration{
							URL:     s.botURL + "/api/budget/reject",
							Context: map[string]any{"request_id": idHex},
						},
					},
				},
			}},
		},
	})

	return nil
}

func formatBudgetMsg(campaign, partner string, amount float64, purpose, deadline string, step int) string {
	stepNames := []string{
		"", "Sale Created", "Partner Content", "TLQC Confirmed",
		"Payment Info", "Team Lead Approved", "Bank Note Added", "Completed",
	}
	stepLabel := "PENDING"
	if step > 0 && step <= len(stepNames)-1 {
		stepLabel = fmt.Sprintf("Step %d/7 - %s", step, stepNames[step])
	}

	return fmt.Sprintf("**BUDGET REQUEST**\n| | |\n|:--|:--|\n| Campaign | %s |\n| Partner | %s |\n| Amount | %.0f |\n| Purpose | %s |\n| Deadline | %s |\n| Status | %s |",
		campaign, partner, amount, purpose, deadline, stepLabel)
}
