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
	date := now.Format(time.DateOnly)

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return "", fmt.Errorf("get today record: %w", err)
	}
	if record != nil {
		return "", fmt.Errorf("@%s already checked in today at %s", username, record.CheckIn.Format(time.TimeOnly))
	}

	record = &model.AttendanceRecord{
		UserID:    userID,
		Username:  username,
		ChannelID: channelID,
		Date:      date,
		CheckIn:   &now,
		Status:    model.AttendanceStatusWorking,
	}
	if err := s.store.CreateRecord(ctx, record); err != nil {
		return "", fmt.Errorf("create record: %w", err)
	}

	msg := fmt.Sprintf("@%s checked in at %s", username, now.Format(time.TimeOnly))
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

	msg := fmt.Sprintf("@%s started break at %s — %s", username, now.Format(time.TimeOnly), reason)
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

	duration := now.Sub(last.Start).Round(time.Minute)
	msg := fmt.Sprintf("@%s ended break at %s (break: %s)", username, now.Format(time.TimeOnly), duration)
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
		return "", fmt.Errorf("@%s already checked out at %s", username, record.CheckOut.Format(time.TimeOnly))
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

	workDuration := now.Sub(*record.CheckIn).Round(time.Minute)
	msg := fmt.Sprintf("@%s checked out at %s (total: %s, breaks: %s)",
		username, now.Format(time.TimeOnly), workDuration, totalBreak.Round(time.Minute))
	s.mm.CreatePost(&mattermost.Post{ChannelID: record.ChannelID, RootID: record.PostID, Message: msg})
	return msg, nil
}

func (s *AttendanceService) CreateLeaveRequest(ctx context.Context, userID, username, channelID string, leaveType model.LeaveType, startDate, endDate, reason, timeStr string) error {
	// Lookup username if not provided (dialog submissions may omit it)
	if username == "" {
		user, err := s.mm.GetUser(userID)
		if err != nil {
			return fmt.Errorf("get user info: %w", err)
		}
		username = user.Username
	}

	// For late arrival / early departure: single date, no range needed
	if leaveType == model.LeaveTypeLateArrival || leaveType == model.LeaveTypeEarlyDeparture {
		endDate = startDate
	}

	if err := validateDates(startDate, endDate); err != nil {
		return fmt.Errorf("validate dates: %w", err)
	}

	// Resolve approval channel before creating any posts
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
	req := &model.LeaveRequest{
		UserID:            userID,
		Username:          username,
		ChannelID:         channelID,
		ApprovalChannelID: approvalChannelID,
		Type:              leaveType,
		StartDate:         startDate,
		EndDate:           endDate,
		Reason:            reason,
		ExpectedTime:      timeStr,
		Status:            model.LeaveStatusPending,
	}
	if err := s.store.CreateLeaveRequest(ctx, req); err != nil {
		return fmt.Errorf("create leave request: %w", err)
	}

	idHex := req.ID.Hex()
	leaveTypeName := leaveTypeLabel(leaveType)
	infoMsg := formatLeaveMsg(username, leaveTypeName, leaveType, startDate, endDate, reason, timeStr, "Pending", "")

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
	if req.UserID == approverID {
		return "", fmt.Errorf("cannot approve your own request")
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
		req.StartDate, req.EndDate, req.Reason, req.ExpectedTime,
		"**APPROVED**", fmt.Sprintf("| **Approved by** | @%s at %s |", approverUsername, now.Format(time.TimeOnly)))

	// Update info post in main channel
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   updatedMsg,
	})

	// Notify requester via DM
	s.mm.SendDM(req.UserID, fmt.Sprintf("Your leave request (%s, %s → %s) was **APPROVED** by @%s",
		leaveTypeLabel(req.Type), req.StartDate, req.EndDate, approverUsername))

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
	if req.UserID == rejecterID {
		return "", fmt.Errorf("cannot reject your own request")
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

	extra := fmt.Sprintf("| **Rejected by** | @%s at %s |", rejecterUsername, now.Format(time.TimeOnly))
	if reason != "" {
		extra += fmt.Sprintf("\n| **Reject reason** | %s |", reason)
	}

	updatedMsg := formatLeaveMsg(req.Username, leaveTypeLabel(req.Type), req.Type,
		req.StartDate, req.EndDate, req.Reason, req.ExpectedTime,
		"**REJECTED**", extra)

	// Update info post in main channel
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

	dmMsg := fmt.Sprintf("Your leave request (%s, %s → %s) was **REJECTED** by @%s",
		leaveTypeLabel(req.Type), req.StartDate, req.EndDate, rejecterUsername)
	if reason != "" {
		dmMsg += fmt.Sprintf("\n> **Reason:** %s", reason)
	}
	s.mm.SendDM(req.UserID, dmMsg)

	return updatedMsg, nil
}

func validateDates(start, end string) error {
	s, err := time.Parse("2006-01-02", start)
	if err != nil {
		return err
	}
	e, err := time.Parse("2006-01-02", end)
	if err != nil {
		return err
	}
	if e.Before(s) {
		return fmt.Errorf("end date must not be before start date")
	}
	return nil
}

func formatLeaveMsg(username, leaveTypeName string, leaveTypeRaw model.LeaveType, startDate, endDate, reason, timeStr, status, extra string) string {
	var msg string
	switch leaveTypeRaw {
	case model.LeaveTypeLateArrival:
		msg = fmt.Sprintf("#### Late Arrival Request\n| | |\n|:--|:--|\n| **User** | @%s |\n| **Date** | %s |\n| **Expected Arrival** | %s |\n| **Reason** | %s |\n| **Status** | %s |",
			username, startDate, timeStr, reason, status)
	case model.LeaveTypeEarlyDeparture:
		msg = fmt.Sprintf("#### Early Departure Request\n| | |\n|:--|:--|\n| **User** | @%s |\n| **Date** | %s |\n| **Expected Departure** | %s |\n| **Reason** | %s |\n| **Status** | %s |",
			username, startDate, timeStr, reason, status)
	default:
		msg = fmt.Sprintf("#### Leave Request\n| | |\n|:--|:--|\n| **User** | @%s |\n| **Type** | %s |\n| **Date** | %s → %s |\n| **Reason** | %s |\n| **Status** | %s |",
			username, leaveTypeName, startDate, endDate, reason, status)
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
