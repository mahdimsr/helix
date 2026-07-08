package database

import (
	"context"
	"fmt"
	"helix/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type CandleRepository struct {
	collection *mongo.Collection
}

func NewCandleRepository(db *mongo.Database) *CandleRepository {
	return &CandleRepository{
		collection: db.Collection("candles"),
	}
}

func (r *CandleRepository) CreateMany(ctx context.Context, candles []models.Candle) error {
	if len(candles) == 0 {
		return nil
	}

	var documents []interface{}
	for _, candle := range candles {
		documents = append(documents, candle)
	}

	_, err := r.collection.InsertMany(ctx, documents)
	if err != nil {
		return fmt.Errorf("failed to insert many candles: %w", err)
	}

	return nil
}

func (r *CandleRepository) Fetch(ctx context.Context, symbol string, timeframe string, startTime int64, endTime int64) ([]models.Candle, error) {
	// ساخت فیلتر جستجو
	filter := bson.M{
		"symbol":    symbol,
		"timeframe": timeframe,
		"time": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}

	findOptions := options.Find().SetSort(bson.D{{Key: "time", Value: 1}})

	// اجرای کوئری در مونگو
	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch candles: %w", err)
	}
	// بستن کرسر در پایان کار
	defer cursor.Close(ctx)

	var candles []models.Candle
	if err = cursor.All(ctx, &candles); err != nil {
		return nil, fmt.Errorf("failed to decode candles: %w", err)
	}

	return candles, nil
}
