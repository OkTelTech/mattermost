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
	date := now.Format(time.DateOnly)

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return "", fmt.Errorf("get today record: %w", err)
	}
	if record != nil {
		return "", fmt.Errorf("@%s already checked in today", username)
	}

	// Get channel info to retrieve TeamID
	channelInfo, err := s.mm.GetChannel(channelID)
	if err != nil {
		return "", fmt.Errorf("get channel info: %w", err)
	}

	record = &model.AttendanceRecord{
		UserID:    userID,
		Username:  username,
		TeamID:    channelInfo.TeamID,
		ChannelID: channelID,
		Date:      date,
		CheckIn:   &now,
		Status:    model.AttendanceStatusWorking,
	}
	if err := s.store.CreateRecord(ctx, record); err != nil {
		return "", fmt.Errorf("create record: %w", err)
	}

	msg := fmt.Sprintf("@%s checked in", username)
	post, err := s.mm.CreatePost(&mattermost.Post{ChannelID: channelID, Message: msg})
	if err != nil {
		return "", fmt.Errorf("create post: %w", err)
	}

	record.PostID = post.ID
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", fmt.Errorf("update record: %w", err)
	}

	return msg, nil
}

func (s *AttendanceService) BreakStart(ctx context.Context, userID, username, reason string) (string, error) {
	now := time.Now()
	date := now.Format(time.DateOnly)

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("@%s has not checked in today", username)
	}
	if record.Status == model.AttendanceStatusBreak {
		return "", fmt.Errorf("@%s is already on break", username)
	}
	if record.Status != model.AttendanceStatusWorking {
		return "", fmt.Errorf("@%s is not currently working (status: %s)", username, record.Status)
	}

	record.Breaks = append(record.Breaks, model.BreakRecord{
		Start:  now,
		Reason: reason,
	})
	record.Status = model.AttendanceStatusBreak
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	msg := fmt.Sprintf("@%s started break — %s", username, reason)
	s.mm.CreatePost(&mattermost.Post{ChannelID: record.ChannelID, RootID: record.PostID, Message: msg})
	return msg, nil
}

func (s *AttendanceService) BreakEnd(ctx context.Context, userID, username string) (string, error) {
	now := time.Now()
	date := now.Format(time.DateOnly)

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("@%s has not checked in today", username)
	}
	if record.Status != model.AttendanceStatusBreak {
		return "", fmt.Errorf("@%s is not on break", username)
	}

	// Close the last open break
	last := &record.Breaks[len(record.Breaks)-1]
	last.End = &now
	record.Status = model.AttendanceStatusWorking
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	msg := fmt.Sprintf("@%s ended break", username)
	s.mm.CreatePost(&mattermost.Post{ChannelID: record.ChannelID, RootID: record.PostID, Message: msg})
	return msg, nil
}

func (s *AttendanceService) CheckOut(ctx context.Context, userID, username string) (string, error) {
	now := time.Now()
	date := now.Format(time.DateOnly)

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("@%s has not checked in today", username)
	}
	if record.CheckOut != nil {
		return "", fmt.Errorf("@%s already checked out today", username)
	}

	record.CheckOut = &now
	record.Status = model.AttendanceStatusCompleted
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	// Calculate total break time
	var totalBreak time.Duration
	for _, b := range record.Breaks {
		end := now
		if b.End != nil {
			end = *b.End
		}
		totalBreak += end.Sub(b.Start)
	}

	msg := fmt.Sprintf("@%s checked out", username)
	s.mm.CreatePost(&mattermost.Post{ChannelID: record.ChannelID, RootID: record.PostID, Message: msg})
	return msg, nil
}

