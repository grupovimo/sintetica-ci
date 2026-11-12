-- El mail (docs/nube.md, "Mail"): Gmail u Outlook conectados con OAuth. Por
-- usuario y proveedor, el refresh token cifrado (como el de Spotify), la
-- dirección de la cuenta y hasta cuándo se miró. Nada de lo que dicen los
-- mails se guarda: los no leídos viven en la memoria de la API.
create table if not exists correo_cuentas (
  usuario_id uuid not null references usuarios on delete cascade,
  proveedor text not null check (proveedor in ('gmail', 'outlook')),
  refresh_cifrado bytea not null,
  direccion text not null default '',
  -- null = recién conectada: la primera vuelta solo pone la marca (sin avisar
  -- de los mails viejos).
  revisado timestamptz,
  creada timestamptz not null default now(),
  primary key (usuario_id, proveedor)
);
