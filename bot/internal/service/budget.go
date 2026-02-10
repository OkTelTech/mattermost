package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"oktel-bot/internal/i18n"
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
		TeamID:            channelInfo.TeamID,
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

	// Post info to budget-sale (no buttons)
	salePost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: channels.Sale,
		Message:   "@all",
		Props: mattermost.Props{
			MessageKey: "budget.msg.request_created",
			MessageData: map[string]any{
				"Name":    name,
				"Partner": partner,
				"Amount":  amount,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("post to sale channel: %w", err)
	}

	// Post to budget-partner-{partner} with content button
	partnerPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: channels.Partner,
		Message:   "@all",
		Props: mattermost.Props{
			MessageKey: "budget.msg.partner_fill_content",
			MessageData: map[string]any{
				"Name":    name,
				"Partner": partner,
				"Amount":  amount,
			},
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: i18n.T(ctx, "budget.btn.fill_content"),
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
	infoMsg := formatBudgetStatus(ctx, req, i18n.T(ctx, "budget.status.step2"))
	contentDetail := fmt.Sprintf("\n| %s | %s |\n| %s | %s |\n| %s | %s |",
		i18n.T(ctx, "budget.info.post_content"), postContent,
		i18n.T(ctx, "budget.info.post_link"), postLink,
		i18n.T(ctx, "budget.info.page_link"), pageLink)

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
					Name: i18n.T(ctx, "budget.btn.confirm"),
					Type: "button",
					Integration: mattermost.Integration{
						URL:     s.botURL + "/api/budget/tlqc-confirm",
						Context: map[string]any{"request_id": idHex},
					},
				},
				{
					Name: i18n.T(ctx, "budget.btn.return"),
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
			Message:   tlqcMention,
			Props: mattermost.Props{
				MessageKey: "budget.msg.partner_resubmit",
				MessageData: map[string]any{
					"Username": s.extractUsername(tlqcMention),
				},
			},
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
	infoMsg := formatBudgetStatus(ctx, req, i18n.T(ctx, "budget.status.step3"))

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
						Name: i18n.T(ctx, "budget.btn.fill_payment"),
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
		Message:   partnerMention,
		Props: mattermost.Props{
			MessageKey: "budget.msg.fill_payment",
			MessageData: map[string]any{
				"Username": s.extractUsername(partnerMention),
			},
		},
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
	infoMsg := formatBudgetStatus(ctx, req, i18n.T(ctx, "budget.status.returned"))

	// Update TLQC post (remove buttons)
	s.mm.UpdatePost(req.TLQCPostID, &mattermost.Post{
		ChannelID: req.TLQCChannelID,
		Message:   formatBudgetStatus(ctx, req, i18n.T(ctx, "budget.status.returned_rework")),
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
						Name: i18n.T(ctx, "budget.btn.fill_content"),
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
		Message:   partnerMention,
		Props: mattermost.Props{
			MessageKey: "budget.msg.content_returned",
			MessageData: map[string]any{
				"Username": s.extractUsername(partnerMention),
				"Reason":   reason,
			},
		},
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
	infoMsg := formatBudgetStatus(ctx, req, i18n.T(ctx, "budget.status.step4"))
	paymentDetail := fmt.Sprintf("\n| %s | %s |\n| %s | %s |\n| %s | %s |\n| %s | %s |",
		i18n.T(ctx, "budget.info.recipient"), recipientName,
		i18n.T(ctx, "budget.info.bank_account"), bankAccount,
		i18n.T(ctx, "budget.info.bank"), bankName,
		i18n.T(ctx, "budget.info.payment_amount"), paymentAmount)

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
		Message:   "@all",
		Props: mattermost.Props{
			MessageKey: "budget.msg.approval_review",
			MessageData: map[string]any{
				"Name":    req.Name,
				"Partner": req.Partner,
				"Amount":  req.Amount,
			},
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: i18n.T(ctx, "budget.btn.approve"),
						Type: "button",
						Integration: mattermost.Integration{
							URL:     s.botURL + "/api/budget/approval-approve",
							Context: map[string]any{"request_id": idHex},
						},
					},
					{
						Name: i18n.T(ctx, "budget.btn.reject"),
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
	infoMsg := formatBudgetStatus(ctx, req, i18n.T(ctx, "budget.status.step5"))

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
		Message:   "@all",
		Props: mattermost.Props{
			MessageKey: "budget.msg.finance_complete",
			MessageData: map[string]any{
				"Name":    req.Name,
				"Partner": req.Partner,
				"Amount":  req.Amount,
			},
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: i18n.T(ctx, "budget.btn.complete"),
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

	completedMsg := formatCompletedMsg(ctx, req)
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
		return fmt.Errorf(i18n.T(ctx, "budget.err.not_found"))
	}
	if req.CurrentStep >= model.BudgetStepCompleted {
		return fmt.Errorf(i18n.T(ctx, "budget.err.already_completed"))
	}
	if req.RejectedAt != nil {
		return fmt.Errorf(i18n.T(ctx, "budget.err.already_rejected"))
	}

	now := time.Now()
	req.RejectedAt = &now
	if err := s.store.Update(ctx, req); err != nil {
		return err
	}

	rejectedMsg := formatRejectedMsg(ctx, req)
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
		return nil, fmt.Errorf(i18n.T(ctx, "budget.err.not_found"))
	}
	if req.RejectedAt != nil {
		return nil, fmt.Errorf(i18n.T(ctx, "budget.err.been_rejected"))
	}
	if req.CurrentStep != expectedStep {
		return nil, fmt.Errorf(i18n.T(ctx, "budget.err.wrong_step", map[string]any{
			"Current": fmt.Sprintf("%d", req.CurrentStep), "Expected": fmt.Sprintf("%d", expectedStep),
		}))
	}
	return req, nil
}

func formatBudgetInfo(ctx context.Context, req *model.BudgetRequest) string {
	return fmt.Sprintf("%s\n| | |\n|:--|:--|\n| %s | %s |\n| %s | %s |\n| %s | %s |\n| %s | %s |\n| %s | %s |",
		i18n.T(ctx, "budget.header"),
		i18n.T(ctx, "budget.info.name"), req.Name,
		i18n.T(ctx, "budget.info.partner"), req.Partner,
		i18n.T(ctx, "budget.info.amount"), req.Amount,
		i18n.T(ctx, "budget.info.purpose"), req.Purpose,
		i18n.T(ctx, "budget.info.deadline"), req.Deadline)
}

func formatBudgetStatus(ctx context.Context, req *model.BudgetRequest, statusLabel string) string {
	return formatBudgetInfo(ctx, req) + fmt.Sprintf("\n| %s | %s |", i18n.T(ctx, "budget.info.status"), statusLabel)
}

func formatCompletedMsg(ctx context.Context, req *model.BudgetRequest) string {
	msg := formatBudgetInfo(ctx, req)
	msg += fmt.Sprintf("\n| %s | %s |", i18n.T(ctx, "budget.info.transaction_code"), req.TransactionCode)
	if req.BillURL != "" {
		msg += fmt.Sprintf("\n| %s | %s |", i18n.T(ctx, "budget.info.bill"), req.BillURL)
	}
	msg += fmt.Sprintf("\n| %s | %s |", i18n.T(ctx, "budget.info.status"), i18n.T(ctx, "budget.status.completed"))
	return msg
}

func formatRejectedMsg(ctx context.Context, req *model.BudgetRequest) string {
	return formatBudgetInfo(ctx, req) + fmt.Sprintf("\n| %s | %s |",
		i18n.T(ctx, "budget.info.status"),
		i18n.T(ctx, "budget.status.rejected", map[string]any{"Step": fmt.Sprintf("%d", req.CurrentStep)}))
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

// extractUsername removes the "@" prefix from a mention string
func (s *BudgetService) extractUsername(mention string) string {
	if len(mention) > 0 && mention[0] == '@' {
		return mention[1:]
	}
	return mention
}
