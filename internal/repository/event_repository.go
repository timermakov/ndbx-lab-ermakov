package repository

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

// EventFilter defines event list filtering and pagination options.
type EventFilter struct {
	Title  string
	Limit  uint64
	Offset uint64
}

// EventRepository provides access to events storage.
type EventRepository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, event model.Event) (model.Event, error)
	List(ctx context.Context, filter EventFilter) ([]model.Event, error)
}

// MongoEventRepository stores events in MongoDB.
type MongoEventRepository struct {
	collection *mongo.Collection
}

// NewMongoEventRepository creates an events repository.
func NewMongoEventRepository(db *mongo.Database) *MongoEventRepository {
	return &MongoEventRepository{collection: db.Collection("events")}
}

// EnsureIndexes creates required indexes for events collection.
func (r *MongoEventRepository) EnsureIndexes(ctx context.Context) error {
	models := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "title", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("title_unique"),
		},
		{
			Keys: bson.D{{Key: "title", Value: 1}, {Key: "created_by", Value: 1}},
			Options: options.Index().
				SetName("title_created_by"),
		},
		{
			Keys: bson.D{{Key: "created_by", Value: 1}},
			Options: options.Index().
				SetName("created_by"),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("create event indexes: %w", err)
	}

	return nil
}

// Create inserts an event document and returns it with assigned id.
func (r *MongoEventRepository) Create(ctx context.Context, event model.Event) (model.Event, error) {
	result, err := r.collection.InsertOne(ctx, bson.M{
		"title":       event.Title,
		"description": event.Description,
		"location": bson.M{
			"address": event.Location.Address,
		},
		"created_at":  event.CreatedAt,
		"created_by":  event.CreatedBy,
		"started_at":  event.StartedAt,
		"finished_at": event.FinishedAt,
	})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return model.Event{}, ErrAlreadyExists
		}

		return model.Event{}, fmt.Errorf("insert event: %w", err)
	}

	objectID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return model.Event{}, fmt.Errorf("unexpected inserted id type %T", result.InsertedID)
	}

	event.ID = objectID.Hex()
	return event, nil
}

// List returns events matching filter and pagination.
func (r *MongoEventRepository) List(ctx context.Context, filter EventFilter) ([]model.Event, error) {
	query := bson.M{}
	if filter.Title != "" {
		query["title"] = bson.M{
			"$regex":   filter.Title,
			"$options": "i",
		}
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "created_at", Value: -1}})
	findOptions.SetSkip(int64(filter.Offset))
	if filter.Limit > 0 {
		limit := int64(filter.Limit)
		findOptions.SetLimit(limit)
	}

	cursor, err := r.collection.Find(ctx, query, findOptions)
	if err != nil {
		return nil, fmt.Errorf("find events: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	type eventDoc struct {
		ID          primitive.ObjectID `bson:"_id"`
		Title       string             `bson:"title"`
		Description string             `bson:"description,omitempty"`
		Location    struct {
			Address string `bson:"address"`
		} `bson:"location"`
		CreatedAt  string `bson:"created_at"`
		CreatedBy  string `bson:"created_by"`
		StartedAt  string `bson:"started_at"`
		FinishedAt string `bson:"finished_at"`
	}

	events := make([]model.Event, 0)
	for cursor.Next(ctx) {
		var doc eventDoc
		if decodeErr := cursor.Decode(&doc); decodeErr != nil {
			return nil, fmt.Errorf("decode event: %w", decodeErr)
		}

		events = append(events, model.Event{
			ID:          doc.ID.Hex(),
			Title:       doc.Title,
			Description: doc.Description,
			Location: model.EventLocation{
				Address: doc.Location.Address,
			},
			CreatedAt:  doc.CreatedAt,
			CreatedBy:  doc.CreatedBy,
			StartedAt:  doc.StartedAt,
			FinishedAt: doc.FinishedAt,
		})
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}
