// Las caras base y las animaciones de ejemplo (shared/formato, por la API).
import { api } from './api';
import type { Animacion, Expresion } from './formato';

type Formato = {
  caras: { nombre: string; documento: Expresion }[];
  animaciones: { nombre: string; documento: Animacion }[];
};

let cache: Promise<Formato> | null = null;
export const formatoBase = () => (cache ??= api<Formato>('/v1/formato'));
