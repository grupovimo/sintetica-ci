// La API, a través del proxy de la web (/api). Con la cookie de sesión y el
// encabezado X-Sintetica (sin él, la API rechaza los cambios: CSRF).

export class ErrorApi extends Error {
  constructor(
    public estado: number,
    mensaje: string,
    public detalle = '',
  ) {
    super(mensaje);
  }
}

type Opciones = {
  metodo?: string;
  cuerpo?: unknown;
  fetch?: typeof fetch;
  crudo?: boolean; // el cuerpo ya va como está (Blob, FormData, texto)
};

export async function api<T = unknown>(ruta: string, op: Opciones = {}): Promise<T> {
  const f = op.fetch ?? fetch;
  const encabezados: Record<string, string> = { 'X-Sintetica': '1' };
  let cuerpo: BodyInit | undefined;
  if (op.cuerpo !== undefined) {
    if (op.crudo) {
      cuerpo = op.cuerpo as BodyInit;
    } else {
      cuerpo = JSON.stringify(op.cuerpo);
      encabezados['Content-Type'] = 'application/json';
    }
  }
  let r: Response;
  try {
    r = await f('/api' + ruta, { method: op.metodo ?? (cuerpo ? 'POST' : 'GET'), headers: encabezados, body: cuerpo });
  } catch {
    throw new ErrorApi(0, 'No me pude conectar con el servidor.');
  }
  if (r.status === 204) return undefined as T;
  const texto = await r.text();
  let datos: any = null;
  try {
    datos = texto ? JSON.parse(texto) : null;
  } catch {
    datos = null;
  }
  if (!r.ok) throw new ErrorApi(r.status, datos?.error ?? `Error ${r.status}`, datos?.detalle ?? '');
  return datos as T;
}

export const conBinario = (ruta: string, metodo: string, datos: Uint8Array) =>
  api(ruta, { metodo, cuerpo: new Blob([datos], { type: 'application/octet-stream' }), crudo: true });
