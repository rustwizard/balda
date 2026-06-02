create table if not exists player_state
(
    user_id       bigint
        constraint user_state_pk
            primary key,
    player_id       uuid default gen_random_uuid() not null,
    nickname        text,
    exp             bigint,
    flags           bigint,
    lives           bigint,
    created_at      timestamp with time zone default now()  not null,
    updated_at      timestamp with time zone default now()  not null
);

alter table player_state
    add constraint player_state_users_user_id_fk
        foreign key (user_id) references users;

---- create above / drop below ----

alter table player_state drop constraint if exists player_state_users_user_id_fk;
drop table if exists player_state;