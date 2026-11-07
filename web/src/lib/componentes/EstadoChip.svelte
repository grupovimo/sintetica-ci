<script lang="ts">
  import type { Bicho } from '$lib/tipos';
  let { bicho }: { bicho: Bicho } = $props();

  const texto = $derived.by(() => {
    if (!bicho.conectado) return { clase: 'apagado', texto: 'desconectado' };
    if (bicho.estado?.actualizando) return { clase: 'aviso', texto: 'actualizando' };
    const s = bicho.sincro;
    if (s?.config === 'error' || s?.imagenes === 'error') return { clase: 'error', texto: 'no se sincroniza' };
    if (['enviando', 'trayendo'].includes(s?.config) || ['enviando', 'trayendo'].includes(s?.imagenes))
      return { clase: 'aviso', texto: 'sincronizando' };
    if (bicho.estado?.dormido) return { clase: 'ok', texto: 'durmiendo' };
    return { clase: 'ok', texto: 'conectado' };
  });
</script>

<span class="chip {texto.clase}" title={bicho.sincro?.detalle ?? ''}>{texto.texto}</span>
