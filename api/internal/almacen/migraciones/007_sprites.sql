-- Las caras de imagen (docs/motor-de-vida.md, "Caras de imagen"): los sprites
-- viven en la misma tabla que las imágenes, en los espacios 8 a 19 (el sprite
-- n es el espacio 8 + n, como en la característica Imagen del protocolo).
alter table imagenes drop constraint if exists imagenes_espacio_check;
alter table imagenes add constraint imagenes_espacio_check check (espacio between 0 and 19);
