-- Integraciones (docs/nube.md, "Integraciones"): Telegram con el bot oficial,
-- automatizaciones por bicho ("cuando pasa X → hacer Y") con su historial, y
-- el secreto con el que se firman los webhooks de cada usuario.

-- Un chat de Telegram por usuario (y un usuario por chat).
create table telegram_vinculos (
  usuario_id uuid primary key references usuarios on delete cascade,
  chat_id bigint not null unique,
  nombre text not null default '',
  creado timestamptz not null default now()
);

-- Los códigos de "/start <código>": un solo uso, 15 minutos. Solo su hash.
create table telegram_codigos (
  codigo_hash bytea primary key,
  usuario_id uuid not null references usuarios on delete cascade,
  vence timestamptz not null,
  usado timestamptz,
  creado timestamptz not null default now()
);
create index telegram_codigos_usuario on telegram_codigos (usuario_id, creado desc);

-- Hasta dónde se leyó getUpdates (así un reinicio no repite comandos).
create table telegram_offset (
  id int primary key check (id = 1),
  offset_siguiente bigint not null
);

-- Con qué se firman los webhooks (HMAC-SHA256): se tiene que poder leer.
create table webhook_secretos (
  usuario_id uuid primary key references usuarios on delete cascade,
  secreto text not null,
  creado timestamptz not null default now()
);

-- La lista entera de automatizaciones de un bicho (el JSON que manda la web).
create table automatizaciones (
  bicho_id text primary key references bichos on delete cascade,
  lista jsonb not null default '[]',
  actualizada timestamptz not null default now()
);

-- Lo que corrió (las últimas 50 por bicho).
create table automatizaciones_historial (
  id bigserial primary key,
  bicho_id text not null references bichos on delete cascade,
  automatizacion text not null,
  evento text not null,
  accion text not null,
  ok boolean not null,
  detalle text not null default '',
  en timestamptz not null default now()
);
create index automatizaciones_historial_bicho on automatizaciones_historial (bicho_id, id desc);
