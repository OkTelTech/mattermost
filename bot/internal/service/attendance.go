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

// CheckInResult holds the result of a check-in operation.
type CheckInResult struct {
	Message string
	PostID  string
}

func (s *AttendanceService) CheckIn(ctx context.Context, userID, username, channelID, fileID string) (*CheckInResult, error) {
	now := time.Now()
	date := now.Format(time.DateOnly)

	record, err := s.store.GetTodayRecord(ctx, userID, date)
	if err != nil {
		return nil, fmt.Errorf("get today record: %w", err)
	}
	if record != nil {
		return nil, fmt.Errorf(i18n.T(ctx, "attendance.msg.already_checked_in", map[string]any{
			"Username": username, "Time": record.CheckIn.Format(time.TimeOnly),
		}))
	}

	// Get channel info to retrieve TeamID
	channelInfo, err := s.mm.GetChannel(channelID)
	if err != nil {
		return nil, fmt.Errorf("get channel info: %w", err)
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
		return nil, fmt.Errorf("create record: %w", err)
	}

	msg := "@" + username
	msgData := map[string]any{
		"Username": username,
	}
	if fileID != "" {
		msgData["FileID"] = fileID
		record.CheckInImageID = fileID
	}
	postReq := &mattermost.Post{
		ChannelID: channelID,
		Message:   msg,
		Props: mattermost.Props{
			MessageKey:  "attendance.msg.checked_in",
			MessageData: msgData,
		},
	}

	post, err := s.mm.CreatePost(postReq)
	if err != nil {
		return nil, fmt.Errorf("create post: %w", err)
	}

	record.PostID = post.ID
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return nil, fmt.Errorf("update record: %w", err)
	}

	return &CheckInResult{Message: fmt.Sprintf("%s checked in at %s", username, now.Format(time.TimeOnly)), PostID: post.ID}, nil
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

	s.mm.CreatePost(&mattermost.Post{
		ChannelID: record.ChannelID,
		RootID:    record.PostID,
		Message:   "@" + username,
		Props: mattermost.Props{
			MessageKey: "attendance.msg.break_start",
			MessageData: map[string]any{
				"Username": username,
				"Reason":   reason,
			},
		},
	})
	return fmt.Sprintf("%s started break at %s", username, now.Format(time.TimeOnly)), nil
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
	breakDuration := now.Sub(last.Start)
	record.Status = model.AttendanceStatusWorking
	if err := s.store.UpdateRecord(ctx, record); err != nil {
		return "", err
	}

	displayReason := i18n.T(ctx, "attendance.break_reason."+last.Reason)
	fallbackData := map[string]any{
		"Username": username,
		"Reason":   displayReason,
		"Duration": formatDuration(ctx, breakDuration),
	}
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: record.ChannelID,
		RootID:    record.PostID,
		Message:   i18n.T(ctx, "attendance.msg.break_end", fallbackData),
		Props: mattermost.Props{
			MessageKey: "attendance.msg.break_end",
			MessageData: map[string]any{
				"Username": username,
				"Reason":   last.Reason,
				"Duration": int(breakDuration.Round(time.Second).Seconds()),
			},
		},
	})
	return fmt.Sprintf("%s ended break at %s", username, now.Format(time.TimeOnly)), nil
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
	if record.Status == model.AttendanceStatusBreak {
		return "", fmt.Errorf(i18n.T(ctx, "attendance.err.must_end_break", map[string]any{"Username": username}))
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

	// Calculate total break time and build break details
	var totalBreak time.Duration
	var breakLines []string
	var breaksData []map[string]any
	for idx, b := range record.Breaks {
		end := now
		if b.End != nil {
			end = *b.End
		}
		dur := end.Sub(b.Start)
		totalBreak += dur
		displayReason := i18n.T(ctx, "attendance.break_reason."+b.Reason)
		breakLines = append(breakLines, fmt.Sprintf("%d. %s — %s",
			idx+1, displayReason, formatDuration(ctx, dur),
		))
		breaksData = append(breaksData, map[string]any{
			"Reason":   b.Reason,
			"Duration": int(dur.Round(time.Second).Seconds()),
		})
	}

	totalTime := now.Sub(*record.CheckIn)
	actualWork := totalTime - totalBreak

	breakList := ""
	if len(breakLines) > 0 {
		breakList = strings.Join(breakLines, "\n") + "\n"
	}

	// Fallback Message with pre-translated strings
	fallbackData := map[string]any{
		"Username":       username,
		"TotalTime":      formatDuration(ctx, totalTime),
		"ActualWorkTime": formatDuration(ctx, actualWork),
		"TotalBreakTime": formatDuration(ctx, totalBreak),
		"BreakCount":     len(record.Breaks),
		"BreakList":      breakList,
	}
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: record.ChannelID,
		RootID:    record.PostID,
		Message:   i18n.T(ctx, "attendance.msg.checked_out", fallbackData),
		Props: mattermost.Props{
			MessageKey: "attendance.msg.checked_out",
			MessageData: map[string]any{
				"Username":       username,
				"TotalTime":      int(totalTime.Round(time.Second).Seconds()),
				"ActualWorkTime": int(actualWork.Round(time.Second).Seconds()),
				"TotalBreakTime": int(totalBreak.Round(time.Second).Seconds()),
				"BreakCount":     len(record.Breaks),
				"Breaks":         breaksData,
			},
		},
	})
	return fmt.Sprintf("%s checked out at %s", username, now.Format(time.TimeOnly)), nil
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

	// Check for overlapping leave requests (pending or approved) on the same dates
	existing, err := s.store.FindLeaveRequestsByUserAndDates(ctx, userID, dates)
	if err != nil {
		return fmt.Errorf("check existing leaves: %w", err)
	}
	for _, e := range existing {
		if e.Type == leaveType && (e.Status == model.LeaveStatusPending || e.Status == model.LeaveStatusApproved || e.Status == model.LeaveStatusPendingChange) {
			checkDates := e.Dates
			if e.Status == model.LeaveStatusPendingChange && e.NewDate != "" {
				checkDates = append(append([]string{}, checkDates...), e.NewDate)
			}
			overlap := findOverlap(dates, checkDates)
			if len(overlap) > 0 {
				displayOverlap := make([]string, len(overlap))
				for i, d := range overlap {
					displayOverlap[i] = model.FormatDateDisplay(d)
				}
				return fmt.Errorf(i18n.T(ctx, "attendance.err.duplicate_leave", map[string]any{"Dates": strings.Join(displayOverlap, ", ")}))
			}
		}
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

	msgKey := leaveMessageKey(leaveType)
	msgData := leaveMessageData(username, leaveType, dates, reason, timeStr, string(model.LeaveStatusPending))

	// Post info message to main channel (no buttons)
	infoPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: channelID,
		Message:   "@" + username,
		Props: mattermost.Props{
			MessageKey:  msgKey,
			MessageData: msgData,
		},
	})
	if err != nil {
		return fmt.Errorf("post info message: %w", err)
	}

	// Post approval message to approval channel (with buttons)
	msgData["Mention"] = "@all"
	approvalPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: approvalChannelID,
		Message:   "@all",
		Props: mattermost.Props{
			MessageKey:  msgKey,
			MessageData: msgData,
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

func (s *AttendanceService) ApproveLeave(ctx context.Context, requestID, approverID, approverUsername string) (*LeaveUpdateResult, error) {
	id, err := bson.ObjectIDFromHex(requestID)
	if err != nil {
		return nil, fmt.Errorf("invalid request ID: %w", err)
	}

	req, err := s.store.GetLeaveRequestByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return nil, fmt.Errorf(i18n.T(ctx, "attendance.err.not_found"))
	}
	if req.Status != model.LeaveStatusPending {
		return nil, fmt.Errorf(i18n.T(ctx, "attendance.err.already_processed", map[string]any{"Status": string(req.Status)}))
	}
	now := time.Now()
	req.Status = model.LeaveStatusApproved
	req.ApproverID = approverID
	req.ApproverUsername = approverUsername
	req.ApprovedAt = &now

	if err := s.store.UpdateLeaveRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("update leave request: %w", err)
	}

	msgKey := leaveMessageKey(req.Type)
	msgData := leaveMessageData(req.Username, req.Type, req.Dates, req.Reason, req.ExpectedTime, string(req.Status))

	// Update info post in main channel (status only)
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey:  msgKey,
			MessageData: msgData,
		},
	})

	// Reply in thread to notify requester
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ChannelID,
		RootID:    req.PostID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey: "attendance.msg.approved",
			MessageData: map[string]any{
				"Username": req.Username,
				"Approver": approverUsername,
			},
		},
	})

	return &LeaveUpdateResult{MessageKey: msgKey, MessageData: msgData}, nil
}

