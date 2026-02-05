package service

import (
	"context"
	"fmt"
	"strings"
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

// channelIDs holds the resolved IDs for all budget channels.
type channelIDs struct {
	Sale     string
	Partner  string
	TLQC     string
	Approval string
	Finance  string
}

func (s *BudgetService) resolveChannels(teamID, partner, suffix, saleChannelID string) (*channelIDs, error) {
	partnerCh := model.PartnerChannelName(partner) + suffix
	partnerID, err := s.mm.GetChannelByName(teamID, partnerCh)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", partnerCh, err)
	}
	tlqcCh := model.BudgetTLQCChannel + suffix
	tlqcID, err := s.mm.GetChannelByName(teamID, tlqcCh)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", tlqcCh, err)
	}
	approvalCh := model.BudgetApprovalChannel + suffix
	approvalID, err := s.mm.GetChannelByName(teamID, approvalCh)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", approvalCh, err)
	}
	financeCh := model.BudgetFinanceChannel + suffix
	financeID, err := s.mm.GetChannelByName(teamID, financeCh)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", financeCh, err)
	}
	return &channelIDs{
		Sale:     saleChannelID,
		Partner:  partnerID,
		TLQC:     tlqcID,
		Approval: approvalID,
		Finance:  financeID,
	}, nil
}

// CreateRequest handles step 1: Sale creates a budget request from budget-sale channel.
func (s *BudgetService) CreateRequest(ctx context.Context, userID, channelID, name, partner, amount, purpose, deadline string) error {
	// Extract suffix and teamID from sale channel (e.g. "budget-sale-dev" → suffix "-dev")
	channelInfo, err := s.mm.GetChannel(channelID)
	if err != nil {
		return fmt.Errorf("get channel info: %w", err)
	}
	suffix := strings.TrimPrefix(channelInfo.Name, model.BudgetSaleChannel)

	channels, err := s.resolveChannels(channelInfo.TeamID, partner, suffix, channelID)
	if err != nil {
		return fmt.Errorf("resolve channels: %w", err)
	}

	req := &model.BudgetRequest{
		CurrentStep:       model.BudgetStepSaleCreated,
		SaleChannelID:     channels.Sale,
		PartnerChannelID:  channels.Partner,
		TLQCChannelID:     channels.TLQC,
		ApprovalChannelID: channels.Approval,
		FinanceChannelID:  channels.Finance,
		SaleUserID:        userID,
		Name:              name,
		Partner:           partner,
		Amount:            amount,
		Purpose:           purpose,
		Deadline:          deadline,
	}
	if err := s.store.Create(ctx, req); err != nil {
		return fmt.Errorf("create budget request: %w", err)
	}

	idHex := req.ID.Hex()
	infoMsg := formatBudgetStatus(req, "Step 1/6 - Sale Created")

	// Post info to budget-sale (no buttons)
	salePost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: channels.Sale,
		Message:   "@all\n" + infoMsg,
	})
	if err != nil {
		return fmt.Errorf("post to sale channel: %w", err)
	}

	// Post to budget-partner-{partner} with content button
	partnerPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: channels.Partner,
		Message:   "@all\n" + infoMsg,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: "Fill Post Content",
						Type: "button",
						Integration: mattermost.Integration{
							URL:     s.botURL + "/api/budget/partner-content-form",
							Context: map[string]any{"request_id": idHex},
						},
					},
				},
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("post to partner channel: %w", err)
	}

	req.SalePostID = salePost.ID
	req.PartnerPostID = partnerPost.ID
	return s.store.Update(ctx, req)
}

