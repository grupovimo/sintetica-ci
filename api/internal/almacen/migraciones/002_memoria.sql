-- La memoria del motor de vida (docs/motor-de-vida.md, "Memoria y relación"):
-- lo que el bicho recuerda, tal como lo manda (112 bytes con su CRC). La API la
-- trae cada tanto y la muestra como un diario; "deseada" es la que tiene que
-- recibir (una mudanza de placa), hasta que el bicho la confirma.
create table memorias (
  bicho_id text primary key references bichos on delete cascade,
  datos bytea,
  actualizada timestamptz,
  deseada bytea,
  deseada_desde text
);
