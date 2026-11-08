-- La voz de ElevenLabs con la cuenta de cada usuario (docs/nube.md, "Voz").

-- La clave va cifrada como el refresh token de Spotify (AES-GCM con una clave
-- que sale de SECRETO) y nunca se devuelve. Los topes los elige el usuario
-- para que no se le dispare la factura.
create table elevenlabs_cuentas (
  usuario_id uuid primary key references usuarios on delete cascade,
  clave_cifrada bytea not null,
  voz text not null default '',
  voz_nombre text not null default '',
  leer boolean not null default true,
  tope_caracteres int not null default 5000,
  tope_minutos int not null default 10,
  -- Las client tools creadas en su cuenta (nombre → id).
  herramientas jsonb not null default '{}',
  creada timestamptz not null default now()
);

-- Cuántos caracteres se leyeron en voz alta por día.
create table elevenlabs_uso (
  usuario_id uuid not null references usuarios on delete cascade,
  dia date not null,
  caracteres int not null default 0,
  primary key (usuario_id, dia)
);

-- Un agente conversacional por robot, en la cuenta del dueño. La huella es
-- de lo que se le mandó la última vez (si cambia, se actualiza).
create table elevenlabs_agentes (
  bicho_id text not null references bichos on delete cascade,
  usuario_id uuid not null references usuarios on delete cascade,
  agente_id text not null,
  huella text not null default '',
  actualizado timestamptz not null default now(),
  primary key (bicho_id, usuario_id)
);
