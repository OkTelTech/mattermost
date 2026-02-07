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

func NewBudgetStore(ctx context.Context, db *MongoDB) (*BudgetStore, error) {
	budget := db.Collection("budget_requests")

	if _, err := budget.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "created_at", Value: 1}}},
		{Keys: bson.D{{Key: "team_id", Value: 1}, {Key: "created_at", Value: 1}}},
	}); err != nil {
		return nil, fmt.Errorf("create budget indexes: %w", err)
	}

	return &BudgetStore{coll: budget}, nil
}

// Create inserts a new budget request and sets the ID on the struct.
func (s *BudgetStore) Create(ctx context.Context, req *model.BudgetRequest) error {
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()
	res, err := s.coll.InsertOne(ctx, req)
	if err != nil {
		return err
	}
	req.ID = res.InsertedID.(bson.ObjectID)
	return nil
}

func (s *BudgetStore) GetByID(ctx context.Context, id bson.ObjectID) (*model.BudgetRequest, error) {
	var req model.BudgetRequest
	err := s.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&req)
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
