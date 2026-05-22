CREATE KEYSPACE IF NOT EXISTS __CASSANDRA_KEYSPACE__
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

CREATE TABLE IF NOT EXISTS __CASSANDRA_KEYSPACE__.event_reactions (
    event_id text,
    created_by text,
    like_value tinyint,
    created_at timestamp,
    PRIMARY KEY ((event_id), created_by)
);

CREATE INDEX IF NOT EXISTS event_reactions_like_value_idx
ON __CASSANDRA_KEYSPACE__.event_reactions (like_value);

CREATE TABLE IF NOT EXISTS __CASSANDRA_KEYSPACE__.event_reviews (
    event_id text,
    created_by text,
    id uuid,
    rating tinyint,
    comment text,
    created_at timestamp,
    updated_at timestamp,
    PRIMARY KEY ((event_id), created_by)
);

CREATE INDEX IF NOT EXISTS event_reviews_id_idx
ON __CASSANDRA_KEYSPACE__.event_reviews (id);
