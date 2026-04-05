package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

// EventFilter defines event list filtering and pagination options.
type EventFilter struct {
	ID            string
	Title         string
	Category      string
	City          string
	CreatedBy     string
	CreatedByName string
	StartedAtFrom string
	StartedAtTo   string
	PriceFrom     *uint64
	PriceTo       *uint64
	Limit         uint64
	Offset        uint64
}

// EventPatch defines mutable event fields.
type EventPatch struct {
	Category   *string
	Price      *uint64
	City       *string
	RemoveCity bool
}

// EventRepository provides access to events storage.
type EventRepository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, event model.Event) (model.Event, error)
	GetByID(ctx context.Context, id string) (model.Event, error)
	UpdateByIDAndOrganizer(ctx context.Context, id, organizerID string, patch EventPatch) (bool, error)
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
		{
			Keys: bson.D{{Key: "category", Value: 1}},
			Options: options.Index().
				SetName("category"),
		},
		{
			Keys: bson.D{{Key: "price", Value: 1}},
			Options: options.Index().
				SetName("price"),
		},
		{
			Keys: bson.D{{Key: "location.city", Value: 1}},
			Options: options.Index().
				SetName("location_city"),
		},
		{
			Keys: bson.D{{Key: "started_at", Value: 1}},
			Options: options.Index().
				SetName("started_at"),
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
		"category":    event.Category,
		"price":       event.Price,
		"description": event.Description,
		"location": bson.M{
			"address": event.Location.Address,
			"city":    event.Location.City,
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

// GetByID fetches an event by id.
func (r *MongoEventRepository) GetByID(ctx context.Context, id string) (model.Event, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return model.Event{}, ErrNotFound
	}

	var doc eventDoc
	if err := r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.Event{}, ErrNotFound
		}

		return model.Event{}, fmt.Errorf("find event by id: %w", err)
	}

	return decodeEventDocument(doc), nil
}

// UpdateByIDAndOrganizer applies partial update for organizer-owned event.
func (r *MongoEventRepository) UpdateByIDAndOrganizer(
	ctx context.Context,
	id, organizerID string,
	patch EventPatch,
) (bool, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, nil
	}

	filter := bson.M{
		"_id":        objectID,
		"created_by": organizerID,
	}

	setFields := bson.M{}
	unsetFields := bson.M{}

	if patch.Category != nil {
		setFields["category"] = *patch.Category
	}
	if patch.Price != nil {
		setFields["price"] = *patch.Price
	}
	if patch.City != nil {
		setFields["location.city"] = *patch.City
	}
	if patch.RemoveCity {
		unsetFields["location.city"] = ""
	}

	if len(setFields) == 0 && len(unsetFields) == 0 {
		count, err := r.collection.CountDocuments(ctx, filter)
		if err != nil {
			return false, fmt.Errorf("count event for patch: %w", err)
		}

		return count > 0, nil
	}

	update := bson.M{}
	if len(setFields) > 0 {
		update["$set"] = setFields
	}
	if len(unsetFields) > 0 {
		update["$unset"] = unsetFields
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, fmt.Errorf("update event by id and organizer: %w", err)
	}

	return result.MatchedCount > 0, nil
}

// List returns events matching filter and pagination.
func (r *MongoEventRepository) List(ctx context.Context, filter EventFilter) ([]model.Event, error) {
	query := bson.M{}
	if filter.ID != "" {
		objectID, err := primitive.ObjectIDFromHex(filter.ID)
		if err != nil {
			return []model.Event{}, nil
		}
		query["_id"] = objectID
	}
	if filter.Title != "" {
		query["title"] = bson.M{
			"$regex":   filter.Title,
			"$options": "i",
		}
	}
	if filter.Category != "" {
		query["category"] = filter.Category
	}
	if filter.City != "" {
		query["location.city"] = filter.City
	}
	if filter.CreatedBy != "" {
		query["created_by"] = filter.CreatedBy
	}
	if filter.StartedAtFrom != "" || filter.StartedAtTo != "" {
		rangeQuery := bson.M{}
		if filter.StartedAtFrom != "" {
			rangeQuery["$gte"] = filter.StartedAtFrom
		}
		if filter.StartedAtTo != "" {
			rangeQuery["$lte"] = filter.StartedAtTo
		}
		query["started_at"] = rangeQuery
	}
	if filter.PriceFrom != nil || filter.PriceTo != nil {
		rangeQuery := bson.M{}
		if filter.PriceFrom != nil {
			rangeQuery["$gte"] = *filter.PriceFrom
		}
		if filter.PriceTo != nil {
			rangeQuery["$lte"] = *filter.PriceTo
		}
		query["price"] = rangeQuery
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

	events := make([]model.Event, 0)
	for cursor.Next(ctx) {
		var doc eventDoc
		if decodeErr := cursor.Decode(&doc); decodeErr != nil {
			return nil, fmt.Errorf("decode event: %w", decodeErr)
		}

		event := decodeEventDocument(doc)
		// Legacy records may contain category with different letter-case.
		event.Category = strings.ToLower(event.Category)
		events = append(events, event)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}

type eventDoc struct {
	ID          primitive.ObjectID `bson:"_id"`
	Title       string             `bson:"title"`
	Category    string             `bson:"category,omitempty"`
	Price       uint64             `bson:"price"`
	Description string             `bson:"description,omitempty"`
	Location    struct {
		Address string `bson:"address"`
		City    string `bson:"city,omitempty"`
	} `bson:"location"`
	CreatedAt  string `bson:"created_at"`
	CreatedBy  string `bson:"created_by"`
	StartedAt  string `bson:"started_at"`
	FinishedAt string `bson:"finished_at"`
}

func decodeEventDocument(doc eventDoc) model.Event {
	return model.Event{
		ID:          doc.ID.Hex(),
		Title:       doc.Title,
		Category:    doc.Category,
		Price:       doc.Price,
		Description: doc.Description,
		Location: model.EventLocation{
			Address: doc.Location.Address,
			City:    doc.Location.City,
		},
		CreatedAt:  doc.CreatedAt,
		CreatedBy:  doc.CreatedBy,
		StartedAt:  doc.StartedAt,
		FinishedAt: doc.FinishedAt,
	}
}
