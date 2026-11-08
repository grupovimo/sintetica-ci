// Miniaturas de las caras del motor de vida, dibujadas una vez por el núcleo
// (las de fábrica y las del usuario), para elegir caras viéndolas.
import { nucleoEditor, pintar } from '$lib/nucleo';

export async function miniaturasDeCaras(
  config: unknown,
  pantalla: { ancho: number; alto: number; redonda: boolean },
): Promise<Record<string, string>> {
  const s = await nucleoEditor();
  s.editor.iniciar(pantalla.ancho * 2, pantalla.alto * 2, pantalla.redonda);
  s.editor.config(config);
  const c = document.createElement('canvas');
  const m: Record<string, string> = {};
  for (const n of s.editor.caras() as string[]) {
    const i = s.editor.indiceCara(n);
    if (i < 0) continue;
    const { rgba, ancho, alto } = s.editor.rostro(s.editor.parametrosCara(i), 0.4, 0, -1);
    pintar(c, rgba, ancho, alto);
    m[n] = c.toDataURL('image/jpeg', 0.8);
  }
  return m;
}
