package database

import (
	"context"
	"fmt"
	"helix/models"
	"sort"
	"time"

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

func (r *CandleRepository) Fetch(ctx context.Context, symbol string, timeframe string, startTime string, endTime string) ([]models.Candle, error) {
	layout := "2006-01-02"

	startTimeObj, err := time.Parse(layout, startTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start date format, expected YYYY-MM-DD: %w", err)
	}

	endTimeObj, err := time.Parse(layout, endTime)
	if err != nil {
		return nil, fmt.Errorf("invalid end date format, expected YYYY-MM-DD: %w", err)
	}

	// ساخت فیلتر جستجو
	// نکته: اگر دیتابیس شما زمان را به میلی‌ثانیه ذخیره می‌کند، این بخش درست است.
	filter := bson.M{
		"symbol":    symbol,
		"timeframe": timeframe,
		"time": bson.M{
			"$gte": startTimeObj.UnixMilli(),
			"$lt":  endTimeObj.UnixMilli(),
		},
	}

	// 1 در MongoDB یعنی صعودی (قدیم به جدید)
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

	// ✅ بخش جدید: نرمال‌سازی و تضمین نهایی (دقیقاً مثل لایو)
	for i := range candles {
		t := candles[i].Time

		// نرمال‌سازی: اگر زمان به میلی‌ثانیه بود، به ثانیه تبدیل کن
		if t > 32503680000 {
			t = t / 1000
			candles[i].Time = t // آپدیت فیلد اصلی به ثانیه برای هماهنگی با بک‌تست
		}

		// ساخت ReadableTime برای هماهنگی کامل با لایو
		candles[i].ReadableTime = time.Unix(t, 0).UTC()
	}

	// ✅ تضمین نهایی مرتب‌سازی از قدیم به جدید در سطح Go
	// (حتی اگر دیتابیس به هر دلیلی ترتیب را بهم ریخته باشد، این خط آن را اصلاح می‌کند)
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Time < candles[j].Time
	})

	return candles, nil
}
