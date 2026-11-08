-- Entrar con Telegram (docs/nube.md, "Cuentas"): el link
-- https://t.me/<bot>?start=entrar-<token> y el código de 6 dígitos que manda
-- el bot. Las cuentas que se crean desde Telegram no tienen email.

alter table usuarios alter column email drop not null;

create table if not exists telegram_entradas (
  id bigserial primary key,
  token_hash bytea not null unique,   -- el del link (solo el hash)
  secreto_hash bytea not null,        -- el del navegador o la app que lo pidió
  cliente text not null check (cliente in ('web', 'app')),
  ip text not null default '',
  estado text not null default 'pendiente' check (estado in ('pendiente', 'confirmar', 'aprobada', 'usada', 'rechazada')),
  chat_id bigint,                     -- quién tocó Start (un solo chat por entrada)
  usuario_id uuid references usuarios on delete cascade,
  codigo_hash bytea,                  -- el de 6 dígitos que mandó el bot
  intentos int not null default 0,
  sesion_hash bytea,                  -- la sesión que abrió (para "No fui yo")
  vence timestamptz not null,
  creado timestamptz not null default now()
);
create index if not exists telegram_entradas_chat on telegram_entradas (chat_id, creado desc);
