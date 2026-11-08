-- El servidor MCP (docs/nube.md, "MCP"): las IAs de cada usuario (Claude,
-- ChatGPT) manejan sus robots. OAuth 2.1 con registro dinámico de clientes
-- (RFC 7591), PKCE S256 y tokens opacos (solo se guardan sus hashes), más los
-- tokens personales que el usuario crea desde la web (Claude Code, pruebas).

-- Los clientes que se registraron solos (POST /oauth/registrar).
create table mcp_clientes (
  id text primary key,                       -- el client_id
  nombre text not null,
  redirecciones text[] not null,
  metodo text not null default 'none',       -- token_endpoint_auth_method
  secreto_hash bytea,                        -- solo si no es público
  uri text not null default '',
  ip text not null default '',
  creado timestamptz not null default now()
);

-- Una IA conectada: lo que el usuario permitió a un cliente. Revocarla corta
-- todos sus tokens.
create table mcp_concesiones (
  id uuid primary key default gen_random_uuid(),
  usuario_id uuid not null references usuarios on delete cascade,
  cliente_id text not null references mcp_clientes on delete cascade,
  alcance text not null default 'robots',
  creada timestamptz not null default now(),
  usada timestamptz,
  revocada timestamptz
);
create index mcp_concesiones_usuario on mcp_concesiones (usuario_id, creada desc);

-- Los códigos de autorización: un solo uso, 5 minutos, con su desafío PKCE.
create table mcp_codigos (
  codigo_hash bytea primary key,
  usuario_id uuid not null references usuarios on delete cascade,
  cliente_id text not null references mcp_clientes on delete cascade,
  redireccion text not null,
  desafio text not null,                     -- code_challenge (S256)
  alcance text not null default 'robots',
  recurso text not null default '',
  vence timestamptz not null,
  usado timestamptz,
  concesion_id uuid references mcp_concesiones on delete cascade
);

-- Los tokens: de acceso (1 h), de refresco (60 días, rotan) y personales.
create table mcp_tokens (
  token_hash bytea primary key,
  id uuid not null unique default gen_random_uuid(),
  tipo text not null check (tipo in ('acceso', 'refresco', 'personal')),
  usuario_id uuid not null references usuarios on delete cascade,
  concesion_id uuid references mcp_concesiones on delete cascade,
  nombre text not null default '',           -- el de los personales
  vence timestamptz,                         -- null = no vence (personal)
  creado timestamptz not null default now(),
  usado timestamptz,
  rotado timestamptz,                        -- un refresco ya canjeado
  revocado timestamptz
);
create index mcp_tokens_usuario on mcp_tokens (usuario_id, tipo);
create index mcp_tokens_concesion on mcp_tokens (concesion_id);

-- Las versiones de config que cambia una IA llevan de autor "mcp:<cliente>".
alter table configs drop constraint configs_autor_check;
alter table configs add constraint configs_autor_check
  check (autor in ('web', 'app', 'bicho', 'api') or (autor like 'mcp:%' and length(autor) <= 60));
