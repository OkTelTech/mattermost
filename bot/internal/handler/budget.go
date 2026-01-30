package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

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

// HandleSlashCommand handles /budget slash command - opens step 1 dialog.
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
		URL:       h.botURL + "/api/budget/step1",
		Dialog: mattermost.Dialog{
			Title:       "Budget Request - Step 1",
			SubmitLabel: "Submit",
			Elements: []mattermost.DialogElement{
				{DisplayName: "Campaign", Name: "campaign", Type: "text"},
				{DisplayName: "Partner", Name: "partner", Type: "text"},
				{DisplayName: "Amount", Name: "amount", Type: "text", SubType: "number"},
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

// HandleStep1Submit processes budget step 1 dialog.
func (h *BudgetHandler) HandleStep1Submit(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	amount, _ := strconv.ParseFloat(sub.Submission["amount"], 64)

	err := h.svc.CreateRequest(
		r.Context(),
		sub.UserID,
		sub.ChannelID,
		sub.Submission["campaign"],
		sub.Submission["partner"],
		sub.Submission["purpose"],
		sub.Submission["deadline"],
		amount,
	)
	if err != nil {
		log.Printf("ERROR create budget request: %v", err)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleStep2Form opens step 2 dialog (Partner Content).
func (h *BudgetHandler) HandleStep2Form(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/step2",
		Dialog: mattermost.Dialog{
			Title:       "Budget Step 2 - Partner Content",
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
		log.Printf("ERROR open step2 dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: "Failed to open form."})
		return
	}
	writeJSON(w, ActionResponse{})
}

// HandleStep2Submit processes step 2 dialog.
func (h *BudgetHandler) HandleStep2Submit(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	err := h.svc.SubmitStep2(
		r.Context(),
		sub.CallbackID,
		sub.Submission["post_content"],
		sub.Submission["post_link"],
		sub.Submission["page_link"],
	)
	if err != nil {
		log.Printf("ERROR submit step2: %v", err)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleStep3 handles TLQC confirmation (button click, opens dialog for ad account).
func (h *BudgetHandler) HandleStep3(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/step3-submit",
		Dialog: mattermost.Dialog{
			Title:       "Budget Step 3 - TLQC Confirm",
			CallbackID:  requestID,
			SubmitLabel: "Confirm",
			Elements: []mattermost.DialogElement{
				{DisplayName: "Ad Account ID", Name: "ad_account_id", Type: "text"},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open step3 dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: "Failed to open form."})
		return
	}
	writeJSON(w, ActionResponse{})
}

// HandleStep3Submit processes step 3 dialog.
func (h *BudgetHandler) HandleStep3Submit(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	err := h.svc.ConfirmStep3(r.Context(), sub.CallbackID, sub.Submission["ad_account_id"], sub.UserID)
	if err != nil {
		log.Printf("ERROR confirm step3: %v", err)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleStep4Form opens step 4 dialog (Payment Info).
func (h *BudgetHandler) HandleStep4Form(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/step4",
		Dialog: mattermost.Dialog{
			Title:       "Budget Step 4 - Payment Info",
			CallbackID:  requestID,
			SubmitLabel: "Submit",
			Elements: []mattermost.DialogElement{
				{DisplayName: "Recipient Name", Name: "recipient_name", Type: "text"},
				{DisplayName: "Bank Account", Name: "bank_account", Type: "text"},
				{DisplayName: "Bank Name", Name: "bank_name", Type: "text"},
				{DisplayName: "Payment Amount", Name: "payment_amount", Type: "text", SubType: "number"},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open step4 dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: "Failed to open form."})
		return
	}
	writeJSON(w, ActionResponse{})
}

// HandleStep4Submit processes step 4 dialog.
func (h *BudgetHandler) HandleStep4Submit(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	paymentAmount, _ := strconv.ParseFloat(sub.Submission["payment_amount"], 64)

	err := h.svc.SubmitStep4(
		r.Context(),
		sub.CallbackID,
		sub.Submission["recipient_name"],
		sub.Submission["bank_account"],
		sub.Submission["bank_name"],
		paymentAmount,
	)
	if err != nil {
		log.Printf("ERROR submit step4: %v", err)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleStep5 handles Team Lead approval (button click).
func (h *BudgetHandler) HandleStep5(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	requestID, _ := req.Context["request_id"].(string)

	err := h.svc.ApproveStep5(r.Context(), requestID, req.UserID)
	if err != nil {
		writeJSON(w, ActionResponse{EphemeralText: err.Error()})
		return
	}
	writeJSON(w, ActionResponse{Update: &ActionUpdate{Message: "Step 5 approved. Waiting for Step 6..."}})
}

// HandleStep6Form opens step 6 dialog (Bank Note).
func (h *BudgetHandler) HandleStep6Form(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/step6",
		Dialog: mattermost.Dialog{
			Title:       "Budget Step 6 - Bank Note",
			CallbackID:  requestID,
			SubmitLabel: "Submit",
			Elements: []mattermost.DialogElement{
				{DisplayName: "Bank Note", Name: "bank_note", Type: "textarea"},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open step6 dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: "Failed to open form."})
		return
	}
	writeJSON(w, ActionResponse{})
}

// HandleStep6Submit processes step 6 dialog.
func (h *BudgetHandler) HandleStep6Submit(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	err := h.svc.SubmitStep6(r.Context(), sub.CallbackID, sub.Submission["bank_note"])
	if err != nil {
		log.Printf("ERROR submit step6: %v", err)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleStep7Form opens step 7 dialog (Finance Completion).
func (h *BudgetHandler) HandleStep7Form(w http.ResponseWriter, r *http.Request) {
	var req ActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	requestID, _ := req.Context["request_id"].(string)

	err := h.mm.OpenDialog(&mattermost.DialogRequest{
		TriggerID: req.TriggerID,
		URL:       h.botURL + "/api/budget/step7",
		Dialog: mattermost.Dialog{
			Title:       "Budget Step 7 - Finance Complete",
			CallbackID:  requestID,
			SubmitLabel: "Complete",
			Elements: []mattermost.DialogElement{
				{DisplayName: "Transaction Code", Name: "transaction_code", Type: "text"},
			},
		},
	})
	if err != nil {
		log.Printf("ERROR open step7 dialog: %v", err)
		writeJSON(w, ActionResponse{EphemeralText: "Failed to open form."})
		return
	}
	writeJSON(w, ActionResponse{})
}

// HandleStep7Submit processes step 7 dialog.
func (h *BudgetHandler) HandleStep7Submit(w http.ResponseWriter, r *http.Request) {
	var sub DialogSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if sub.Cancelled {
		w.WriteHeader(http.StatusOK)
		return
	}

	err := h.svc.CompleteStep7(r.Context(), sub.CallbackID, sub.Submission["transaction_code"])
	if err != nil {
		log.Printf("ERROR complete step7: %v", err)
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
	writeJSON(w, ActionResponse{Update: &ActionUpdate{Message: "Request rejected."}})
}

// RegisterRoutes registers all budget routes on the given mux.
func (h *BudgetHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/budget", h.HandleSlashCommand)
	mux.HandleFunc("POST /api/budget/step1", h.HandleStep1Submit)
	mux.HandleFunc("POST /api/budget/step2-form", h.HandleStep2Form)
	mux.HandleFunc("POST /api/budget/step2", h.HandleStep2Submit)
	mux.HandleFunc("POST /api/budget/step3", h.HandleStep3)
	mux.HandleFunc("POST /api/budget/step3-submit", h.HandleStep3Submit)
	mux.HandleFunc("POST /api/budget/step4-form", h.HandleStep4Form)
	mux.HandleFunc("POST /api/budget/step4", h.HandleStep4Submit)
	mux.HandleFunc("POST /api/budget/step5", h.HandleStep5)
	mux.HandleFunc("POST /api/budget/step6-form", h.HandleStep6Form)
	mux.HandleFunc("POST /api/budget/step6", h.HandleStep6Submit)
	mux.HandleFunc("POST /api/budget/step7-form", h.HandleStep7Form)
	mux.HandleFunc("POST /api/budget/step7", h.HandleStep7Submit)
	mux.HandleFunc("POST /api/budget/reject", h.HandleReject)
}
