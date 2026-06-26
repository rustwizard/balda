alter table player_state
    add column if not exists rating int not null default 1000;
