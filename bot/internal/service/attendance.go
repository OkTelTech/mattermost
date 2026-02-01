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

	msg := fmt.Sprintf("@%s checked in at %s", username, now.Format("15:04"))
	post, err := s.mm.CreatePost(&mattermost.Post{ChannelID: channelID, Message: msg})
	if err != nil {
		return "", fmt.Errorf("create post: %w", err)
	}

	record = &model.AttendanceRecord{
		UserID:    userID,
		Username:  username,
		ChannelID: channelID,
		PostID:    post.ID,
		Date:      date,
		CheckIn:   &now,
		Status:    model.StatusWorking,
	}
	if err := s.store.CreateRecord(ctx, record); err != nil {
		return "", fmt.Errorf("create record: %w", err)
	}

	return msg, nil
}

func (s *AttendanceService) BreakStart(ctx context.Context, userID, username, reason string) (string, error) {
	now := time.Now()
	date := now.Format("2006-01-02")

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("@%s has not checked in today", username)
	}
	if record.Status == model.StatusBreak {
		return "", fmt.Errorf("@%s is already on break", username)
	}
	if record.Status != model.StatusWorking {
		return "", fmt.Errorf("@%s is not currently working (status: %s)", username, record.Status)
	}

	record.Breaks = append(record.Breaks, model.BreakRecord{
		Start:  now,
		Reason: reason,
	})
	record.Status = model.StatusBreak
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	msg := fmt.Sprintf("@%s started break at %s — %s", username, now.Format("15:04"), reason)
	s.mm.CreatePost(&mattermost.Post{ChannelID: record.ChannelID, RootID: record.PostID, Message: msg})
	return msg, nil
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

	// Close the last open break
	last := &record.Breaks[len(record.Breaks)-1]
	last.End = &now
	record.Status = model.StatusWorking
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	duration := now.Sub(last.Start).Round(time.Minute)
	msg := fmt.Sprintf("@%s ended break at %s (break: %s)", username, now.Format("15:04"), duration)
	s.mm.CreatePost(&mattermost.Post{ChannelID: record.ChannelID, RootID: record.PostID, Message: msg})
	return msg, nil
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
		username, now.Format("15:04"), workDuration, totalBreak.Round(time.Minute))
	s.mm.CreatePost(&mattermost.Post{ChannelID: record.ChannelID, RootID: record.PostID, Message: msg})
	return msg, nil
}

func (s *AttendanceService) CreateLeaveRequest(ctx context.Context, userID, username, channelID, leaveType, startDate, endDate, reason string) error {
	now := time.Now()
	date := now.Format("20060102")

	count, err := s.store.CountTodayLeaveRequests(ctx, date)
	if err != nil {
		return fmt.Errorf("count today requests: %w", err)
	}
	requestID := fmt.Sprintf("LR-%s%02d", date, count+1)

	// Lookup username if not provided (dialog submissions may omit it)
	if username == "" {
		user, err := s.mm.GetUser(userID)
		if err != nil {
			return fmt.Errorf("get user info: %w", err)
		}
		username = user.Username
	}

	days, err := calcDays(startDate, endDate)
	if err != nil {
		return fmt.Errorf("calc days: %w", err)
	}

	leaveTypeName := leaveTypeLabel(leaveType)

	// Resolve approval channel FIRST before creating any posts
	channelInfo, err := s.mm.GetChannel(channelID)
	if err != nil {
		return fmt.Errorf("get channel info: %w", err)
	}

	approvalChannelName := channelInfo.Name + approvalSuffix
	approvalChannelID, err := s.mm.GetChannelByName(channelInfo.TeamID, approvalChannelName)
	if err != nil {
		return fmt.Errorf("get approval channel '%s': %w", approvalChannelName, err)
	}

	// Post info message to main channel (no buttons)
	infoMsg := formatLeaveMsg(username, leaveTypeName, startDate, endDate, days, reason, "Pending", "")

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

func (s *AttendanceService) ApproveLeave(ctx context.Context, requestID, approverID, approverUsername string) (string, error) {
	req, err := s.store.GetLeaveRequest(ctx, requestID)
	if err != nil {
		return "", fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return "", fmt.Errorf("leave request %s not found", requestID)
	}
	if req.Status != model.LeaveStatusPending {
		return "", fmt.Errorf("request %s is already %s", requestID, req.Status)
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

	updatedMsg := formatLeaveMsg(req.Username, leaveTypeLabel(req.Type),
		req.StartDate, req.EndDate, req.Days, req.Reason,
		"**APPROVED**", fmt.Sprintf("| **Approved by** | @%s at %s |", approverUsername, now.Format("15:04")))

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
	req, err := s.store.GetLeaveRequest(ctx, requestID)
	if err != nil {
		return "", fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return "", fmt.Errorf("leave request %s not found", requestID)
	}
	if req.Status != model.LeaveStatusPending {
		return "", fmt.Errorf("request %s is already %s", requestID, req.Status)
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

	updatedMsg := formatLeaveMsg(req.Username, leaveTypeLabel(req.Type),
		req.StartDate, req.EndDate, req.Days, req.Reason,
		"**REJECTED**", fmt.Sprintf("| **Rejected by** | @%s at %s |", rejecterUsername, now.Format("15:04")))

	// Update info post in main channel
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   updatedMsg,
	})

	s.mm.SendDM(req.UserID, fmt.Sprintf("Your leave request (%s, %s → %s) was **REJECTED** by @%s",
		leaveTypeLabel(req.Type), req.StartDate, req.EndDate, rejecterUsername))

	return updatedMsg, nil
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

func formatLeaveMsg(username, leaveType, startDate, endDate string, days int, reason, status, extra string) string {
	msg := fmt.Sprintf("#### Leave Request\n| | |\n|:--|:--|\n| **User** | @%s |\n| **Type** | %s |\n| **Date** | %s → %s (%d days) |\n| **Reason** | %s |\n| **Status** | %s |",
		username, leaveType, startDate, endDate, days, reason, status)
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
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