func (s *AttendanceService) RejectLeave(ctx context.Context, requestID, rejecterID, rejecterUsername, reason string) error {
	id, err := bson.ObjectIDFromHex(requestID)
	if err != nil {
		return fmt.Errorf("invalid request ID: %w", err)
	}

	req, err := s.store.GetLeaveRequestByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return fmt.Errorf(i18n.T(ctx, "attendance.err.not_found"))
	}
	if req.Status != model.LeaveStatusPending {
		return fmt.Errorf(i18n.T(ctx, "attendance.err.already_processed", map[string]any{"Status": string(req.Status)}))
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

	msgKey := leaveMessageKey(req.Type)
	msgData := leaveMessageData(req.Username, req.Type, req.Dates, req.Reason, req.ExpectedTime, string(req.Status))

	// Update info post in main channel (status only)
	s.mm.UpdatePost(req.PostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey:  msgKey,
			MessageData: msgData,
		},
	})

	// Update approval post (remove buttons, show updated status)
	s.mm.UpdatePost(req.ApprovalPostID, &mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey:  msgKey,
			MessageData: msgData,
			Attachments: []mattermost.Attachment{},
		},
	})

	// Reply in thread to notify requester
	rejectData := map[string]any{
		"Username": req.Username,
		"Approver": rejecterUsername,
		"Reason":   reason,
	}
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ChannelID,
		RootID:    req.PostID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey:  "attendance.msg.rejected",
			MessageData: rejectData,
		},
	})

	return nil
}

