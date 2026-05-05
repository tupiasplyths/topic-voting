DROP MATERIALIZED VIEW IF EXISTS vote_tallies;
DROP INDEX IF EXISTS idx_vote_tallies_topic_label;

ALTER TABLE topics ADD COLUMN voting_mode VARCHAR(20) NOT NULL DEFAULT 'chat';
ALTER TABLE topics ADD CONSTRAINT chk_voting_mode CHECK (voting_mode IN ('chat', 'donation'));

ALTER TABLE votes ALTER COLUMN weight TYPE DECIMAL(12,2);

ALTER TABLE votes ADD COLUMN donation_amount DECIMAL(12,2) NOT NULL DEFAULT 0;
ALTER TABLE votes ADD COLUMN donation_currency VARCHAR(3) NOT NULL DEFAULT '';

CREATE MATERIALIZED VIEW vote_tallies AS
SELECT
    topic_id,
    classified_label,
    SUM(weight) AS total_weight,
    COUNT(*)    AS vote_count,
    MAX(created_at) AS last_vote_at
FROM votes
GROUP BY topic_id, classified_label
ORDER BY topic_id, total_weight DESC;

CREATE UNIQUE INDEX idx_vote_tallies_topic_label ON vote_tallies (topic_id, classified_label);