func (s *AttendanceService) CreateLeaveRequest(ctx context.Context, userID, username, channelID string, leaveType model.LeaveType, dates []string, reason, timeStr string) error {
	// Lookup username if not provided (dialog submissions may omit it)
	if username == "" {
		user, err := s.mm.GetUser(userID)
		if err != nil {
			return fmt.Errorf("get user info: %w", err)
		}
		username = user.Username
	}

	if err := validateDateList(dates); err != nil {
		return fmt.Errorf("validate dates: %w", err)
	}

	// Resolve approval channel before creating any posts
	channelInfo, err := s.mm.GetChannel(channelID)
	if err != nil {
		return fmt.Errorf("get channel info: %w", err)
	}

	// Extract suffix from channel name (e.g. "attendance-dev" → suffix "-dev")
	suffix := strings.TrimPrefix(channelInfo.Name, model.AttendanceChannel)
	approvalChannelName := model.AttendanceApprovalChannel + suffix
	approvalChannelID, err := s.mm.GetChannelByName(channelInfo.TeamID, approvalChannelName)
	if err != nil {
		return fmt.Errorf("get approval channel '%s': %w", approvalChannelName, err)
	}

	// Create DB record first to get the ID
	req := &model.LeaveRequest{
		UserID:            userID,
		Username:          username,
		TeamID:            channelInfo.TeamID,
		ChannelID:         channelID,
		ApprovalChannelID: approvalChannelID,
		Type:              leaveType,
		Dates:             dates,
		Reason:            reason,
		ExpectedTime:      timeStr,
		Status:            model.LeaveStatusPending,
	}
	if err := s.store.CreateLeaveRequest(ctx, req); err != nil {
		return fmt.Errorf("create leave request: %w", err)
	}

	idHex := req.ID.Hex()
	leaveTypeName := leaveTypeLabel(leaveType)
	infoMsg := formatLeaveMsg(username, leaveTypeName, leaveType, dates, reason, timeStr, "Pending", "")

	// Post info message to main channel (no buttons)
	infoPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: channelID,
		Message:   infoMsg,
	})
	if err != nil {
		return fmt.Errorf("post info message: %w", err)
	}

	// Post approval message to approval channel (with buttons)
	approvalPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: approvalChannelID,
		Message:   "@all\n" + infoMsg,
		Props: mattermost.Props{
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: "Approve",
						Type: "button",
						Integration: mattermost.Integration{
							URL: s.botURL + "/api/attendance/approve",
							Context: map[string]any{
								"request_id": idHex,
							},
						},
					},
					{
						Name: "Reject",
						Type: "button",
						Integration: mattermost.Integration{
							URL: s.botURL + "/api/attendance/reject",
							Context: map[string]any{
								"request_id": idHex,
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

	// Update record with post IDs
	req.PostID = infoPost.ID
	req.ApprovalPostID = approvalPost.ID
	return s.store.UpdateLeaveRequest(ctx, req)
}

func (s *AttendanceService) ApproveLeave(ctx context.Context, requestID, approverID, approverUsername string) (string, error) {
	id, err := bson.ObjectIDFromHex(requestID)
	if err != nil {
		return "", fmt.Errorf("invalid request ID: %w", err)
	}

	req, err := s.store.GetLeaveRequestByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return "", fmt.Errorf("leave request not found")
	}
	if req.Status != model.LeaveStatusPending {
		return "", fmt.Errorf("request is already %s", req.Status)
	}

	now := time.Now()
	req.Status = model.LeaveStatusApproved
	req.ApproverID = approverID
	req.ApproverUsername = approverUsername
	req.ApprovedAt = &now

	if err := s.store.UpdateLeaveRequest(ctx, req); err != nil {
		return "", fmt.Errorf("update leave request: %w", err)
	}

	updatedMsg := formatLeaveMsg(req.Username, leaveTypeLabel(req.Type), req.Type,
		req.Dates, req.Reason, req.ExpectedTime, "**APPROVED**", "")

	// Update info post in main channel (status only)
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   updatedMsg,
	})

	// Reply in thread to notify requester
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ChannelID,
		RootID:    req.PostID,
		Message:   fmt.Sprintf("@%s your leave request has been **APPROVED** by @%s", req.Username, approverUsername),
	})

	return updatedMsg, nil
}

