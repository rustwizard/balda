create table if not exists user_state
(
    user_id       bigint
        constraint user_state_pk
            primary key,
    nickname        text,
    exp             bigint,
    flags           bigint,
    lives           bigint,
    created_at      timestamp with time zone default now()  not null,
    updated_at      timestamp with time zone default now()  not null
);

alter table user_state
    owner to balda;

alter table user_state
    add constraint user_state_users_user_id_fk
        foreign key (user_id) references users;