alter table player_state
    add column if not exists total_games int not null default 0,
    add column if not exists consecutive_wins int not null default 0;

alter table player_state alter column flags set default 0;
update player_state set flags = 0 where flags is null;

alter table game_result_players
    add column if not exists best_word_length int not null default 0;

---- create above / drop below ----

alter table game_result_players drop column if exists best_word_length;
alter table player_state drop column if exists consecutive_wins;
alter table player_state drop column if exists total_games;
