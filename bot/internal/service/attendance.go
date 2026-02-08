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
			"Username": username, "Time": record.CheckIn.Format(time.TimeOnly),
		}))
		return "", fmt.Errorf(i18n.T(ctx, "attendance.msg.already_checked_in", map[string]any{
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

	msg := i18n.T(ctx, "attendance.msg.checked_in", map[string]any{
		"Username": username, "Time": now.Format(time.TimeOnly),
	})
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
		return "", fmt.Errorf(i18n.T(ctx, "attendance.msg.not_checked_in", map[string]any{"Username": username}))
	}
	if record.Status == model.AttendanceStatusBreak {
		return "", fmt.Errorf(i18n.T(ctx, "attendance.msg.already_on_break", map[string]any{"Username": username}))
	}
	if record.Status != model.AttendanceStatusWorking {
		return "", fmt.Errorf(i18n.T(ctx, "attendance.msg.not_working", map[string]any{
			"Username": username, "Status": string(record.Status),
		}))
	}

	record.Breaks = append(record.Breaks, model.BreakRecord{
		Start:  now,
		Reason: reason,
	})
	record.Status = model.AttendanceStatusBreak
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	msg := i18n.T(ctx, "attendance.msg.break_start", map[string]any{
		"Username": username, "Time": now.Format(time.TimeOnly), "Reason": reason,
	})
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
		return "", fmt.Errorf(i18n.T(ctx, "attendance.msg.not_checked_in", map[string]any{"Username": username}))
	}
	if record.Status != model.AttendanceStatusBreak {
		return "", fmt.Errorf(i18n.T(ctx, "attendance.msg.not_on_break", map[string]any{"Username": username}))
	}

	// Close the last open break
	last := &record.Breaks[len(record.Breaks)-1]
	last.End = &now
	record.Status = model.AttendanceStatusWorking
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	msg := i18n.T(ctx, "attendance.msg.break_end", map[string]any{
		"Username": username, "Time": now.Format(time.TimeOnly),
	})
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
		return "", fmt.Errorf(i18n.T(ctx, "attendance.msg.not_checked_in", map[string]any{"Username": username}))
	}
	if record.CheckOut != nil {
		return "", fmt.Errorf(i18n.T(ctx, "attendance.msg.already_checked_out", map[string]any{
			"Username": username, "Time": record.CheckOut.Format(time.TimeOnly),
		}))
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

	msg := i18n.T(ctx, "attendance.msg.checked_out", map[string]any{
		"Username": username, "Time": now.Format(time.TimeOnly),
	})
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

	if err := validateDateList(ctx, dates); err != nil {
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
	infoMsg := formatLeaveMsg(ctx, username, leaveType, dates, reason, timeStr, i18n.T(ctx, "leave.status.pending"), "")

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
						Name: i18n.T(ctx, "attendance.btn.approve"),
						Type: "button",
						Integration: mattermost.Integration{
							URL: s.botURL + "/api/attendance/approve",
							Context: map[string]any{
								"request_id": idHex,
							},
						},
					},
					{
						Name: i18n.T(ctx, "attendance.btn.reject"),
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
		return "", fmt.Errorf(i18n.T(ctx, "attendance.err.not_found"))
	}
	if req.Status != model.LeaveStatusPending {
		return "", fmt.Errorf(i18n.T(ctx, "attendance.err.already_processed", map[string]any{"Status": string(req.Status)}))
	}
	if req.UserID == approverID {
		return "", fmt.Errorf(i18n.T(ctx, "attendance.err.self_approve"))
	}

	now := time.Now()
	req.Status = model.LeaveStatusApproved
	req.ApproverID = approverID
	req.ApproverUsername = approverUsername
	req.ApprovedAt = &now

	if err := s.store.UpdateLeaveRequest(ctx, req); err != nil {
		return "", fmt.Errorf("update leave request: %w", err)
	}

	updatedMsg := formatLeaveMsg(ctx, req.Username, req.Type,
		req.Dates, req.Reason, req.ExpectedTime, i18n.T(ctx, "leave.status.approved"), "")

	// Update info post in main channel (status only)
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   updatedMsg,
	})

	// Reply in thread to notify requester
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ChannelID,
		RootID:    req.PostID,
		Message: i18n.T(ctx, "attendance.msg.approved", map[string]any{
			"Username": req.Username, "Approver": approverUsername,
		}),
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
		return "", fmt.Errorf(i18n.T(ctx, "attendance.err.not_found"))
	}
	if req.Status != model.LeaveStatusPending {
		return "", fmt.Errorf(i18n.T(ctx, "attendance.err.already_processed", map[string]any{"Status": string(req.Status)}))
	}
	if req.UserID == rejecterID {
		return "", fmt.Errorf(i18n.T(ctx, "attendance.err.self_reject"))
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

	updatedMsg := formatLeaveMsg(ctx, req.Username, req.Type,
		req.Dates, req.Reason, req.ExpectedTime, i18n.T(ctx, "leave.status.rejected"), "")

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
	replyMsg := i18n.T(ctx, "attendance.msg.rejected", map[string]any{
		"Username": req.Username, "Approver": rejecterUsername,
	})
	if reason != "" {
		replyMsg += i18n.T(ctx, "attendance.msg.reject_reason", map[string]any{"Reason": reason})
	}
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ChannelID,
		RootID:    req.PostID,
		Message:   replyMsg,
	})

	return updatedMsg, nil
}

