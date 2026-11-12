-- Las listas (docs/nube.md, "Listas"): la del súper, pendientes o la que sea,
-- de la cuenta (no de un robot): lo que se anota en la compu aparece en el
-- celu y al revés. Topes: 30 listas por usuario y 200 cosas por lista (en
-- almacen/listas.go).
create table if not exists listas (
  id uuid primary key default gen_random_uuid(),
  usuario_id uuid not null references usuarios on delete cascade,
  nombre text not null,
  creada timestamptz not null default now()
);
create unique index if not exists listas_usuario_nombre on listas (usuario_id, lower(nombre));

create table if not exists listas_items (
  id uuid primary key default gen_random_uuid(),
  lista_id uuid not null references listas on delete cascade,
  texto text not null,
  hecho boolean not null default false,
  creado timestamptz not null default now(),
  hecho_en timestamptz,
  orden int not null default 0
);
create index if not exists listas_items_lista on listas_items (lista_id, orden);
