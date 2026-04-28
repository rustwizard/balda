create table if not exists game_results
(
    id            bigserial primary key,
    game_id       uuid        not null unique,
    winner_id     uuid,                                                   -- null means draw
    finish_reason text        not null,                                   -- 'board_full' | 'kick' | 'accept_end'
    finished_at   timestamptz not null default now()
);

create table if not exists game_result_players
(
    game_result_id bigint not null references game_results (id) on delete cascade,
    player_id      uuid   not null,
    score          int    not null,
    words_count    int    not null,
    exp_gained     int    not null,
    primary key (game_result_id, player_id)
);

---- create above / drop below ----

drop table if exists game_result_players;
drop table if exists game_results;