func validateDateList(ctx context.Context, dates []string) error {
	if len(dates) == 0 {
		return fmt.Errorf(i18n.T(ctx, "attendance.err.date_required"))
	}
	today := time.Now().Format(time.DateOnly)
	for _, d := range dates {
		if _, err := time.Parse(time.DateOnly, d); err != nil {
			return fmt.Errorf(i18n.T(ctx, "attendance.err.invalid_date", map[string]any{"Date": d}))
		}
		if d < today {
			return fmt.Errorf(i18n.T(ctx, "attendance.err.past_date", map[string]any{"Date": d}))
		}
	}
	return nil
}

func formatLeaveMsg(ctx context.Context, username string, leaveTypeRaw model.LeaveType, dates []string, reason, timeStr, status, extra string) string {
	var msg string
	switch leaveTypeRaw {
	case model.LeaveTypeLateArrival:
		msg = fmt.Sprintf("%s\n| | |\n|:--|:--|\n| %s | @%s |\n| %s | %s |\n| %s | %s |\n| %s | %s |\n| %s | %s |",
			i18n.T(ctx, "leave.header.late"),
			i18n.T(ctx, "leave.field.user"), username,
			i18n.T(ctx, "leave.field.date"), dates[0],
			i18n.T(ctx, "leave.field.arrival"), timeStr,
			i18n.T(ctx, "leave.field.reason"), reason,
			i18n.T(ctx, "leave.field.status"), status)
	case model.LeaveTypeEarlyDeparture:
		msg = fmt.Sprintf("%s\n| | |\n|:--|:--|\n| %s | @%s |\n| %s | %s |\n| %s | %s |\n| %s | %s |\n| %s | %s |",
			i18n.T(ctx, "leave.header.early"),
			i18n.T(ctx, "leave.field.user"), username,
			i18n.T(ctx, "leave.field.date"), dates[0],
			i18n.T(ctx, "leave.field.departure"), timeStr,
			i18n.T(ctx, "leave.field.reason"), reason,
			i18n.T(ctx, "leave.field.status"), status)
	default:
		msg = fmt.Sprintf("%s\n| | |\n|:--|:--|\n| %s | @%s |\n| %s | %s |\n| %s | %s |\n| %s | %s |\n| %s | %s |",
			i18n.T(ctx, "leave.header.leave"),
			i18n.T(ctx, "leave.field.user"), username,
			i18n.T(ctx, "leave.field.type"), leaveTypeLabel(ctx, leaveTypeRaw),
			i18n.T(ctx, "leave.field.dates"), strings.Join(dates, ", "),
			i18n.T(ctx, "leave.field.reason"), reason,
			i18n.T(ctx, "leave.field.status"), status)
	}
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func leaveTypeLabel(ctx context.Context, t model.LeaveType) string {
	switch t {
	case model.LeaveTypeAnnual:
		return i18n.T(ctx, "leave.type.annual")
	case model.LeaveTypeEmergency:
		return i18n.T(ctx, "leave.type.emergency")
	case model.LeaveTypeSick:
		return i18n.T(ctx, "leave.type.sick")
	case model.LeaveTypeLateArrival:
		return i18n.T(ctx, "leave.type.late")
	case model.LeaveTypeEarlyDeparture:
		return i18n.T(ctx, "leave.type.early")
	default:
		return string(t)
	}
}