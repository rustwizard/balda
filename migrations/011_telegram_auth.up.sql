alter table users
    add column if not exists telegram_id bigint;

create unique index if not exists users_telegram_id_uindex
    on users (telegram_id)
    where telegram_id is not null;

-- Telegram users have no email; allow NULL emails while keeping uniqueness
-- for the ones that are set.
drop index if exists users_email_uindex;
create unique index users_email_uindex
    on users (email)
    where email is not null;

---- create above / drop below ----

drop index if exists users_email_uindex;
create unique index users_email_uindex
    on users (email);

drop index if exists users_telegram_id_uindex;
alter table users drop column if exists telegram_id;
