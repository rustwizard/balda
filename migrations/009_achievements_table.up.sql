CREATE TABLE IF NOT EXISTS achievements (
    id text PRIMARY KEY,
    name text NOT NULL,
    description text,
    icon_url text,
    condition_type text NOT NULL,
    operator text NOT NULL DEFAULT 'gte',
    threshold int NOT NULL DEFAULT 1,
    bit_position int NOT NULL UNIQUE,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_achievements_bit_position ON achievements (bit_position);

INSERT INTO achievements (id, name, description, condition_type, operator, threshold, bit_position)
VALUES
    ('first_game', 'Дебютант', 'Сыграть первую партию', 'total_games', 'gte', 1, 0),
    ('first_win', 'Первая победа', 'Одержать первую победу', 'win', 'gte', 1, 1),
    ('high_scorer_50', 'Рекордсмен', 'Набрать 50+ очков за партию', 'score', 'gte', 50, 2),
    ('wordsmith_10', 'Словесный мастер', 'Составить 10+ слов за партию', 'words_count', 'gte', 10, 3),
    ('giant_word', 'Гигант', 'Составить слово из 10+ букв', 'best_word_length', 'gte', 10, 4),
    ('winning_streak_3', 'Победная серия', '3 победы подряд', 'consecutive_wins', 'gte', 3, 5),
    ('veteran_10', 'Ветеран', 'Сыграть 10 партий', 'total_games', 'gte', 10, 6)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    condition_type = EXCLUDED.condition_type,
    operator = EXCLUDED.operator,
    threshold = EXCLUDED.threshold,
    bit_position = EXCLUDED.bit_position,
    updated_at = now();

---- create above / drop below ----

DROP TABLE IF EXISTS achievements;
