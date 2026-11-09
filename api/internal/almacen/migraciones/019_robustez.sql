-- Lo que no puede vivir solo en memoria (docs/nube.md, "Lo que sobrevive a
-- un reinicio"): los topes que importan (toques entre amigos, disparos de las
-- automatizaciones, códigos de amigo probados), lo ya avisado una vez (los
-- pagos de Mercado Pago, las entregas de GitHub) y el último toque que le
-- llegó a cada robot, para devolverlo. Se limpia sola cada hora (Limpiar).

-- Un renglón por vez que se dejó pasar algo: clave + cuándo. `vence` es
-- cuándo deja de contar (en + la ventana del tope).
create table if not exists topes (
  clave text not null,
  en timestamptz not null default now(),
  vence timestamptz not null
);
create index if not exists topes_clave on topes (clave, en desc);
create index if not exists topes_vence on topes (vence);

-- Lo que se hace una sola vez (hasta `vence`): "mp:<usuario>:<pago>", etc.
create table if not exists vistos (
  clave text primary key,
  vence timestamptz not null
);
create index if not exists vistos_vence on vistos (vence);

-- El último toque que le llegó a un robot (vale 10 min para devolverlo).
create table if not exists toques_recibidos (
  bicho_id text primary key references bichos on delete cascade,
  de_usuario uuid not null references usuarios on delete cascade,
  desde text not null default '',       -- el robot desde el que se mandó ('' = desde la web)
  tipo text not null,
  expresion text not null default '',
  en timestamptz not null default now()
);
create index if not exists toques_recibidos_en on toques_recibidos (en);

-- Sumar un mail a una cuenta (la de Telegram no tiene): un código al mail,
-- uno pendiente por cuenta.
create table if not exists email_pendientes (
  usuario_id uuid primary key references usuarios on delete cascade,
  email text not null check (email = lower(email)),
  codigo_hash bytea not null,
  intentos int not null default 0,
  vence timestamptz not null,
  creado timestamptz not null default now()
);