// GetUserFutureLeaves returns leave requests that have at least one future date.
func (s *AttendanceService) GetUserFutureLeaves(ctx context.Context, userID string) ([]model.LeaveRequest, error) {
	today := time.Now().Format(time.DateOnly)
	return s.store.FindFutureLeaveRequestsByUser(ctx, userID, today)
}

func (s *AttendanceService) RequestDateChange(ctx context.Context, requestID string, userID string, oldDate, newDate, changeReason string) error {
	id, err := bson.ObjectIDFromHex(requestID)
	if err != nil {
		return fmt.Errorf("invalid request ID: %w", err)
	}

	req, err := s.store.GetLeaveRequestByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return fmt.Errorf(i18n.T(ctx, "attendance.err.not_found"))
	}
	if req.UserID != userID {
		return fmt.Errorf(i18n.T(ctx, "attendance.err.change_not_owner"))
	}
	if req.Status != model.LeaveStatusPending && req.Status != model.LeaveStatusApproved {
		return fmt.Errorf(i18n.T(ctx, "attendance.err.change_invalid_status", map[string]any{"Status": string(req.Status)}))
	}

	// Verify oldDate is in the request's dates and is in the future
	today := time.Now().Format(time.DateOnly)
	found := false
	for _, d := range req.Dates {
		if d == oldDate {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf(i18n.T(ctx, "attendance.err.change_past_dates"))
	}
	if oldDate < today {
		return fmt.Errorf(i18n.T(ctx, "attendance.err.change_past_dates"))
	}

	// Validate new date
	if err := validateDateList(ctx, []string{newDate}); err != nil {
		return err
	}

	// Check if new date overlaps with other dates in the same request (excluding the old date)
	for _, d := range req.Dates {
		if d != oldDate && d == newDate {
			return fmt.Errorf(i18n.T(ctx, "attendance.err.duplicate_leave", map[string]any{"Dates": model.FormatDateDisplay(newDate)}))
		}
	}

	// Check overlap of new date against other leave requests (exclude self)
	existing, err := s.store.FindLeaveRequestsByUserAndDates(ctx, userID, []string{newDate})
	if err != nil {
		return fmt.Errorf("check existing leaves: %w", err)
	}
	for _, e := range existing {
		if e.ID == req.ID {
			continue
		}
		if e.Status == model.LeaveStatusPending || e.Status == model.LeaveStatusApproved || e.Status == model.LeaveStatusPendingChange {
			checkDates := e.Dates
			if e.Status == model.LeaveStatusPendingChange && e.NewDate != "" {
				checkDates = append(append([]string{}, checkDates...), e.NewDate)
			}
			overlap := findOverlap([]string{newDate}, checkDates)
			if len(overlap) > 0 {
				return fmt.Errorf(i18n.T(ctx, "attendance.err.duplicate_leave", map[string]any{"Dates": model.FormatDateDisplay(newDate)}))
			}
		}
	}

	if req.Status == model.LeaveStatusPending {
		// Pending: directly apply the change and update existing posts
		for i, d := range req.Dates {
			if d == oldDate {
				req.Dates[i] = newDate
				break
			}
		}

		if err := s.store.UpdateLeaveRequest(ctx, req); err != nil {
			return fmt.Errorf("update leave request: %w", err)
		}

		msgKey := leaveMessageKey(req.Type)
		msgData := leaveMessageData(req.Username, req.Type, req.Dates, req.Reason, req.ExpectedTime, string(req.Status))

		// Update existing info post in main channel
		s.mm.UpdatePost(req.PostID, &mattermost.Post{
			ChannelID: req.ChannelID,
			Message:   "@" + req.Username,
			Props: mattermost.Props{
				MessageKey:  msgKey,
				MessageData: msgData,
			},
		})

		// Update existing approval post (keep buttons)
		msgData["Mention"] = "@all"
		s.mm.UpdatePost(req.ApprovalPostID, &mattermost.Post{
			ChannelID: req.ApprovalChannelID,
			Message:   "@all",
			Props: mattermost.Props{
				MessageKey:  msgKey,
				MessageData: msgData,
				Attachments: []mattermost.Attachment{{
					Actions: []mattermost.Action{
						{
							Name: i18n.T(ctx, "attendance.btn.approve"),
							Type: "button",
							Integration: mattermost.Integration{
								URL:     s.botURL + "/api/attendance/approve",
								Context: map[string]any{"request_id": req.ID.Hex()},
							},
						},
						{
							Name: i18n.T(ctx, "attendance.btn.reject"),
							Type: "button",
							Integration: mattermost.Integration{
								URL:     s.botURL + "/api/attendance/reject",
								Context: map[string]any{"request_id": req.ID.Hex()},
							},
						},
					},
				}},
			},
		})

		// Thread reply on approval post to notify about the change
		s.mm.CreatePost(&mattermost.Post{
			ChannelID: req.ApprovalChannelID,
			RootID:    req.ApprovalPostID,
			Message: i18n.T(ctx, "attendance.msg.date_changed", map[string]any{
				"Username":     req.Username,
				"OldDate":      model.FormatDateDisplay(oldDate),
				"NewDate":      model.FormatDateDisplay(newDate),
				"ChangeReason": changeReason,
			}),
		})

		return nil
	}

	// Approved: create new approval posts for the date change
	req.PreviousStatus = req.Status
	req.Status = model.LeaveStatusPendingChange
	req.OldDate = oldDate
	req.NewDate = newDate
	req.ChangeReason = changeReason
	req.ApproverID = ""
	req.ApproverUsername = ""
	req.ApprovedAt = nil
	req.RejectReason = ""

	if err := s.store.UpdateLeaveRequest(ctx, req); err != nil {
		return fmt.Errorf("update leave request: %w", err)
	}

	idHex := req.ID.Hex()
	changeMsgKey := "leave.msg.change_leave"
	changeMsgData := leaveChangeMessageData(req.Username, oldDate, newDate, req.Reason, changeReason, string(req.Status))

	// Create NEW info post in main channel (no buttons)
	infoPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey:  changeMsgKey,
			MessageData: changeMsgData,
		},
	})
	if err != nil {
		return fmt.Errorf("post change info message: %w", err)
	}

	// Create NEW approval post in approval channel (with buttons)
	changeMsgData["Mention"] = "@all"
	approvalPost, err := s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   "@all",
		Props: mattermost.Props{
			MessageKey:  changeMsgKey,
			MessageData: changeMsgData,
			Attachments: []mattermost.Attachment{{
				Actions: []mattermost.Action{
					{
						Name: i18n.T(ctx, "attendance.btn.approve_change"),
						Type: "button",
						Integration: mattermost.Integration{
							URL:     s.botURL + "/api/attendance/change-approve",
							Context: map[string]any{"request_id": idHex},
						},
					},
					{
						Name: i18n.T(ctx, "attendance.btn.reject_change"),
						Type: "button",
						Integration: mattermost.Integration{
							URL:     s.botURL + "/api/attendance/change-reject",
							Context: map[string]any{"request_id": idHex},
						},
					},
				},
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("post change approval message: %w", err)
	}

	req.ChangePostID = infoPost.ID
	req.ChangeApprovalPostID = approvalPost.ID
	return s.store.UpdateLeaveRequest(ctx, req)
}

func (s *AttendanceService) ApproveDateChange(ctx context.Context, requestID, approverID, approverUsername string) (*LeaveUpdateResult, error) {
	id, err := bson.ObjectIDFromHex(requestID)
	if err != nil {
		return nil, fmt.Errorf("invalid request ID: %w", err)
	}

	req, err := s.store.GetLeaveRequestByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return nil, fmt.Errorf(i18n.T(ctx, "attendance.err.not_found"))
	}
	if req.Status != model.LeaveStatusPendingChange {
		return nil, fmt.Errorf(i18n.T(ctx, "attendance.err.not_pending_change"))
	}

	now := time.Now()
	// Save change info before clearing
	oldDate := req.OldDate
	newDate := req.NewDate
	changeReason := req.ChangeReason

	// Replace the old date with the new date in the dates array
	for i, d := range req.Dates {
		if d == req.OldDate {
			req.Dates[i] = req.NewDate
			break
		}
	}
	req.Status = model.LeaveStatusApproved
	req.ApproverID = approverID
	req.ApproverUsername = approverUsername
	req.ApprovedAt = &now
	req.OldDate = ""
	req.NewDate = ""
	req.ChangeReason = ""
	req.PreviousStatus = ""

	if err := s.store.UpdateLeaveRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("update leave request: %w", err)
	}

	// Keep the change message format, just update status
	changeMsgKey := "leave.msg.change_leave"
	changeMsgData := leaveChangeMessageData(req.Username, oldDate, newDate, req.Reason, changeReason, string(req.Status))

	// Update change info post (same format, updated status)
	s.mm.UpdatePost(req.ChangePostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey:  changeMsgKey,
			MessageData: changeMsgData,
		},
	})

	// Thread reply on change post to notify requester
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ChannelID,
		RootID:    req.ChangePostID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey: "attendance.msg.change_approved",
			MessageData: map[string]any{
				"Username": req.Username,
				"Approver": approverUsername,
			},
		},
	})

	// Return result to update the approval post (remove buttons, keep change format)
	return &LeaveUpdateResult{MessageKey: changeMsgKey, MessageData: changeMsgData}, nil
}

