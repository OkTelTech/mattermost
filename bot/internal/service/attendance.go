package service

import (
	"context"
	"fmt"
	"time"

	"oktel-bot/internal/mattermost"
	"oktel-bot/internal/model"
	"oktel-bot/internal/store"
)

const approvalSuffix = "-approval"

type AttendanceService struct {
	store  *store.AttendanceStore
	mm     *mattermost.Client
	botURL string // Bot service base URL for integration callbacks
}

func NewAttendanceService(store *store.AttendanceStore, mm *mattermost.Client, botURL string) *AttendanceService {
	return &AttendanceService{store: store, mm: mm, botURL: botURL}
}

func (s *AttendanceService) CheckIn(ctx context.Context, userID, username, channelID string) (string, error) {
	now := time.Now()
	date := now.Format("2006-01-02")

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return "", fmt.Errorf("get today record: %w", err)
	}
	if record != nil {
		return "", fmt.Errorf("@%s already checked in today at %s", username, record.CheckIn.Format("15:04"))
	}

	record = &model.AttendanceRecord{
		UserID:    userID,
		Username:  username,
		ChannelID: channelID,
		Date:      date,
		CheckIn:   &now,
		Status:    model.StatusWorking,
	}
	if err := s.store.CreateRecord(ctx, record); err != nil {
		return "", fmt.Errorf("create record: %w", err)
	}

	return fmt.Sprintf("@%s checked in at %s", username, now.Format("15:04")), nil
}

func (s *AttendanceService) BreakStart(ctx context.Context, userID, username string) (string, error) {
	now := time.Now()
	date := now.Format("2006-01-02")

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("@%s has not checked in today", username)
	}
	if record.Status != model.StatusWorking {
		return "", fmt.Errorf("@%s is not currently working (status: %s)", username, record.Status)
	}
	if record.BreakStart != nil {
		return "", fmt.Errorf("@%s already started break at %s", username, record.BreakStart.Format("15:04"))
	}

	record.BreakStart = &now
	record.Status = model.StatusBreak
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	return fmt.Sprintf("@%s started break at %s", username, now.Format("15:04")), nil
}

func (s *AttendanceService) BreakEnd(ctx context.Context, userID, username string) (string, error) {
	now := time.Now()
	date := now.Format("2006-01-02")

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("@%s has not checked in today", username)
	}
	if record.Status != model.StatusBreak {
		return "", fmt.Errorf("@%s is not on break", username)
	}

	record.BreakEnd = &now
	record.Status = model.StatusWorking
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	duration := now.Sub(*record.BreakStart).Round(time.Minute)
	return fmt.Sprintf("@%s ended break at %s (break: %s)", username, now.Format("15:04"), duration), nil
}

func (s *AttendanceService) CheckOut(ctx context.Context, userID, username string) (string, error) {
	now := time.Now()
	date := now.Format("2006-01-02")

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("@%s has not checked in today", username)
	}
	if record.CheckOut != nil {
		return "", fmt.Errorf("@%s already checked out at %s", username, record.CheckOut.Format("15:04"))
	}

	record.CheckOut = &now
	record.Status = model.StatusCompleted
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	workDuration := now.Sub(*record.CheckIn).Round(time.Minute)
	return fmt.Sprintf("@%s checked out at %s (total: %s)", username, now.Format("15:04"), workDuration), nil
}

func (s *AttendanceService) CreateLeaveRequest(ctx context.Context, userID, username, channelID, leaveType, startDate, endDate, reason string) error {
	now := time.Now()
	date := now.Format("20060102")

	count, err := s.store.CountTodayLeaveRequests(ctx, date)
	if err != nil {
		return fmt.Errorf("count today requests: %w", err)
	}
	requestID := fmt.Sprintf("LR-%s%02d", date, count+1)

	days, err := calcDays(startDate, endDate)
	if err != nil {
		return fmt.Errorf("calc days: %w", err)
	}

	leaveTypeName := leaveTypeLabel(leaveType)

	// Post info message to main channel (no buttons)
	infoMsg := fmt.Sprintf("**LEAVE REQUEST #%s**\n| | |\n|:--|:--|\n| User | @%s |\n| Type | %s |\n| Date | %s → %s (%d days) |\n| Reason | %s |\n| Status | PENDING |",
		requestID, username, leaveTypeName, startDate, endDate, days, reason)

	infoPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: channelID,
		Message:   infoMsg,
	})
	if err != nil {
		return fmt.Errorf("post info message: %w", err)
	}

	// Post approval message to approval channel (with buttons)
	channelInfo, err := s.mm.GetChannel(channelID)
	if err != nil {
		return fmt.Errorf("get channel info: %w", err)
	}

	approvalChannelName := channelInfo.Name + approvalSuffix
	approvalChannelID, err := s.mm.GetChannelByName(channelInfo.TeamID, approvalChannelName)
	if err != nil {
		return fmt.Errorf("get approval channel '%s': %w", approvalChannelName, err)
	}

	approvalPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: approvalChannelID,
		Message:   infoMsg,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: "Approve",
						Type: "button",
						Integration: mattermost.Integration{
							URL: s.botURL + "/api/attendance/approve",
							Context: map[string]any{
								"request_id": requestID,
							},
						},
					},
					{
						Name: "Reject",
						Type: "button",
						Integration: mattermost.Integration{
							URL: s.botURL + "/api/attendance/reject",
							Context: map[string]any{
								"request_id": requestID,
							},
						},
					},
				},
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("post approval message: %w", err)
	}

	req := &model.LeaveRequest{
		RequestID:         requestID,
		UserID:            userID,
		Username:          username,
		ChannelID:         channelID,
		ApprovalChannelID: approvalChannelID,
		PostID:            infoPost.ID,
		ApprovalPostID:    approvalPost.ID,
		Type:              leaveType,
		StartDate:         startDate,
		EndDate:           endDate,
		Days:              days,
		Reason:            reason,
		Status:            model.LeaveStatusPending,
	}
	return s.store.CreateLeaveRequest(ctx, req)
}

