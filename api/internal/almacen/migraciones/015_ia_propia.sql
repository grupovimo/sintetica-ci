-- La clave de IA propia de cada usuario (docs/nube.md, "Charla").

-- Cifrada como la de ElevenLabs (AES-GCM con una clave que sale de SECRETO,
-- con su derivación "ia") y nunca se devuelve. tope: charlas por día (0 = sin
-- tope: la factura es de él).
create table ia_cuentas (
  usuario_id uuid primary key references usuarios on delete cascade,
  proveedor text not null,
  clave_cifrada bytea not null,
  url text not null default '',
  modelo text not null default '',
  tope int not null default 0,
  creada timestamptz not null default now()
);
