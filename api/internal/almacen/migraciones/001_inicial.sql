-- SintéticaOS: cuentas, bichos, configs, imágenes, biblioteca y OTA.

create table usuarios (
  id uuid primary key default gen_random_uuid(),
  email text not null unique check (email = lower(email)),
  nombre text not null default '',
  admin boolean not null default false,
  creado timestamptz not null default now()
);

-- Links mágicos: un token largo para el link y un código corto para tipearlo.
create table enlaces (
  id bigserial primary key,
  email text not null,
  token_hash bytea not null unique,
  codigo_hash bytea not null,
  cliente text not null check (cliente in ('web', 'app')),
  intentos int not null default 0,
  ip text not null default '',
  vence timestamptz not null,
  usado timestamptz,
  creado timestamptz not null default now()
);
create index enlaces_email on enlaces (email, creado desc);

create table sesiones (
  token_hash bytea primary key,
  usuario_id uuid not null references usuarios on delete cascade,
  cliente text not null,
  descripcion text not null default '',
  creada timestamptz not null default now(),
  usada timestamptz not null default now(),
  vence timestamptz not null
);
create index sesiones_usuario on sesiones (usuario_id);

create table bichos (
  id text primary key check (id ~ '^[0-9A-F]{12}$'),
  usuario_id uuid references usuarios on delete set null,
  perfil text not null default '',
  apodo text not null default '',
  virtual boolean not null default false,
  conectado boolean not null default false,
  ultima_conexion timestamptz,
  estado bytea,
  estado_en timestamptz,
  creado timestamptz not null default now()
);
create index bichos_usuario on bichos (usuario_id);

-- Las credenciales MQTT de cada bicho. Una pendiente se vuelve activa cuando
-- el bicho se conecta con ella: es la prueba de que quien la pidió lo tiene.
create table credenciales (
  id bigserial primary key,
  bicho_id text not null references bichos on delete cascade,
  usuario_id uuid not null references usuarios on delete cascade,
  clave_hash bytea not null,
  estado text not null check (estado in ('pendiente', 'activa', 'revocada')),
  creada timestamptz not null default now(),
  vence timestamptz,
  usada timestamptz
);
create index credenciales_bicho on credenciales (bicho_id, estado);

-- Cada cambio de config es una versión nueva; la última es la que se quiere.
create table configs (
  id bigserial primary key,
  bicho_id text not null references bichos on delete cascade,
  version int not null,
  texto text not null,
  crc bigint not null,
  autor text not null check (autor in ('web', 'app', 'bicho', 'api')),
  usuario_id uuid references usuarios on delete set null,
  creada timestamptz not null default now(),
  unique (bicho_id, version)
);

-- Las imágenes que se quieren en el bicho (sin fila = espacio vacío).
create table imagenes (
  bicho_id text not null references bichos on delete cascade,
  espacio smallint not null check (espacio between 0 and 7),
  datos bytea not null,
  crc bigint not null,
  actualizada timestamptz not null default now(),
  primary key (bicho_id, espacio)
);

-- Lo último en lo que el bicho y la API estuvieron de acuerdo: para saber,
-- cuando no coinciden, quién cambió (sincronización de tres vías).
create table sincro (
  bicho_id text primary key references bichos on delete cascade,
  config_crc bigint,
  imagenes jsonb not null default '{}'
);

create table biblioteca (
  id uuid primary key default gen_random_uuid(),
  usuario_id uuid not null references usuarios on delete cascade,
  tipo text not null check (tipo in ('cara', 'animacion')),
  nombre text not null check (nombre ~ '^[a-z][a-z0-9-]{0,23}$'),
  documento jsonb not null,
  actualizado timestamptz not null default now(),
  unique (usuario_id, tipo, nombre)
);

create table firmwares (
  id bigserial primary key,
  perfil text not null,
  version text not null,
  tamano int not null,
  sha256 bytea not null,
  firma bytea not null,
  binario bytea not null,
  notas text not null default '',
  creado timestamptz not null default now(),
  unique (perfil, version)
);

create table actualizaciones (
  id bigserial primary key,
  bicho_id text not null references bichos on delete cascade,
  firmware_id bigint not null references firmwares on delete cascade,
  estado text not null check (estado in ('enviada', 'descargando', 'instalada', 'confirmada', 'fallida')),
  progreso int not null default 0,
  detalle text not null default '',
  creada timestamptz not null default now(),
  actualizada timestamptz not null default now()
);
create index actualizaciones_bicho on actualizaciones (bicho_id, creada desc);