func (s *AttendanceService) ApproveLeave(ctx context.Context, requestID, approverID, approverUsername string) error {
	req, err := s.store.GetLeaveRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return fmt.Errorf("leave request %s not found", requestID)
	}
	if req.Status != model.LeaveStatusPending {
		return fmt.Errorf("request %s is already %s", requestID, req.Status)
	}
	if req.UserID == approverID {
		return fmt.Errorf("cannot approve your own request")
	}

	now := time.Now()
	req.Status = model.LeaveStatusApproved
	req.ApproverID = approverID
	req.ApproverUsername = approverUsername
	req.ApprovedAt = &now

	if err := s.store.UpdateLeaveRequest(ctx, req); err != nil {
		return fmt.Errorf("update leave request: %w", err)
	}

	// Update approval post (remove buttons)
	updatedMsg := fmt.Sprintf("**LEAVE REQUEST #%s**\n| | |\n|:--|:--|\n| User | @%s |\n| Type | %s |\n| Date | %s → %s (%d days) |\n| Reason | %s |\n| Status | **APPROVED** |\n| Approved by | @%s at %s |",
		req.RequestID, req.Username, leaveTypeLabel(req.Type),
		req.StartDate, req.EndDate, req.Days, req.Reason,
		approverUsername, now.Format("15:04"))

	s.mm.UpdatePost(req.ApprovalPostID, &mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   updatedMsg,
	})

	// Update info post in main channel
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   updatedMsg,
	})

	// Notify requester via DM
	s.mm.SendDM(req.UserID, fmt.Sprintf("Your leave request #%s was **APPROVED** by @%s", req.RequestID, approverUsername))

	return nil
}

func (s *AttendanceService) RejectLeave(ctx context.Context, requestID, rejecterID, rejecterUsername, reason string) error {
	req, err := s.store.GetLeaveRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return fmt.Errorf("leave request %s not found", requestID)
	}
	if req.Status != model.LeaveStatusPending {
		return fmt.Errorf("request %s is already %s", requestID, req.Status)
	}
	if req.UserID == rejecterID {
		return fmt.Errorf("cannot reject your own request")
	}

	now := time.Now()
	req.Status = model.LeaveStatusRejected
	req.ApproverID = rejecterID
	req.ApproverUsername = rejecterUsername
	req.ApprovedAt = &now
	req.RejectReason = reason

	if err := s.store.UpdateLeaveRequest(ctx, req); err != nil {
		return fmt.Errorf("update leave request: %w", err)
	}

	updatedMsg := fmt.Sprintf("**LEAVE REQUEST #%s**\n| | |\n|:--|:--|\n| User | @%s |\n| Type | %s |\n| Date | %s → %s (%d days) |\n| Reason | %s |\n| Status | **REJECTED** |\n| Rejected by | @%s at %s |",
		req.RequestID, req.Username, leaveTypeLabel(req.Type),
		req.StartDate, req.EndDate, req.Days, req.Reason,
		rejecterUsername, now.Format("15:04"))

	s.mm.UpdatePost(req.ApprovalPostID, &mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   updatedMsg,
	})

	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   updatedMsg,
	})

	s.mm.SendDM(req.UserID, fmt.Sprintf("Your leave request #%s was **REJECTED** by @%s", req.RequestID, rejecterUsername))

	return nil
}

func calcDays(start, end string) (int, error) {
	s, err := time.Parse("2006-01-02", start)
	if err != nil {
		return 0, err
	}
	e, err := time.Parse("2006-01-02", end)
	if err != nil {
		return 0, err
	}
	days := int(e.Sub(s).Hours()/24) + 1
	if days < 1 {
		return 0, fmt.Errorf("end date must be after start date")
	}
	return days, nil
}

func leaveTypeLabel(t string) string {
	switch t {
	case model.LeaveTypeAnnual:
		return "Annual Leave"
	case model.LeaveTypeEmergency:
		return "Emergency Leave"
	case model.LeaveTypeSick:
		return "Sick Leave"
	default:
		return t
	}
}
