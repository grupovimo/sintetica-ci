-- Las reservas de la landing ("Reservá el tuyo"): las deja cualquiera, sin
-- cuenta; las ven los administradores en la web (/reservas).
create table reservas (
  id bigserial primary key,
  nombre text not null,
  contacto text not null,
  color text not null,
  mensaje text not null default '',
  ip text not null default '',
  creada timestamptz not null default now(),
  atendida boolean not null default false
);
create index reservas_creada on reservas (creada desc);