// SubmitContent handles step 2: Partner submits content info from budget-partner-{partner}.
func (s *BudgetService) SubmitContent(ctx context.Context, requestID, userID, postContent, postLink, pageLink string) error {
	req, err := s.getAndValidate(ctx, requestID, model.BudgetStepSaleCreated)
	if err != nil {
		return err
	}

	now := time.Now()
	req.PartnerUserID = userID
	req.PostContent = postContent
	req.PostLink = postLink
	req.PageLink = pageLink
	req.ContentAt = &now
	req.CurrentStep = model.BudgetStepPartnerContent

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	idHex := req.ID.Hex()
	infoMsg := formatBudgetStatus(req, "Step 2/6 - Partner Content Added")
	contentDetail := fmt.Sprintf("\n| **Post Content** | %s |\n| **Post Link** | %s |\n| **Page Link** | %s |",
		postContent, postLink, pageLink)

	// Update partner post (remove button, show content)
	s.mm.UpdatePost(req.PartnerPostID, &mattermost.Post{
		ChannelID: req.PartnerChannelID,
		Message:   infoMsg + contentDetail,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{},
		},
	})

	tlqcProps := mattermost.Props{
		Attachments: []mattermost.Attachment{{
			Actions: []mattermost.Action{
				{
					Name: "Confirm",
					Type: "button",
					Integration: mattermost.Integration{
						URL:     s.botURL + "/api/budget/tlqc-confirm",
						Context: map[string]any{"request_id": idHex},
					},
				},
				{
					Name: "Return to Partner",
					Type: "button",
					Integration: mattermost.Integration{
						URL:     s.botURL + "/api/budget/tlqc-return-form",
						Context: map[string]any{"request_id": idHex},
					},
				},
			},
		}},
	}

	if req.TLQCPostID != "" {
		// Resubmit: update existing TLQC post and notify in thread
		s.mm.UpdatePost(req.TLQCPostID, &mattermost.Post{
			ChannelID: req.TLQCChannelID,
			Message:   infoMsg + contentDetail,
			Props:     tlqcProps,
		})
		tlqcMention := s.userMention(req.TLQCUserID)
		s.mm.CreatePost(&mattermost.Post{
			ChannelID: req.TLQCChannelID,
			RootID:    req.TLQCPostID,
			Message:   tlqcMention + " Partner has resubmitted the content. Please review.",
		})
	} else {
		// First submit: create new TLQC post
		tlqcPost, err := s.mm.CreatePost(&mattermost.Post{
			ChannelID: req.TLQCChannelID,
			Message:   "@all\n" + infoMsg + contentDetail,
			Props:     tlqcProps,
		})
		if err != nil {
			return fmt.Errorf("post to tlqc channel: %w", err)
		}
		req.TLQCPostID = tlqcPost.ID
	}

	// Update sale post status
	s.mm.UpdatePost(req.SalePostID, &mattermost.Post{
		ChannelID: req.SaleChannelID,
		Message:   infoMsg,
	})

	return s.store.Update(ctx, req)
}

// ConfirmTLQC handles step 3: TLQC confirms from budget-tlqc (button only, no dialog).
func (s *BudgetService) ConfirmTLQC(ctx context.Context, requestID, userID string) error {
	req, err := s.getAndValidate(ctx, requestID, model.BudgetStepPartnerContent)
	if err != nil {
		return err
	}

	now := time.Now()
	req.TLQCUserID = userID
	req.TLQCConfirmedAt = &now
	req.CurrentStep = model.BudgetStepTLQCConfirmed

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	idHex := req.ID.Hex()
	infoMsg := formatBudgetStatus(req, "Step 3/6 - TLQC Confirmed")

	// Update TLQC post (remove button)
	s.mm.UpdatePost(req.TLQCPostID, &mattermost.Post{
		ChannelID: req.TLQCChannelID,
		Message:   infoMsg,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{},
		},
	})

	// Update partner post with payment button
	s.mm.UpdatePost(req.PartnerPostID, &mattermost.Post{
		ChannelID: req.PartnerChannelID,
		Message:   infoMsg,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: "Fill Payment Info",
						Type: "button",
						Integration: mattermost.Integration{
							URL:     s.botURL + "/api/budget/partner-payment-form",
							Context: map[string]any{"request_id": idHex},
						},
					},
				},
			}},
		},
	})

	// Notify partner user in thread to fill payment info
	partnerMention := s.userMention(req.PartnerUserID)
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.PartnerChannelID,
		RootID:    req.PartnerPostID,
		Message:   partnerMention + " Content confirmed by TLQC. Please fill in the payment info.",
	})

	// Update sale post status
	s.mm.UpdatePost(req.SalePostID, &mattermost.Post{
		ChannelID: req.SaleChannelID,
		Message:   infoMsg,
	})

	return nil
}

