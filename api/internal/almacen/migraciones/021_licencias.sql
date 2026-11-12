-- Las licencias del escritorio (pago único, una por cuenta): pendiente al
-- crear el pago, pagada cuando Mercado Pago lo confirma. El precio es la fila
-- 'escritorio' de la preventa (precio_ars; activa = se vende).
create table if not exists licencias (
  id bigserial primary key,
  usuario_id uuid not null references usuarios on delete cascade,
  producto text not null default 'escritorio',
  monto numeric(14, 2) not null,
  moneda text not null default 'ARS',
  estado text not null default 'pendiente'
    check (estado in ('pendiente', 'pagada', 'rechazada', 'devuelta')),
  mp_preferencia text not null default '',
  mp_pago text not null default '',
  creada timestamptz not null default now(),
  pagada timestamptz
);
create index if not exists licencias_usuario on licencias (usuario_id, producto);

insert into preventa (modelo) values ('escritorio') on conflict do nothing;
