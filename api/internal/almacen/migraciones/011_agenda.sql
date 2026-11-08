-- La agenda (docs/nube.md, "Agenda"): un calendario iCal por usuario. La
-- dirección es secreta (la de Google trae la clave en el camino): se guarda
-- cifrada (AES-GCM con una clave que sale de SECRETO) y solo se muestra la
-- pista (el host y el final). Los próximos eventos se guardan ya expandidos
-- (se releen cada 5 min) para avisar sin bajar el .ics cada minuto.
create table agendas (
  usuario_id uuid primary key references usuarios on delete cascade,
  ics_cifrado bytea not null,
  ics_pista text not null,
  aviso int not null default 10 check (aviso between 0 and 15),
  proximos jsonb not null default '[]',
  leida timestamptz,
  error text not null default '',
  -- uid + inicio de lo que ya se avisó (los últimos 100), así un reinicio no repite.
  avisados text[] not null default '{}',
  -- el día (de los robots) del último resumen de mañana.
  resumen_dia date,
  creada timestamptz not null default now()
);