func (s *AttendanceService) RejectDateChange(ctx context.Context, requestID, rejecterID, rejecterUsername, reason string) error {
	id, err := bson.ObjectIDFromHex(requestID)
	if err != nil {
		return fmt.Errorf("invalid request ID: %w", err)
	}

	req, err := s.store.GetLeaveRequestByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get leave request: %w", err)
	}
	if req == nil {
		return fmt.Errorf(i18n.T(ctx, "attendance.err.not_found"))
	}
	if req.Status != model.LeaveStatusPendingChange {
		return fmt.Errorf(i18n.T(ctx, "attendance.err.not_pending_change"))
	}

	// Save change info before clearing
	oldDate := req.OldDate
	newDate := req.NewDate
	changeReason := req.ChangeReason

	// Restore previous status
	req.Status = req.PreviousStatus
	req.OldDate = ""
	req.NewDate = ""
	req.ChangeReason = ""
	req.PreviousStatus = ""

	if err := s.store.UpdateLeaveRequest(ctx, req); err != nil {
		return fmt.Errorf("update leave request: %w", err)
	}

	// Keep the change message format, just update status to rejected
	changeMsgKey := "leave.msg.change_leave"
	changeMsgData := leaveChangeMessageData(req.Username, oldDate, newDate, req.Reason, changeReason, string(model.LeaveStatusRejected))

	// Update change info post (same format, show rejected)
	s.mm.UpdatePost(req.ChangePostID, &mattermost.Post{
		ChannelID: req.ChannelID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey:  changeMsgKey,
			MessageData: changeMsgData,
		},
	})

	// Update change approval post (remove buttons, keep change format)
	s.mm.UpdatePost(req.ChangeApprovalPostID, &mattermost.Post{
		ChannelID: req.ApprovalChannelID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey:  changeMsgKey,
			MessageData: changeMsgData,
			Attachments: []mattermost.Attachment{},
		},
	})

	// Thread reply on change post to notify requester
	s.mm.CreatePost(&mattermost.Post{
		ChannelID: req.ChannelID,
		RootID:    req.ChangePostID,
		Message:   "@" + req.Username,
		Props: mattermost.Props{
			MessageKey: "attendance.msg.change_rejected",
			MessageData: map[string]any{
				"Username": req.Username,
				"Approver": rejecterUsername,
				"Reason":   reason,
			},
		},
	})

	return nil
}

