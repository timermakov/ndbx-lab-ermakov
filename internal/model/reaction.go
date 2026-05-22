package model

// ReactionValue stores the type of user's reaction to an event.
type ReactionValue int8

const (
	// ReactionLike represents a positive event reaction.
	ReactionLike ReactionValue = 1
	// ReactionDislike represents a negative event reaction.
	ReactionDislike ReactionValue = -1
)

// EventReactions stores likes/dislikes counters for a logical event.
type EventReactions struct {
	Likes    uint64 `json:"likes"`
	Dislikes uint64 `json:"dislikes"`
}
