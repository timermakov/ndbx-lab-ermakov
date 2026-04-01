package model

// User represents a user MongoDB document from the users collection.
type User struct {
	ID           string `bson:"_id,omitempty" json:"id,omitempty"`
	FullName     string `bson:"full_name" json:"full_name"`
	Username     string `bson:"username" json:"username"`
	PasswordHash string `bson:"password_hash" json:"-"`
}