// ReturnToPartner handles TLQC returning content to partner for rework.
func (s *BudgetService) ReturnToPartner(ctx context.Context, requestID, userID, reason string) error {
	req, err := s.getAndValidate(ctx, requestID, model.BudgetStepPartnerContent)
	if err != nil {
		return err
	}

	partnerMention := s.userMention(req.PartnerUserID)

	// Reset to step 1 so partner can resubmit content; keep TLQCPostID for reuse
	req.TLQCUserID = userID
	req.PartnerUserID = ""
	req.PostContent = ""
	req.PostLink = ""
	req.PageLink = ""
	req.ContentAt = nil
	req.CurrentStep = model.BudgetStepSaleCreated

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	idHex := req.ID.Hex()
	infoMsg := formatBudgetStatus(req, "Step 1/6 - Returned by TLQC, please redo content")

	// Update TLQC post (remove buttons)
	s.mm.UpdatePost(req.TLQCPostID, &mattermost.Post{
		ChannelID: req.TLQCChannelID,
		Message:   formatBudgetStatus(req, "Returned to Partner for rework"),
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{},
		},
	})

	// Update partner post with Fill Post Content button again
	s.mm.UpdatePost(req.PartnerPostID, &mattermost.Post{
		ChannelID: req.PartnerChannelID,
		Message:   infoMsg,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: "Fill Post Content",
						Type: "button",
						Integration: mattermost.Integration{
							URL:     s.botURL + "/api/budget/partner-content-form",
							Context: map[string]any{"request_id": idHex},
						},
					},
				},
			}},
		},
	})

	// Notify partner in thread with reason
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.PartnerChannelID,
		RootID:    req.PartnerPostID,
		Message:   fmt.Sprintf("%s Content returned by TLQC. Please redo and resubmit.\n**Reason:** %s", partnerMention, reason),
	})

	// Update sale post status
	s.mm.UpdatePost(req.SalePostID, &mattermost.Post{
		ChannelID: req.SaleChannelID,
		Message:   infoMsg,
	})

	return nil
}

// SubmitPayment handles step 4: Partner submits payment info from budget-partner-{partner}.
func (s *BudgetService) SubmitPayment(ctx context.Context, requestID, recipientName, bankAccount, bankName, paymentAmount string) error {
	req, err := s.getAndValidate(ctx, requestID, model.BudgetStepTLQCConfirmed)
	if err != nil {
		return err
	}

	now := time.Now()
	req.RecipientName = recipientName
	req.BankAccount = bankAccount
	req.BankName = bankName
	req.PaymentAmount = paymentAmount
	req.PaymentAt = &now
	req.CurrentStep = model.BudgetStepPaymentInfo

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	idHex := req.ID.Hex()
	infoMsg := formatBudgetStatus(req, "Step 4/6 - Payment Info Added")
	paymentDetail := fmt.Sprintf("\n| **Recipient** | %s |\n| **Bank Account** | %s |\n| **Bank** | %s |\n| **Payment Amount** | %s |",
		recipientName, bankAccount, bankName, paymentAmount)

	// Update partner post (remove button, show payment info)
	s.mm.UpdatePost(req.PartnerPostID, &mattermost.Post{
		ChannelID: req.PartnerChannelID,
		Message:   infoMsg + paymentDetail,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{},
		},
	})

	// Create post in budget-approval with approve + reject buttons
	approvalPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   "@all\n" + infoMsg + paymentDetail,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: "Approve",
						Type: "button",
						Integration: mattermost.Integration{
							URL:     s.botURL + "/api/budget/approval-approve",
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
	if err != nil {
		return fmt.Errorf("post to approval channel: %w", err)
	}

	// Update sale post status
	s.mm.UpdatePost(req.SalePostID, &mattermost.Post{
		ChannelID: req.SaleChannelID,
		Message:   infoMsg,
	})

	req.ApprovalPostID = approvalPost.ID
	return s.store.Update(ctx, req)
}

// Approve handles step 5: Approver approves from budget-approval (button only).
func (s *BudgetService) Approve(ctx context.Context, requestID, userID string) error {
	req, err := s.getAndValidate(ctx, requestID, model.BudgetStepPaymentInfo)
	if err != nil {
		return err
	}

	now := time.Now()
	req.ApproverID = userID
	req.ApprovedAt = &now
	req.CurrentStep = model.BudgetStepApproved

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	idHex := req.ID.Hex()
	infoMsg := formatBudgetStatus(req, "Step 5/6 - Approved")

	// Update approval post (remove buttons)
	s.mm.UpdatePost(req.ApprovalPostID, &mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   infoMsg,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{},
		},
	})

	// Create post in budget-finance with complete button
	financePost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.FinanceChannelID,
		Message:   "@all\n" + infoMsg,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: "Complete",
						Type: "button",
						Integration: mattermost.Integration{
							URL:     s.botURL + "/api/budget/finance-complete-form",
							Context: map[string]any{"request_id": idHex},
						},
					},
				},
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("post to finance channel: %w", err)
	}

	// Update sale post status
	s.mm.UpdatePost(req.SalePostID, &mattermost.Post{
		ChannelID: req.SaleChannelID,
		Message:   infoMsg,
	})

	req.FinancePostID = financePost.ID
	return s.store.Update(ctx, req)
}

