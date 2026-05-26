package repository

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jRecommendationRepository stores recommendation graph in Neo4j.
type Neo4jRecommendationRepository struct {
	driver neo4j.DriverWithContext
}

// NewNeo4jRecommendationRepository creates a Neo4j recommendation repository.
func NewNeo4jRecommendationRepository(driver neo4j.DriverWithContext) *Neo4jRecommendationRepository {
	return &Neo4jRecommendationRepository{driver: driver}
}

// UpsertUser creates a user node if it does not exist.
func (r *Neo4jRecommendationRepository) UpsertUser(ctx context.Context, userID string) error {
	const query = `
MERGE (:User {id: $user_id})
`

	_, err := neo4j.ExecuteQuery(
		ctx,
		r.driver,
		query,
		map[string]any{
			"user_id": userID,
		},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return fmt.Errorf("upsert user node: %w", err)
	}

	return nil
}

// UpsertEvent creates an event node if it does not exist.
func (r *Neo4jRecommendationRepository) UpsertEvent(
	ctx context.Context,
	eventID, title, startedAt string,
) error {
	const query = `
MERGE (e:Event {id: $event_id})
SET e.title = $title, e.started_at = $started_at
`

	_, err := neo4j.ExecuteQuery(
		ctx,
		r.driver,
		query,
		map[string]any{
			"event_id":   eventID,
			"title":      title,
			"started_at": startedAt,
		},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return fmt.Errorf("upsert event node: %w", err)
	}

	return nil
}

// UpsertLike creates a like edge in recommendation graph.
func (r *Neo4jRecommendationRepository) UpsertLike(ctx context.Context, userID, eventID string) error {
	const query = `
MERGE (u:User {id: $user_id})
MERGE (e:Event {id: $event_id})
MERGE (u)-[:LIKED]->(e)
`

	_, err := neo4j.ExecuteQuery(
		ctx,
		r.driver,
		query,
		map[string]any{
			"user_id":  userID,
			"event_id": eventID,
		},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return fmt.Errorf("upsert liked edge: %w", err)
	}

	return nil
}

// ListRecommendedEventIDs returns ordered recommendation candidates for a user.
func (r *Neo4jRecommendationRepository) ListRecommendedEventIDs(
	ctx context.Context,
	userID string,
) ([]string, error) {
	const query = `
MATCH (u:User {id: $user_id})-[:LIKED]->(:Event)<-[:LIKED]-(peer:User)-[:LIKED]->(candidate:Event)
WHERE NOT (u)-[:LIKED]->(candidate)
WITH DISTINCT candidate
MATCH (candidate)<-[:LIKED]-(:User)
WITH candidate, count(*) AS popularity
ORDER BY candidate.title ASC, datetime(candidate.started_at) ASC
WITH candidate.title AS title, collect({
  id: candidate.id,
  popularity: popularity,
  started_at: candidate.started_at
}) AS same_title
WITH same_title[0] AS picked
ORDER BY picked.popularity DESC, datetime(picked.started_at) ASC
RETURN picked.id AS event_id
`

	result, err := neo4j.ExecuteQuery(
		ctx,
		r.driver,
		query,
		map[string]any{
			"user_id": userID,
		},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("query recommended events: %w", err)
	}

	ids := make([]string, 0, len(result.Records))
	for _, record := range result.Records {
		value, found := record.Get("event_id")
		if !found {
			continue
		}

		eventID, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected event_id type %T", value)
		}

		ids = append(ids, eventID)
	}

	return ids, nil
}
