alter table users
    add column if not exists role text not null default 'player';

create table if not exists refresh_tokens
(
    token_id   uuid        primary key default gen_random_uuid(),
    user_id    bigint      not null references users (user_id) on delete cascade,
    token_hash text        not null unique, -- HMAC-SHA256(JWT_SECRET, rawToken)
    issued_at  timestamptz not null default now(),
    expires_at timestamptz not null,
    revoked    boolean     not null default false,
    user_agent text,
    ip_addr    inet
);

create index if not exists idx_refresh_tokens_user_id on refresh_tokens (user_id);
create index if not exists idx_refresh_tokens_expires on refresh_tokens (expires_at) where revoked = false;

---- create above / drop below ----

drop table if exists refresh_tokens;
alter table users drop column if exists role;
