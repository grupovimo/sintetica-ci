-- Qué pieza se reserva (unidad o broche); las de antes eran todas unidades.
alter table reservas add column pieza text not null default 'unidad';
