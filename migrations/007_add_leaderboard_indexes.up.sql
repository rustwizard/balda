CREATE INDEX IF NOT EXISTS idx_player_state_rating_updated
    ON player_state (rating DESC, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_player_state_exp_updated
    ON player_state (exp DESC, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_player_state_updated_at
    ON player_state (updated_at DESC);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_player_state_rating_updated;
DROP INDEX IF EXISTS idx_player_state_exp_updated;
DROP INDEX IF EXISTS idx_player_state_updated_at;
