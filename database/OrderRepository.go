package database

import (
	"context"
	"helix/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type OrderRepository struct {
	collection *mongo.Collection
}

func NewOrderRepository(db *mongo.Database) *OrderRepository {
	return &OrderRepository{collection: db.Collection("orders")}
}

func (repo *OrderRepository) Create(ctx context.Context, order *models.Order) (*mongo.InsertOneResult, error) {
	return repo.collection.InsertOne(ctx, order)
}

func (repo *OrderRepository) GetByProperty(ctx context.Context, filter bson.M) (*models.Order, error) {
	var order models.Order
	err := repo.collection.FindOne(ctx, filter).Decode(&order)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (repo *OrderRepository) Update(ctx context.Context, filter bson.M, data bson.M) (*mongo.UpdateResult, error) {

	update := bson.M{"$set": data}

	return repo.collection.UpdateOne(ctx, filter, update)
}
