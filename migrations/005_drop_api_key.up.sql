alter table users drop column if exists api_key;

---- create above / drop below ----

alter table users add column if not exists api_key uuid default gen_random_uuid() not null;
