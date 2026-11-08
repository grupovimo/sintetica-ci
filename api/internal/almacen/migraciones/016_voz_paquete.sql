-- El paquete de voz sin internet (docs/nube.md, "Paquete de voz").

-- Los audios ya generados, por usuario (así dos robots con la misma voz y la
-- misma frase comparten el mp3). clave = sha256 de la voz y el texto final.
create table voz_clips (
  usuario_id uuid not null references usuarios on delete cascade,
  clave text not null,
  voz text not null,
  texto text not null,
  datos bytea not null,
  creado timestamptz not null default now(),
  primary key (usuario_id, clave)
);

-- El paquete de cada robot: qué frases entraron (en orden de prioridad, con
-- su clave y su tamaño), la voz y la versión (sube cuando cambia lo que hay).
create table voz_paquetes (
  bicho_id text primary key references bichos on delete cascade,
  usuario_id uuid not null references usuarios on delete cascade,
  voz text not null default '',
  version int not null default 0,
  huella text not null default '',
  frases jsonb not null default '[]',
  estado text not null default 'nuevo',
  error text not null default '',
  tope_bytes int not null default 0,
  actualizado timestamptz not null default now()
);

-- Los caracteres que se leyeron con la voz gratis para los paquetes (su
-- propio tope, aparte del de leer en voz alta).
create table voz_paquete_uso (
  usuario_id uuid not null references usuarios on delete cascade,
  dia date not null,
  caracteres int not null default 0,
  primary key (usuario_id, dia)
);