func leaveChangeMessageData(username, oldDate, newDate, reason, changeReason, status string) map[string]any {
	return map[string]any{
		"Username":     username,
		"OldDate":      model.FormatDateDisplay(oldDate),
		"NewDate":      model.FormatDateDisplay(newDate),
		"Reason":       reason,
		"ChangeReason": changeReason,
		"Status":       status,
	}
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

func findOverlap(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	var out []string
	for _, v := range a {
		if _, ok := set[v]; ok {
			out = append(out, v)
		}
	}
	return out
}

// LeaveUpdateResult holds the MessageKey and MessageData for updating leave posts.
type LeaveUpdateResult struct {
	MessageKey  string
	MessageData map[string]any
}

func leaveMessageKey(leaveType model.LeaveType) string {
	switch leaveType {
	case model.LeaveTypeLateArrival:
		return "leave.msg.late"
	case model.LeaveTypeEarlyDeparture:
		return "leave.msg.early"
	default:
		return "leave.msg.leave"
	}
}

func leaveMessageData(username string, leaveType model.LeaveType, dates []string, reason, timeStr, status string) map[string]any {
	displayDates := make([]string, len(dates))
	for i, d := range dates {
		displayDates[i] = model.FormatDateDisplay(d)
	}
	return map[string]any{
		"Username":     username,
		"LeaveType":    string(leaveType),
		"Dates":        strings.Join(displayDates, ", "),
		"Reason":       reason,
		"ExpectedTime": timeStr,
		"Status":       status,
	}
}


func formatDuration(ctx context.Context, d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", h, i18n.T(ctx, "duration.h")))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", m, i18n.T(ctx, "duration.m")))
	}
	if s > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d %s", s, i18n.T(ctx, "duration.s")))
	}
	return strings.Join(parts, " ")
}
