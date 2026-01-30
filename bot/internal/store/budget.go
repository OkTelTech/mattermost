package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"oktel-bot/internal/model"
)

type BudgetStore struct {
	coll *mongo.Collection
}

func NewBudgetStore(db *MongoDB) *BudgetStore {
	return &BudgetStore{coll: db.Collection("budget_requests")}
}

func (s *BudgetStore) Create(ctx context.Context, req *model.BudgetRequest) error {
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()
	_, err := s.coll.InsertOne(ctx, req)
	return err
}

func (s *BudgetStore) GetByRequestID(ctx context.Context, requestID string) (*model.BudgetRequest, error) {
	var req model.BudgetRequest
	err := s.coll.FindOne(ctx, bson.M{"request_id": requestID}).Decode(&req)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find budget request: %w", err)
	}
	return &req, nil
}

func (s *BudgetStore) Update(ctx context.Context, req *model.BudgetRequest) error {
	req.UpdatedAt = time.Now()
	_, err := s.coll.ReplaceOne(ctx, bson.M{"_id": req.ID}, req)
	return err
}

func (s *BudgetStore) CountTodayRequests(ctx context.Context, date string) (int64, error) {
	return s.coll.CountDocuments(ctx, bson.M{
		"request_id": bson.M{"$regex": fmt.Sprintf("^BR-%s", date)},
	})
}
