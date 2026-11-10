-- El centro de control del dueño (/admin): los componentes con su costo y
-- dónde se compran, la lista de materiales de cada modelo, las compras, las
-- unidades (de armada a desplegada, atadas al bicho real) y la preventa del
-- cubito con las señas de Mercado Pago. Solo lo ven los administradores: qué
-- placas y componentes se usan vive acá y en ningún archivo del repo.

create table if not exists componentes (
  id bigserial primary key,
  nombre text not null,
  categoria text not null default '',
  proveedor text not null default '',
  link text not null default '',
  moneda text not null default 'USD' check (moneda in ('USD', 'ARS')),
  costo numeric(14, 2) not null default 0 check (costo >= 0),
  stock int not null default 0,
  stock_minimo int not null default 0 check (stock_minimo >= 0),
  notas text not null default '',
  creado timestamptz not null default now(),
  actualizado timestamptz not null default now()
);

-- La lista de materiales: cuántos de cada componente lleva un modelo (la clave
-- de la pieza: unidad, broche, hebilla...).
create table if not exists modelo_componentes (
  modelo text not null,
  componente_id bigint not null references componentes on delete cascade,
  cantidad numeric(10, 3) not null check (cantidad > 0),
  primary key (modelo, componente_id)
);

-- Lo pedido a un proveedor. Al llegar (recibida) suma al stock.
create table if not exists compras (
  id bigserial primary key,
  componente_id bigint not null references componentes on delete cascade,
  cantidad int not null check (cantidad > 0),
  costo_unitario numeric(14, 2) not null default 0 check (costo_unitario >= 0),
  moneda text not null default 'USD' check (moneda in ('USD', 'ARS')),
  proveedor text not null default '',
  link text not null default '',
  pedida date not null default current_date,
  recibida date,
  notas text not null default '',
  creada timestamptz not null default now()
);
create index if not exists compras_pedida on compras (pedida desc, id desc);

-- Cada unidad física, con su número grabado. El costo queda congelado el día
-- que se arma (la lista de materiales con el dólar oficial de ese día).
create table if not exists unidades (
  id bigserial primary key,
  serie text not null unique,
  modelo text not null,
  color text not null default '',
  estado text not null default 'armada'
    check (estado in ('armada', 'vendida', 'desplegada', 'garantia', 'devuelta', 'baja')),
  bicho_id text unique references bichos on delete set null,
  costo_usd numeric(14, 2) not null default 0,
  costo_ars numeric(14, 2) not null default 0,
  dolar numeric(14, 2) not null default 0,
  cliente text not null default '',
  contacto text not null default '',
  precio numeric(14, 2) not null default 0,
  moneda text not null default 'ARS' check (moneda in ('USD', 'ARS')),
  canal text not null default '',
  cobrado boolean not null default false,
  vendida_en date,
  desplegada_en date,
  notas text not null default '',
  historial jsonb not null default '[]',
  creada timestamptz not null default now()
);
create index if not exists unidades_creada on unidades (creada desc);

-- La preventa (una fila por modelo; por ahora, la Unidad). En pesos: Mercado
-- Pago cobra en ARS.
create table if not exists preventa (
  modelo text primary key,
  activa boolean not null default false,
  precio_ars numeric(14, 2) not null default 0,
  precio_final_ars numeric(14, 2) not null default 0,
  sena_ars numeric(14, 2) not null default 0,
  cupos int not null default 0,
  entrega text not null default '',
  actualizado timestamptz not null default now()
);
insert into preventa (modelo) values ('unidad') on conflict do nothing;

-- Las señas de la preventa: pendiente al crear el pago, aprobada cuando Mercado
-- Pago lo confirma.
create table if not exists senas (
  id bigserial primary key,
  modelo text not null default 'unidad',
  nombre text not null,
  contacto text not null,
  email text not null default '',
  color text not null default '',
  monto_ars numeric(14, 2) not null,
  estado text not null default 'pendiente'
    check (estado in ('pendiente', 'aprobada', 'rechazada', 'devuelta')),
  mp_preferencia text not null default '',
  mp_pago text not null default '',
  unidad_id bigint references unidades on delete set null,
  ip text not null default '',
  creada timestamptz not null default now(),
  pagada timestamptz
);
create index if not exists senas_creada on senas (creada desc);
