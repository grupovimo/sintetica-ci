-- Las versiones de config que cambia una automatización (la acción "modo")
-- llevan de autor "automatizacion".
alter table configs drop constraint configs_autor_check;
alter table configs add constraint configs_autor_check
  check (autor in ('web', 'app', 'bicho', 'api', 'automatizacion') or (autor like 'mcp:%' and length(autor) <= 60));
