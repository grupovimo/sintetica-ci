-- La charla sin IA en la nube (docs/nube.md, "Charla"): los gustos que el
-- robot aprendió charlando ("me gusta el fútbol", por el aviso 0xCC o por la
-- nube) y lo que la charla dejó en la config para una sola vez (una alarma o
-- un recordatorio "mañana a las 7"), para sacarlo cuando pase.

create table if not exists charla_gustos (
  bicho_id text not null references bichos on delete cascade,
  gusto text not null,
  clave text not null,                  -- el gusto en minúsculas (sin repetir)
  en timestamptz not null default now(),
  primary key (bicho_id, clave)
);

create table if not exists charla_una_vez (
  id bigserial primary key,
  bicho_id text not null references bichos on delete cascade,
  tipo text not null check (tipo in ('alarma', 'recordatorio')),
  vence timestamptz not null,           -- cuando ya pasó (un rato después de la hora)
  puesto jsonb not null,                -- lo que se puso en la config
  antes jsonb,                          -- el despertador que había (para volver a ponerlo)
  creado timestamptz not null default now()
);
create index if not exists charla_una_vez_vence on charla_una_vez (vence);

-- Las versiones de la config que deja la charla.
alter table configs drop constraint configs_autor_check;
alter table configs add constraint configs_autor_check
  check (autor in ('web', 'app', 'bicho', 'api', 'automatizacion', 'charla') or (autor like 'mcp:%' and length(autor) <= 60));
