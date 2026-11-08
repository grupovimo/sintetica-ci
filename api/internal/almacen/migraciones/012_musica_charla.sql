-- Música (Spotify) y charla con IA (docs/nube.md, "Música" y "Charla").

-- La cuenta de Spotify de un usuario: el refresh token, cifrado como la
-- dirección de la agenda (AES-GCM con una clave que sale de SECRETO).
create table spotify_cuentas (
  usuario_id uuid primary key references usuarios on delete cascade,
  refresh_cifrado bytea not null,
  nombre text not null default '',
  creada timestamptz not null default now()
);

-- Cuántas charlas lleva cada usuario por día (el tope es de 60).
create table charlas_uso (
  usuario_id uuid not null references usuarios on delete cascade,
  dia date not null,
  cantidad int not null default 0,
  primary key (usuario_id, dia)
);

-- El último robot con el que charló (Telegram, si tiene varios).
create table charla_robot (
  usuario_id uuid primary key references usuarios on delete cascade,
  bicho_id text not null references bichos on delete cascade,
  en timestamptz not null default now()
);
