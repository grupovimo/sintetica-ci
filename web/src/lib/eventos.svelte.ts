// Los eventos en vivo de la API (SSE): estado, conexión, sincronización, OTA,
// automatizaciones, Telegram, el MCP, los toques de amigos y los avisos entrantes.
// Se reconecta solo.
import type { Evento } from './tipos';

type Oyente = (e: Evento) => void;

const oyentes = new Set<Oyente>();
let fuente: EventSource | null = null;
export const conexionEnVivo = $state({ conectado: false });

const TIPOS = ['estado', 'conexion', 'sincro', 'config', 'imagenes', 'sprites', 'ota', 'marco', 'bicho', 'automatizacion', 'telegram', 'mcp', 'toque', 'entrante', 'agenda', 'musica', 'charla'];

function abrir() {
  if (fuente || typeof EventSource === 'undefined') return;
  fuente = new EventSource('/api/v1/eventos');
  fuente.onopen = () => (conexionEnVivo.conectado = true);
  fuente.onerror = () => (conexionEnVivo.conectado = false);
  for (const t of TIPOS) {
    fuente.addEventListener(t, (m) => {
      const e = JSON.parse((m as MessageEvent).data) as Evento;
      for (const o of oyentes) o(e);
    });
  }
}

// Escucha los eventos mientras el componente esté montado (devuelve cómo dejar de escuchar).
export function escuchar(o: Oyente): () => void {
  oyentes.add(o);
  abrir();
  return () => {
    oyentes.delete(o);
    if (oyentes.size === 0) {
      fuente?.close();
      fuente = null;
      conexionEnVivo.conectado = false;
    }
  };
}
