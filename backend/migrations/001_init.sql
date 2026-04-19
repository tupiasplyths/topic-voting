CREATE TABLE IF NOT EXISTS topics (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title         VARCHAR(255) NOT NULL,
    description   TEXT         DEFAULT '',
    is_active     BOOLEAN      NOT NULL DEFAULT FALSE,
    classifier_threshold FLOAT  DEFAULT 0.5,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    closed_at     TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_topics_one_active ON topics (is_active) WHERE is_active = TRUE;

CREATE TABLE IF NOT EXISTS votes (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id          UUID         NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    username          VARCHAR(100) NOT NULL,
    raw_message       TEXT         NOT NULL,
    classified_label  VARCHAR(255) NOT NULL,
    confidence        FLOAT        NOT NULL DEFAULT 0.0,
    weight            INT          NOT NULL DEFAULT 1,
    is_donation       BOOLEAN      NOT NULL DEFAULT FALSE,
    bits_amount       INT          NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_votes_topic_label ON votes (topic_id, classified_label);
CREATE INDEX IF NOT EXISTS idx_votes_topic_created ON votes (topic_id, created_at DESC);

CREATE MATERIALIZED VIEW IF NOT EXISTS vote_tallies AS
SELECT
    topic_id,
    classified_label,
    SUM(weight) AS total_weight,
    COUNT(*)    AS vote_count,
    MAX(created_at) AS last_vote_at
FROM votes
GROUP BY topic_id, classified_label
ORDER BY topic_id, total_weight DESC;

CREATE UNIQUE INDEX IF NOT EXISTS idx_vote_tallies_topic_label ON vote_tallies (topic_id, classified_label);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version   TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);