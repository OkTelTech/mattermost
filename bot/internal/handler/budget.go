package handler

import (
	"encoding/json"
	"log"
	"net/http"

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

// HandleSlashCommand handles /budget slash command - opens create dialog.
func (h *BudgetHandler) HandleSlashCommand(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	triggerID := r.FormValue("trigger_id")
	if triggerID == "" {
		writeJSON(w, SlashResponse{
			ResponseType: "ephemeral",
			Text:         "Missing trigger_id. Please try again.",
		})
		return
	}

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: triggerID,
		URL:       h.botURL + "/api/budget/sale-create",
		Dialog: mattermost.Dialog{
			Title:       "Budget Request",
			SubmitLabel: "Submit",
			Elements: []mattermost.DialogElement{
				{DisplayName: "Name", Name: "name", Type: "text"},
				{DisplayName: "Partner", Name: "partner", Type: "text", Placeholder: "e.g. facebook"},
				{DisplayName: "Amount", Name: "amount", Type: "text", Placeholder: "e.g. 30$ or 30VND"},
				{DisplayName: "Purpose", Name: "purpose", Type: "textarea"},
				{DisplayName: "Deadline", Name: "deadline", Type: "text", SubType: "date", Placeholder: "YYYY-MM-DD"},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open budget dialog: %v", err)
		writeJSON(w, SlashResponse{
			ResponseType: "ephemeral",
			Text:         "Failed to open form. Please try again.",
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

	err := h.svc.CreateRequest(
		r.Context(),
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

	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/partner-content",
		Dialog: mattermost.Dialog{
			Title:       "Budget - Post Content",
			CallbackID:  requestID,
			SubmitLabel: "Submit",
			Elements: []mattermost.DialogElement{
				{DisplayName: "Post Content", Name: "post_content", Type: "textarea"},
				{DisplayName: "Post Link", Name: "post_link", Type: "text", Optional: true},
				{DisplayName: "Page Link", Name: "page_link", Type: "text", Optional: true},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open partner content dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: "Failed to open form."})
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

	err := h.svc.SubmitContent(
		r.Context(),
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

	requestID, _ := req.Context["request_id"].(string)

	err := h.svc.ConfirmTLQC(r.Context(), requestID, req.UserID)
	if err != nil {
		writeJSON(w, ActionResponse{EphemeralText: err.Error()})
		return
	}
	writeJSON(w, ActionResponse{EphemeralText: "TLQC confirmed. Waiting for payment info..."})
}

// HandleTLQCReturnForm opens the return reason dialog for TLQC.
func (h *BudgetHandler) HandleTLQCReturnForm(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/tlqc-return",
		Dialog: mattermost.Dialog{
			Title:       "Return to Partner",
			CallbackID:  requestID,
			SubmitLabel: "Return",
			Elements: []mattermost.DialogElement{
				{DisplayName: "Reason", Name: "reason", Type: "textarea"},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open tlqc return dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: "Failed to open form."})
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

	err := h.svc.ReturnToPartner(r.Context(), sub.CallbackID, sub.UserID, sub.Submission["reason"])
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

	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/partner-payment",
		Dialog: mattermost.Dialog{
			Title:       "Budget - Payment Info",
			CallbackID:  requestID,
			SubmitLabel: "Submit",
			Elements: []mattermost.DialogElement{
				{DisplayName: "Recipient Name", Name: "recipient_name", Type: "text"},
				{DisplayName: "Bank Account", Name: "bank_account", Type: "text"},
				{DisplayName: "Bank Name", Name: "bank_name", Type: "text"},
				{DisplayName: "Payment Amount", Name: "payment_amount", Type: "text", Placeholder: "e.g. 30$ or 30VND"},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open partner payment dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: "Failed to open form."})
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

	err := h.svc.SubmitPayment(
		r.Context(),
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

	requestID, _ := req.Context["request_id"].(string)

	err := h.svc.Approve(r.Context(), requestID, req.UserID)
	if err != nil {
		writeJSON(w, ActionResponse{EphemeralText: err.Error()})
		return
	}
	writeJSON(w, ActionResponse{EphemeralText: "Approved. Waiting for finance..."})
}

// HandleFinanceCompleteForm opens the completion dialog for finance.
func (h *BudgetHandler) HandleFinanceCompleteForm(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/finance-complete",
		Dialog: mattermost.Dialog{
			Title:       "Budget - Complete",
			CallbackID:  requestID,
			SubmitLabel: "Complete",
			Elements: []mattermost.DialogElement{
				{DisplayName: "Transaction Code", Name: "transaction_code", Type: "text"},
				{DisplayName: "Bill URL", Name: "bill_url", Type: "text", Optional: true, Placeholder: "URL or reference"},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open finance complete dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: "Failed to open form."})
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

	err := h.svc.Complete(
		r.Context(),
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

	requestID, _ := req.Context["request_id"].(string)

	err := h.svc.RejectRequest(r.Context(), requestID, req.UserID)
	if err != nil {
		writeJSON(w, ActionResponse{EphemeralText: err.Error()})
		return
	}
	writeJSON(w, ActionResponse{EphemeralText: "Request rejected."})
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
