package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"oktel-bot/internal/i18n"
	"oktel-bot/internal/mattermost"
	"oktel-bot/internal/service"
)

type BudgetHandler struct {
	svc    *service.BudgetService
	mm     *mattermost.Client
	botURL string
}

func NewBudgetHandler(svc *service.BudgetService, mm *mattermost.Client, botURL string) *BudgetHandler {
	return &BudgetHandler{svc: svc, mm: mm, botURL: botURL}
}

// localeCtx fetches the user's locale from Mattermost and returns a context with locale set.
func (h *BudgetHandler) localeCtx(ctx context.Context, userID string) context.Context {
	user, err := h.mm.GetUser(userID)
	if err != nil {
		log.Printf("i18n: GetUser(%s) failed: %v", userID, err)
		return ctx
	}
	if user.Locale == "" {
		return ctx
	}
	return i18n.WithLocale(ctx, user.Locale)
}

// HandleSlashCommand handles /budget slash command - opens create dialog.
func (h *BudgetHandler) HandleSlashCommand(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ctx := h.localeCtx(r.Context(), r.FormValue("user_id"))

	channelName := r.FormValue("channel_name")
	if !strings.HasPrefix(channelName, "budget") {
		writeJSON(w, SlashResponse{
			ResponseType: "ephemeral",
			Text:         i18n.T(ctx, "budget.channel_error"),
		})
		return
	}

	triggerID := r.FormValue("trigger_id")
	if triggerID == "" {
		writeJSON(w, SlashResponse{
			ResponseType: "ephemeral",
			Text:         i18n.T(ctx, "budget.err.missing_trigger"),
		})
		return
	}

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: triggerID,
		URL:       h.botURL + "/api/budget/sale-create",
		Dialog: mattermost.Dialog{
			Title:       i18n.T(ctx, "budget.dialog.create_title"),
			SubmitLabel: i18n.T(ctx, "budget.dialog.submit"),
			Elements: []mattermost.DialogElement{
				{DisplayName: i18n.T(ctx, "budget.field.name"), Name: "name", Type: "text"},
				{DisplayName: i18n.T(ctx, "budget.field.partner"), Name: "partner", Type: "text", Placeholder: i18n.T(ctx, "budget.placeholder.partner")},
				{DisplayName: i18n.T(ctx, "budget.field.amount"), Name: "amount", Type: "text", Placeholder: i18n.T(ctx, "budget.placeholder.amount")},
				{DisplayName: i18n.T(ctx, "budget.field.purpose"), Name: "purpose", Type: "textarea"},
				{DisplayName: i18n.T(ctx, "budget.field.deadline"), Name: "deadline", Type: "text", SubType: "date", Placeholder: i18n.T(ctx, "budget.placeholder.deadline")},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open budget dialog: %v", err)
		writeJSON(w, SlashResponse{
			ResponseType: "ephemeral",
			Text:         i18n.T(ctx, "budget.err.open_form"),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleSaleCreate processes the budget creation dialog submission.
func (h *BudgetHandler) HandleSaleCreate(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := h.localeCtx(r.Context(), sub.UserID)

	err := h.svc.CreateRequest(
		ctx,
		sub.UserID,
		sub.ChannelID,
		sub.Submission["name"],
		sub.Submission["partner"],
		sub.Submission["amount"],
		sub.Submission["purpose"],
		sub.Submission["deadline"],
	)
	if err != nil {
		log.Printf("ERROR create budget request: %v", err)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandlePartnerContentForm opens the content dialog for partner.
func (h *BudgetHandler) HandlePartnerContentForm(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ctx := h.localeCtx(r.Context(), req.UserID)
	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/partner-content",
		Dialog: mattermost.Dialog{
			Title:       i18n.T(ctx, "budget.dialog.content_title"),
			CallbackID:  requestID,
			SubmitLabel: i18n.T(ctx, "budget.dialog.submit"),
			Elements: []mattermost.DialogElement{
				{DisplayName: i18n.T(ctx, "budget.field.post_content"), Name: "post_content", Type: "textarea"},
				{DisplayName: i18n.T(ctx, "budget.field.post_link"), Name: "post_link", Type: "text", Optional: true},
				{DisplayName: i18n.T(ctx, "budget.field.page_link"), Name: "page_link", Type: "text", Optional: true},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open partner content dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: i18n.T(ctx, "budget.err.open_form")})
		return
	}
	writeJSON(w, ActionResponse{})
}

// HandlePartnerContentSubmit processes the partner content dialog submission.
func (h *BudgetHandler) HandlePartnerContentSubmit(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := h.localeCtx(r.Context(), sub.UserID)

	err := h.svc.SubmitContent(
		ctx,
		sub.CallbackID,
		sub.UserID,
		sub.Submission["post_content"],
		sub.Submission["post_link"],
		sub.Submission["page_link"],
	)
	if err != nil {
		log.Printf("ERROR submit partner content: %v", err)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleTLQCConfirm handles TLQC confirmation (button click, no dialog).
func (h *BudgetHandler) HandleTLQCConfirm(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ctx := h.localeCtx(r.Context(), req.UserID)
	requestID, _ := req.Context["request_id"].(string)

	err := h.svc.ConfirmTLQC(ctx, requestID, req.UserID)
	if err != nil {
		writeJSON(w, ActionResponse{EphemeralText: err.Error()})
		return
	}
	writeJSON(w, ActionResponse{EphemeralText: i18n.T(ctx, "budget.msg.tlqc_confirmed")})
}

// HandleTLQCReturnForm opens the return reason dialog for TLQC.
func (h *BudgetHandler) HandleTLQCReturnForm(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ctx := h.localeCtx(r.Context(), req.UserID)
	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/tlqc-return",
		Dialog: mattermost.Dialog{
			Title:       i18n.T(ctx, "budget.dialog.return_title"),
			CallbackID:  requestID,
			SubmitLabel: i18n.T(ctx, "budget.dialog.return"),
			Elements: []mattermost.DialogElement{
				{DisplayName: i18n.T(ctx, "budget.field.reason"), Name: "reason", Type: "textarea"},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open tlqc return dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: i18n.T(ctx, "budget.err.open_form")})
		return
	}
	writeJSON(w, ActionResponse{})
}

// HandleTLQCReturnSubmit processes the TLQC return dialog submission.
func (h *BudgetHandler) HandleTLQCReturnSubmit(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := h.localeCtx(r.Context(), sub.UserID)

	err := h.svc.ReturnToPartner(ctx, sub.CallbackID, sub.UserID, sub.Submission["reason"])
	if err != nil {
		log.Printf("ERROR return to partner: %v", err)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandlePartnerPaymentForm opens the payment info dialog for partner.
func (h *BudgetHandler) HandlePartnerPaymentForm(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ctx := h.localeCtx(r.Context(), req.UserID)
	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/partner-payment",
		Dialog: mattermost.Dialog{
			Title:       i18n.T(ctx, "budget.dialog.payment_title"),
			CallbackID:  requestID,
			SubmitLabel: i18n.T(ctx, "budget.dialog.submit"),
			Elements: []mattermost.DialogElement{
				{DisplayName: i18n.T(ctx, "budget.field.recipient"), Name: "recipient_name", Type: "text"},
				{DisplayName: i18n.T(ctx, "budget.field.bank_account"), Name: "bank_account", Type: "text"},
				{DisplayName: i18n.T(ctx, "budget.field.bank_name"), Name: "bank_name", Type: "text"},
				{DisplayName: i18n.T(ctx, "budget.field.payment_amount"), Name: "payment_amount", Type: "text", Placeholder: i18n.T(ctx, "budget.placeholder.amount")},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open partner payment dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: i18n.T(ctx, "budget.err.open_form")})
		return
	}
	writeJSON(w, ActionResponse{})
}

// HandlePartnerPaymentSubmit processes the partner payment dialog submission.
func (h *BudgetHandler) HandlePartnerPaymentSubmit(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := h.localeCtx(r.Context(), sub.UserID)

	err := h.svc.SubmitPayment(
		ctx,
		sub.CallbackID,
		sub.Submission["recipient_name"],
		sub.Submission["bank_account"],
		sub.Submission["bank_name"],
		sub.Submission["payment_amount"],
	)
	if err != nil {
		log.Printf("ERROR submit partner payment: %v", err)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleApprovalApprove handles approval (button click, no dialog).
func (h *BudgetHandler) HandleApprovalApprove(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ctx := h.localeCtx(r.Context(), req.UserID)
	requestID, _ := req.Context["request_id"].(string)

	err := h.svc.Approve(ctx, requestID, req.UserID)
	if err != nil {
		writeJSON(w, ActionResponse{EphemeralText: err.Error()})
		return
	}
	writeJSON(w, ActionResponse{EphemeralText: i18n.T(ctx, "budget.msg.approved")})
}

// HandleFinanceCompleteForm opens the completion dialog for finance.
func (h *BudgetHandler) HandleFinanceCompleteForm(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ctx := h.localeCtx(r.Context(), req.UserID)
	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/finance-complete",
		Dialog: mattermost.Dialog{
			Title:       i18n.T(ctx, "budget.dialog.complete_title"),
			CallbackID:  requestID,
			SubmitLabel: i18n.T(ctx, "budget.dialog.complete"),
			Elements: []mattermost.DialogElement{
				{DisplayName: i18n.T(ctx, "budget.field.transaction_code"), Name: "transaction_code", Type: "text"},
				{DisplayName: i18n.T(ctx, "budget.field.bill_url"), Name: "bill_url", Type: "text", Optional: true, Placeholder: i18n.T(ctx, "budget.placeholder.bill")},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open finance complete dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: i18n.T(ctx, "budget.err.open_form")})
		return
	}
	writeJSON(w, ActionResponse{})
}

// HandleFinanceCompleteSubmit processes the finance completion dialog submission.
func (h *BudgetHandler) HandleFinanceCompleteSubmit(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := h.localeCtx(r.Context(), sub.UserID)

	err := h.svc.Complete(
		ctx,
		sub.CallbackID,
		sub.Submission["transaction_code"],
		sub.Submission["bill_url"],
		sub.UserID,
	)
	if err != nil {
		log.Printf("ERROR complete budget: %v", err)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleReject handles rejecting a budget request at any step.
func (h *BudgetHandler) HandleReject(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	ctx := h.localeCtx(r.Context(), req.UserID)
	requestID, _ := req.Context["request_id"].(string)

	err := h.svc.RejectRequest(ctx, requestID, req.UserID)
	if err != nil {
		writeJSON(w, ActionResponse{EphemeralText: err.Error()})
		return
	}
	writeJSON(w, ActionResponse{EphemeralText: i18n.T(ctx, "budget.msg.rejected")})
}

// RegisterRoutes registers all budget routes on the given mux.
func (h *BudgetHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/budget", h.HandleSlashCommand)

	// Sale creates request
	mux.HandleFunc("POST /api/budget/sale-create", h.HandleSaleCreate)

	// Partner content
	mux.HandleFunc("POST /api/budget/partner-content-form", h.HandlePartnerContentForm)
	mux.HandleFunc("POST /api/budget/partner-content", h.HandlePartnerContentSubmit)

	// TLQC confirmation + return
	mux.HandleFunc("POST /api/budget/tlqc-confirm", h.HandleTLQCConfirm)
	mux.HandleFunc("POST /api/budget/tlqc-return-form", h.HandleTLQCReturnForm)
	mux.HandleFunc("POST /api/budget/tlqc-return", h.HandleTLQCReturnSubmit)

	// Partner payment
	mux.HandleFunc("POST /api/budget/partner-payment-form", h.HandlePartnerPaymentForm)
	mux.HandleFunc("POST /api/budget/partner-payment", h.HandlePartnerPaymentSubmit)

	// Approval
	mux.HandleFunc("POST /api/budget/approval-approve", h.HandleApprovalApprove)

	// Finance completion
	mux.HandleFunc("POST /api/budget/finance-complete-form", h.HandleFinanceCompleteForm)
	mux.HandleFunc("POST /api/budget/finance-complete", h.HandleFinanceCompleteSubmit)

	// Reject
	mux.HandleFunc("POST /api/budget/reject", h.HandleReject)
}
