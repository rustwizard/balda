alter table game_result_players
    add column if not exists words jsonb;

---- create above / drop below ----

alter table game_result_players drop column if exists words;
