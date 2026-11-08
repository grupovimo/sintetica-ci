-- La voz gratis (Edge TTS) para todos (docs/nube.md, "Voz").

-- Lo que eligió cada usuario (sin fila: prendida, con la voz por defecto).
create table voz_edge (
  usuario_id uuid primary key references usuarios on delete cascade,
  prendida boolean not null default true,
  voz text not null default ''
);

-- Cuántos caracteres leyó Edge por día (un tope para no abusar del servicio).
create table voz_edge_uso (
  usuario_id uuid not null references usuarios on delete cascade,
  dia date not null,
  caracteres int not null default 0,
  primary key (usuario_id, dia)
);