// Complete handles step 6: Finance completes from budget-finance.
func (s *BudgetService) Complete(ctx context.Context, requestID, transactionCode, billURL, userID string) error {
	req, err := s.getAndValidate(ctx, requestID, model.BudgetStepApproved)
	if err != nil {
		return err
	}

	now := time.Now()
	req.FinanceUserID = userID
	req.TransactionCode = transactionCode
	req.BillURL = billURL
	req.CompletedAt = &now
	req.CurrentStep = model.BudgetStepCompleted

	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	completedMsg := formatCompletedMsg(req)
	s.updateAllPosts(req, completedMsg)
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
	if req.CurrentStep >= model.BudgetStepCompleted {
		return fmt.Errorf("request is already completed")
	}
	if req.RejectedAt != nil {
		return fmt.Errorf("request is already rejected")
	}

	now := time.Now()
	req.RejectedAt = &now
	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	rejectedMsg := formatRejectedMsg(req)
	s.updateAllPosts(req, rejectedMsg)
	return nil
}

// updateAllPosts updates all existing posts across all channels with the given message.
func (s *BudgetService) updateAllPosts(req *model.BudgetRequest, msg string) {
	posts := []struct{ postID, channelID string }{
		{req.SalePostID, req.SaleChannelID},
		{req.PartnerPostID, req.PartnerChannelID},
		{req.TLQCPostID, req.TLQCChannelID},
		{req.ApprovalPostID, req.ApprovalChannelID},
		{req.FinancePostID, req.FinanceChannelID},
	}
	for _, p := range posts {
		if p.postID != "" {
			s.mm.UpdatePost(p.postID, &mattermost.Post{
				ChannelID: p.channelID,
				Message:   msg,
				Props: mattermost.Props{
					Attachments: []mattermost.Attachment{},
				},
			})
		}
	}
}

func (s *BudgetService) getAndValidate(ctx context.Context, requestID string, expectedStep model.BudgetStep) (*model.BudgetRequest, error) {
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
	if req.RejectedAt != nil {
		return nil, fmt.Errorf("request has been rejected")
	}
	if req.CurrentStep != expectedStep {
		return nil, fmt.Errorf("request is at step %d, expected step %d", req.CurrentStep, expectedStep)
	}
	return req, nil
}

func formatBudgetInfo(req *model.BudgetRequest) string {
	return fmt.Sprintf("#### Budget Request\n| | |\n|:--|:--|\n| **Name** | %s |\n| **Partner** | %s |\n| **Amount** | %s |\n| **Purpose** | %s |\n| **Deadline** | %s |",
		req.Name, req.Partner, req.Amount, req.Purpose, req.Deadline)
}

func formatBudgetStatus(req *model.BudgetRequest, statusLabel string) string {
	return formatBudgetInfo(req) + fmt.Sprintf("\n| **Status** | %s |", statusLabel)
}

func formatCompletedMsg(req *model.BudgetRequest) string {
	msg := formatBudgetInfo(req)
	msg += fmt.Sprintf("\n| **Transaction Code** | %s |", req.TransactionCode)
	if req.BillURL != "" {
		msg += fmt.Sprintf("\n| **Bill** | %s |", req.BillURL)
	}
	msg += "\n| **Status** | **COMPLETED** |"
	return msg
}

func formatRejectedMsg(req *model.BudgetRequest) string {
	return formatBudgetInfo(req) + fmt.Sprintf("\n| **Status** | **REJECTED** at step %d |", req.CurrentStep)
}

// userMention returns a @username mention for a user ID, falling back to @all on error.
func (s *BudgetService) userMention(userID string) string {
	if userID == "" {
		return "@all"
	}
	user, err := s.mm.GetUser(userID)
	if err != nil {
		return "@all"
	}
	return "@" + user.Username
}
