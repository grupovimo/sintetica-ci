// Lo que la web valida del lado de ella (cortes de bytes y topes de listas)
// coincide con el núcleo: shared/formato/paridad/paridad.json.
import fs from 'node:fs';
import { describe, expect, test } from 'vitest';

import * as f from './formato';

const indice = JSON.parse(fs.readFileSync(new URL('../../../shared/formato/paridad/paridad.json', import.meta.url), 'utf8'));

describe('paridad con el núcleo', () => {
  test('los topes', () => {
    const l = indice.limites;
    expect(f.MAX_RECORDATORIOS).toBe(l.recordatorios);
    expect(f.MAX_TEXTO_RECORDATORIO).toBe(l.bytesRecordatorio);
    expect(f.MAX_FRASES).toBe(l.frasesPropias);
    expect(f.MAX_TEXTO).toBe(l.bytesFrase);
    expect(f.MAX_AVISOS).toBe(l.avisos);
    expect(f.MAX_DECIR_AVISO).toBe(l.bytesDecirAviso);
    expect(f.MAX_NOMBRE_AVISO).toBe(l.bytesAppAviso);
    expect(f.MAX_NOMBRE).toBe(l.bytesNombre);
    expect(f.MAX_PALABRA).toBe(l.bytesPalabra);
    expect(f.MAX_CIUDAD).toBe(l.bytesCiudad);
    expect(f.MAX_LISTA).toBe(l.lista);
    expect(f.MAX_REGLAS).toBe(l.reglasUsuario);
    expect(f.MAX_SPRITES).toBe(l.sprites);
    expect(f.MAX_HABITOS).toBe(l.habitos);
    expect(f.MAX_NOMBRE_HABITO).toBe(l.bytesHabito);
    expect(f.MAX_TEXTO_PRESENCIA).toBe(l.bytesPresencia);
  });

  test('los cortes de bytes no parten letras', () => {
    expect(indice.cortes.length).toBeGreaterThan(5);
    for (const c of indice.cortes) {
      expect(f.cortarBytes(c.texto, c.max)).toBe(c.resultado);
      expect(f.bytes(c.resultado)).toBe(c.bytes);
    }
  });
});
