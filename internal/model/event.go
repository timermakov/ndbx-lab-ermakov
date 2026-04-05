// Package model contains domain and persistence models.
package model

// EventLocation represents an event location subdocument.
type EventLocation struct {
	Address string `bson:"address" json:"address"`
	City    string `bson:"city,omitempty" json:"city,omitempty"`
}

// Event represents an event MongoDB document from the events collection.
type Event struct {
	ID          string        `bson:"_id,omitempty" json:"id,omitempty"`
	Title       string        `bson:"title" json:"title"`
	Category    string        `bson:"category,omitempty" json:"category,omitempty"`
	Price       uint64        `bson:"price" json:"price"`
	Description string        `bson:"description,omitempty" json:"description,omitempty"`
	Location    EventLocation `bson:"location" json:"location"`
	CreatedAt   string        `bson:"created_at" json:"created_at"`
	CreatedBy   string        `bson:"created_by" json:"created_by"`
	StartedAt   string        `bson:"started_at" json:"started_at"`
	FinishedAt  string        `bson:"finished_at" json:"finished_at"`
}
