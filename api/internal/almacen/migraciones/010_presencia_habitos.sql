-- Presencia, hábitos y avisos entrantes (v1.9, docs/nube.md, "Presencia",
-- "Hábitos" y "Avisos entrantes").

-- Llegadas y salidas (de la app del celu o del robot) y cuándo se durmió el
-- robot: de acá salen los horarios aprendidos (los últimos 14 días). Se
-- guardan 60 días.
create table presencias (
  id bigserial primary key,
  bicho_id text not null references bichos on delete cascade,
  tipo text not null check (tipo in ('llegaste', 'te-fuiste', 'duerme')),
  en timestamptz not null default now()
);
create index presencias_bicho on presencias (bicho_id, en desc);

-- Un hábito hecho en un día (de la zona de los robots). El nombre es el del
-- hábito cuando se marcó: si cambia el hábito de ese lugar, empieza de cero.
create table habitos_hechos (
  bicho_id text not null references bichos on delete cascade,
  indice smallint not null check (indice between 0 and 3),
  dia date not null,
  nombre text not null,
  en timestamptz not null default now(),
  primary key (bicho_id, indice, dia)
);

-- Los avisos entrantes (POST /v1/entrantes/{token}): el token sale de la
-- semilla y del secreto del servidor (así se puede volver a mostrar); en la
-- base, solo la semilla y su hash. El token de Mercado Pago, cifrado.
create table entrantes (
  usuario_id uuid primary key references usuarios on delete cascade,
  semilla bytea not null,
  token_hash bytea not null unique,
  mp_cifrado bytea,
  creado timestamptz not null default now()
);
