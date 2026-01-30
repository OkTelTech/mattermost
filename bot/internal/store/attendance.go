package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"oktel-bot/internal/model"
)

type AttendanceStore struct {
	attendance *mongo.Collection
	leave      *mongo.Collection
}

func NewAttendanceStore(db *MongoDB) *AttendanceStore {
	return &AttendanceStore{
		attendance: db.Collection("attendance"),
		leave:      db.Collection("leave_requests"),
	}
}

// GetTodayRecord returns today's attendance record for a user, or nil if not found.
func (s *AttendanceStore) GetTodayRecord(ctx context.Context, userID, date string) (*model.AttendanceRecord, error) {
	var record model.AttendanceRecord
	err := s.attendance.FindOne(ctx, bson.M{
		"user_id": userID,
		"date":    date,
	}).Decode(&record)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find attendance: %w", err)
	}
	return &record, nil
}

// CreateRecord inserts a new attendance record.
func (s *AttendanceStore) CreateRecord(ctx context.Context, record *model.AttendanceRecord) error {
	record.CreatedAt = time.Now()
	record.UpdatedAt = time.Now()
	_, err := s.attendance.InsertOne(ctx, record)
	return err
}

// UpdateRecord updates an existing attendance record.
func (s *AttendanceStore) UpdateRecord(ctx context.Context, record *model.AttendanceRecord) error {
	record.UpdatedAt = time.Now()
	_, err := s.attendance.ReplaceOne(ctx, bson.M{"_id": record.ID}, record)
	return err
}

// CreateLeaveRequest inserts a new leave request.
func (s *AttendanceStore) CreateLeaveRequest(ctx context.Context, req *model.LeaveRequest) error {
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()
	_, err := s.leave.InsertOne(ctx, req)
	return err
}

// GetLeaveRequest retrieves a leave request by its request ID.
func (s *AttendanceStore) GetLeaveRequest(ctx context.Context, requestID string) (*model.LeaveRequest, error) {
	var req model.LeaveRequest
	err := s.leave.FindOne(ctx, bson.M{"request_id": requestID}).Decode(&req)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find leave request: %w", err)
	}
	return &req, nil
}

// UpdateLeaveRequest updates an existing leave request.
func (s *AttendanceStore) UpdateLeaveRequest(ctx context.Context, req *model.LeaveRequest) error {
	req.UpdatedAt = time.Now()
	_, err := s.leave.ReplaceOne(ctx, bson.M{"_id": req.ID}, req)
	return err
}

// CountTodayLeaveRequests returns the number of leave requests created today (for generating request ID).
func (s *AttendanceStore) CountTodayLeaveRequests(ctx context.Context, date string) (int64, error) {
	count, err := s.leave.CountDocuments(ctx, bson.M{
		"request_id": bson.M{"$regex": fmt.Sprintf("^LR-%s", date)},
	})
	return count, err
}
