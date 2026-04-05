package repository

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
)

// UserFilter defines organizer list filtering and pagination options.
type UserFilter struct {
	ID     string
	Name   string
	Limit  uint64
	Offset uint64
}

// UserRepository provides access to users storage.
type UserRepository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, user model.User) (model.User, error)
	GetByID(ctx context.Context, id string) (model.User, error)
	GetByUsername(ctx context.Context, username string) (model.User, error)
	List(ctx context.Context, filter UserFilter) ([]model.User, error)
}

// MongoUserRepository stores users in MongoDB.
type MongoUserRepository struct {
	collection *mongo.Collection
}

// NewMongoUserRepository creates a users repository.
func NewMongoUserRepository(db *mongo.Database) *MongoUserRepository {
	return &MongoUserRepository{collection: db.Collection("users")}
}

// EnsureIndexes creates required indexes for users collection.
func (r *MongoUserRepository) EnsureIndexes(ctx context.Context) error {
	models := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "username", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("username_unique"),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("create user indexes: %w", err)
	}

	return nil
}

// Create inserts a user document and returns it with assigned id.
func (r *MongoUserRepository) Create(ctx context.Context, user model.User) (model.User, error) {
	result, err := r.collection.InsertOne(ctx, bson.M{
		"full_name":     user.FullName,
		"username":      user.Username,
		"password_hash": user.PasswordHash,
	})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return model.User{}, ErrAlreadyExists
		}

		return model.User{}, fmt.Errorf("insert user: %w", err)
	}

	objectID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return model.User{}, fmt.Errorf("unexpected inserted id type %T", result.InsertedID)
	}

	user.ID = objectID.Hex()
	return user, nil
}

// GetByUsername fetches user by username.
func (r *MongoUserRepository) GetByUsername(ctx context.Context, username string) (model.User, error) {
	var userDoc userDoc

	err := r.collection.FindOne(ctx, bson.M{"username": username}).Decode(&userDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.User{}, ErrNotFound
		}

		return model.User{}, fmt.Errorf("find user by username: %w", err)
	}

	return decodeUserDocument(userDoc), nil
}

// GetByID fetches user by id.
func (r *MongoUserRepository) GetByID(ctx context.Context, id string) (model.User, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return model.User{}, ErrNotFound
	}

	var userDoc userDoc
	err = r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&userDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.User{}, ErrNotFound
		}

		return model.User{}, fmt.Errorf("find user by id: %w", err)
	}

	return decodeUserDocument(userDoc), nil
}

// List returns users matching filter and pagination.
func (r *MongoUserRepository) List(ctx context.Context, filter UserFilter) ([]model.User, error) {
	query := bson.M{}
	if filter.ID != "" {
		objectID, err := primitive.ObjectIDFromHex(filter.ID)
		if err != nil {
			return []model.User{}, nil
		}
		query["_id"] = objectID
	}
	if filter.Name != "" {
		query["full_name"] = bson.M{
			"$regex":   filter.Name,
			"$options": "i",
		}
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "full_name", Value: 1}})
	findOptions.SetSkip(int64(filter.Offset))
	if filter.Limit > 0 {
		findOptions.SetLimit(int64(filter.Limit))
	}

	cursor, err := r.collection.Find(ctx, query, findOptions)
	if err != nil {
		return nil, fmt.Errorf("find users: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	users := make([]model.User, 0)
	for cursor.Next(ctx) {
		var doc userDoc
		if decodeErr := cursor.Decode(&doc); decodeErr != nil {
			return nil, fmt.Errorf("decode user: %w", decodeErr)
		}
		users = append(users, decodeUserDocument(doc))
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

type userDoc struct {
	ID           primitive.ObjectID `bson:"_id"`
	FullName     string             `bson:"full_name"`
	Username     string             `bson:"username"`
	PasswordHash string             `bson:"password_hash"`
}

func decodeUserDocument(userDoc userDoc) model.User {
	return model.User{
		ID:           userDoc.ID.Hex(),
		FullName:     userDoc.FullName,
		Username:     userDoc.Username,
		PasswordHash: userDoc.PasswordHash,
	}
}
