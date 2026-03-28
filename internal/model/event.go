package model

// EventLocation represents an event location subdocument.
type EventLocation struct {
	Address string `bson:"address" json:"address"`
}

// Event represents an event MongoDB document from the events collection.
type Event struct {
	ID          string        `bson:"_id,omitempty" json:"id,omitempty"`
	Title       string        `bson:"title" json:"title"`
	Description string        `bson:"description,omitempty" json:"description,omitempty"`
	Location    EventLocation `bson:"location" json:"location"`
	CreatedAt   string        `bson:"created_at" json:"created_at"`
	CreatedBy   string        `bson:"created_by" json:"created_by"`
	StartedAt   string        `bson:"started_at" json:"started_at"`
	FinishedAt  string        `bson:"finished_at" json:"finished_at"`
}
