package model

// EventReview stores a user review for an event.
type EventReview struct {
	ID        string `json:"id"`
	EventID   string `json:"event_id"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
	Rating    int    `json:"rating"`
	UpdatedAt string `json:"updated_at"`
}

// EventReviewsSummary stores aggregated reviews for a logical event.
type EventReviewsSummary struct {
	Count  uint64  `json:"count"`
	Rating float64 `json:"rating"`
}

// EventReviewsCounters stores non-rounded totals for aggregation.
type EventReviewsCounters struct {
	Count       uint64
	TotalRating uint64
}