func (s *AttendanceService) RejectLeave(ctx context.Context, requestID, rejecterID, rejecterUsername, reason string) (string, error) {
	id, err := bson.ObjectIDFromHex(requestID)
	if err != nil {
		return "", fmt.Errorf("invalid request ID: %w", err)
	}

	req, err := s.store.GetLeaveRequestByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return "", fmt.Errorf("leave request not found")
	}
	if req.Status != model.LeaveStatusPending {
		return "", fmt.Errorf("request is already %s", req.Status)
	}

	now := time.Now()
	req.Status = model.LeaveStatusRejected
	req.ApproverID = rejecterID
	req.ApproverUsername = rejecterUsername
	req.ApprovedAt = &now
	req.RejectReason = reason

	if err := s.store.UpdateLeaveRequest(ctx, req); err != nil {
		return "", fmt.Errorf("update leave request: %w", err)
	}

	updatedMsg := formatLeaveMsg(req.Username, leaveTypeLabel(req.Type), req.Type,
		req.Dates, req.Reason, req.ExpectedTime, "**REJECTED**", "")

	// Update info post in main channel (status only)
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   updatedMsg,
	})

	// Update approval post (remove buttons, show updated status)
	s.mm.UpdatePost(req.ApprovalPostID, &mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   updatedMsg,
		Props:     mattermost.Props{Attachments: []mattermost.Attachment{}},
	})

	// Reply in thread to notify requester
	replyMsg := fmt.Sprintf("@%s your leave request has been **REJECTED** by @%s", req.Username, rejecterUsername)
	if reason != "" {
		replyMsg += fmt.Sprintf("\n> **Reason:** %s", reason)
	}
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ChannelID,
		RootID:    req.PostID,
		Message:   replyMsg,
	})

	return updatedMsg, nil
}

func validateDateList(dates []string) error {
	if len(dates) == 0 {
		return fmt.Errorf("at least one date is required")
	}
	today := time.Now().Format(time.DateOnly)
	for _, d := range dates {
		if _, err := time.Parse(time.DateOnly, d); err != nil {
			return fmt.Errorf("invalid date %q: %w", d, err)
		}
		if d < today {
			return fmt.Errorf("date %s is in the past", d)
		}
	}
	return nil
}

func formatLeaveMsg(username, leaveTypeName string, leaveTypeRaw model.LeaveType, dates []string, reason, timeStr, status, extra string) string {
	var msg string
	switch leaveTypeRaw {
	case model.LeaveTypeLateArrival:
		msg = fmt.Sprintf("#### Late Arrival Request\n| | |\n|:--|:--|\n| **User** | @%s |\n| **Date** | %s |\n| **Expected Arrival** | %s |\n| **Reason** | %s |\n| **Status** | %s |",
			username, dates[0], timeStr, reason, status)
	case model.LeaveTypeEarlyDeparture:
		msg = fmt.Sprintf("#### Early Departure Request\n| | |\n|:--|:--|\n| **User** | @%s |\n| **Date** | %s |\n| **Expected Departure** | %s |\n| **Reason** | %s |\n| **Status** | %s |",
			username, dates[0], timeStr, reason, status)
	default:
		msg = fmt.Sprintf("#### Leave Request\n| | |\n|:--|:--|\n| **User** | @%s |\n| **Type** | %s |\n| **Dates** | %s |\n| **Reason** | %s |\n| **Status** | %s |",
			username, leaveTypeName, strings.Join(dates, ", "), reason, status)
	}
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func leaveTypeLabel(t model.LeaveType) string {
	switch t {
	case model.LeaveTypeAnnual:
		return "Annual Leave"
	case model.LeaveTypeEmergency:
		return "Emergency Leave"
	case model.LeaveTypeSick:
		return "Sick Leave"
	case model.LeaveTypeLateArrival:
		return "Late Arrival"
	case model.LeaveTypeEarlyDeparture:
		return "Early Departure"
	default:
		return string(t)
	}
}
