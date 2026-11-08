-- Amigos y toques (v1.9, docs/nube.md, "Amigos y toques"): las invitaciones
-- de un solo uso y las amistades (una fila por par). Cada lado elige qué
-- robots suyos reciben los toques del otro (null = todos).

-- El código de 8 letras: solo su hash, 48 h, un solo uso.
create table amigos_invitaciones (
  codigo_hash bytea primary key,
  usuario_id uuid not null references usuarios on delete cascade,
  vence timestamptz not null,
  usado timestamptz,
  usado_por uuid references usuarios on delete set null,
  creado timestamptz not null default now()
);
create index amigos_invitaciones_usuario on amigos_invitaciones (usuario_id, creado desc);

-- usuario_a < usuario_b: un solo par por amistad.
create table amistades (
  id bigserial primary key,
  usuario_a uuid not null references usuarios on delete cascade,
  usuario_b uuid not null references usuarios on delete cascade,
  robots_a text[], -- los robots de a que reciben los toques de b (null = todos)
  robots_b text[],
  creada timestamptz not null default now(),
  check (usuario_a < usuario_b),
  unique (usuario_a, usuario_b)
);
create index amistades_b on amistades (usuario_b);
