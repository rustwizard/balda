create table if not exists users
(
    user_id       bigserial
        constraint users_pk
            primary key,
    first_name    text,
    last_name     text,
    email         text,
    hash_password text,
    confirmed     boolean                  default false,
    api_key       uuid                     default gen_random_uuid() not null,
    created_at    timestamp with time zone default now()             not null,
    updated_at    timestamp with time zone default now()             not null
);

create unique index users_email_uindex
    on users (email);

