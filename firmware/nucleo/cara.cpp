#include "cara.h"

namespace sintetica {

namespace {

constexpr int MAX_MUESTRAS = 160;
Punto g_muestras[MAX_MUESTRAS];

// Una transformación por eje: x' = ax·x + bx, y' = ay·y + by (normalizado),
// y después a píxeles con la vista.
struct Transformacion {
  float ax = 1, bx = 0, ay = 1, by = 0;
  Vista v;
  Punto aPixel(float x, float y) const {
    return {v.origenX + v.escala * (ax * x + bx), v.origenY + v.escala * (ay * y + by)};
  }
  Punto aPixel(Punto p) const { return aPixel(p.x, p.y); }
  float anchoPx(float w) const { return w * ax * v.escala; }
  float altoPx(float h) const { return h * ay * v.escala; }
};

// Corazón del manual: dos cúbicas (offsets en px de la pantalla de referencia
// respecto del centro del ojo).
const Punto CORAZON[7] = {{0, 21.4f},   {-31.2f, 0.6f}, {-23.4f, -28.0f}, {0, -11.1f},
                          {23.4f, -28.0f}, {31.2f, 0.6f},  {0, 21.4f}};

Punto cubica(Punto a, Punto b, Punto c, Punto d, float t) {
  float u = 1 - t;
  float k0 = u * u * u, k1 = 3 * u * u * t, k2 = 3 * u * t * t, k3 = t * t * t;
  return {k0 * a.x + k1 * b.x + k2 * c.x + k3 * d.x, k0 * a.y + k1 * b.y + k2 * c.y + k3 * d.y};
}

Punto cuadratica(Punto a, Punto b, Punto c, float t) {
  float u = 1 - t;
  return {u * u * a.x + 2 * u * t * b.x + t * t * c.x, u * u * a.y + 2 * u * t * b.y + t * t * c.y};
}

uint32_t colorDe(const Forma& f, const EstadoCara& e) {
  return (f.acento && e.hayColorOjos) ? e.colorOjos : f.color;
}

void dibujarForma(Lienzo& l, const Forma& f, const Transformacion& t, const EstadoCara& e) {
  const uint32_t color = colorDe(f, e);
  switch (f.tipo) {
    case TipoForma::Pildora: {
      float w = f.w, x = f.x, h = f.h;
      if (f.progreso) {
        float izq = f.x - f.w / 2;
        w = f.w * limitar(e.progreso, 0, 1);
        x = izq + w / 2;
      }
      if (f.audio >= 0) h = f.h * (0.35f + 0.65f * limitar(e.audio[f.audio], 0, 1));
      Punto c = t.aPixel(x, f.y);
      float wp = t.anchoPx(w), hp = t.altoPx(h);
      float r = f.r < 0 ? minf(wp, hp) / 2 : minf(f.r * t.ax * t.v.escala, minf(wp, hp) / 2);
      if (f.relleno) l.pildora(c.x, c.y, wp, hp, r, color);
      else l.pildoraContorno(c.x, c.y, wp, hp, r, f.trazo * t.v.escala, color);
      break;
    }
    case TipoForma::Circulo: {
      Punto c = t.aPixel(f.x, f.y);
      float wp = t.anchoPx(2 * f.r), hp = t.altoPx(2 * f.r);
      l.pildora(c.x, c.y, wp, hp, minf(wp, hp) / 2, color);
      break;
    }
    case TipoForma::Corte: {
      // Borde de arriba inclinado y medio círculo abajo.
      const float x0 = f.x - f.w / 2, x1 = f.x + f.w / 2;
      const float arriba = f.y - f.h / 2, abajo = f.y + f.h / 2;
      const float r = f.w / 2;
      int n = 0;
      g_muestras[n++] = t.aPixel(x0, arriba + f.izq);
      g_muestras[n++] = t.aPixel(x1, arriba + f.der);
      constexpr int PASOS = 24;
      for (int i = 0; i <= PASOS; i++) {
        float a = PI * i / PASOS;  // 0 = derecha, π/2 = abajo, π = izquierda
        g_muestras[n++] = t.aPixel(f.x + r * coseno(a), abajo - r + r * seno(a));
      }
      l.poligono(g_muestras, n, color);
      break;
    }
    case TipoForma::Corazon: {
      const float k = f.tamano / 160.0f;
      int n = 0;
      constexpr int PASOS = 20;
      for (int c = 0; c < 2; c++) {
        const Punto* p = &CORAZON[c * 3];
        for (int i = 0; i < PASOS; i++) {
          Punto q = cubica(p[0], p[1], p[2], p[3], static_cast<float>(i) / PASOS);
          g_muestras[n++] = t.aPixel(f.x + q.x * k, f.y + q.y * k);
        }
      }
      l.poligono(g_muestras, n, color);
      break;
    }
    case TipoForma::Arco: {
      constexpr int PASOS = 24;
      for (int i = 0; i <= PASOS; i++)
        g_muestras[i] = t.aPixel(cuadratica(f.puntos[0], f.puntos[1], f.puntos[2], static_cast<float>(i) / PASOS));
      l.trazo(g_muestras, PASOS + 1, f.trazo * t.v.escala, color);
      break;
    }
    case TipoForma::Espiral: {
      int n = 0;
      const float paso = f.giro / (MAX_MUESTRAS - 1);
      for (int i = 0; i < MAX_MUESTRAS; i++) {
        float a = i * paso;
        float r = f.r0 + f.paso * a;
        g_muestras[n++] = t.aPixel(f.x + r * coseno(a), f.y + r * seno(a));
      }
      l.trazo(g_muestras, n, f.trazo * t.v.escala, color);
      break;
    }
    case TipoForma::Poligono: {
      for (int i = 0; i < f.nPuntos; i++) g_muestras[i] = t.aPixel(f.puntos[i]);
      l.poligono(g_muestras, f.nPuntos, color);
      break;
    }
    case TipoForma::Franjas: {
      for (int i = 0; i < f.nPuntos; i++) {
        const Banda& b = f.bandas[i];
        // Las franjas son de la pantalla, no de los ojos: sin transformación.
        int y0 = static_cast<int>(t.v.origenY + b.y * t.v.escala + 0.5f);
        int y1 = static_cast<int>(t.v.origenY + (b.y + b.h) * t.v.escala + 0.5f);
        int dx = static_cast<int>(b.dx * t.v.escala + (b.dx >= 0 ? 0.5f : -0.5f));
        l.correrFilas(y0, y1, dx, paleta::PANTALLA);
      }
      break;
    }
  }
}

}  // namespace

void cajaDeForma(const Forma& f, float* x0, float* y0, float* x1, float* y1) {
  switch (f.tipo) {
    case TipoForma::Pildora:
    case TipoForma::Corte:
      *x0 = f.x - f.w / 2, *x1 = f.x + f.w / 2, *y0 = f.y - f.h / 2, *y1 = f.y + f.h / 2;
      return;
    case TipoForma::Circulo:
      *x0 = f.x - f.r, *x1 = f.x + f.r, *y0 = f.y - f.r, *y1 = f.y + f.r;
      return;
    case TipoForma::Corazon: {
      const float k = f.tamano / 160.0f;
      *x0 = f.x - 24 * k, *x1 = f.x + 24 * k, *y0 = f.y - 18 * k, *y1 = f.y + 21.4f * k;
      return;
    }
    case TipoForma::Espiral: {
      const float r = f.r0 + f.paso * f.giro;
      *x0 = f.x - r, *x1 = f.x + r, *y0 = f.y - r, *y1 = f.y + r;
      return;
    }
    case TipoForma::Arco:
    case TipoForma::Poligono: {
      *x0 = *x1 = f.puntos[0].x;
      *y0 = *y1 = f.puntos[0].y;
      for (int i = 1; i < f.nPuntos; i++) {
        *x0 = minf(*x0, f.puntos[i].x), *x1 = maxf(*x1, f.puntos[i].x);
        *y0 = minf(*y0, f.puntos[i].y), *y1 = maxf(*y1, f.puntos[i].y);
      }
      return;
    }
    case TipoForma::Franjas:
      *x0 = *x1 = *y0 = *y1 = 0;
      return;
  }
}

void dibujarExpresion(Lienzo& l, const Vista& v, const Expresion& e, const EstadoCara& estado) {
  // Agrupar las formas que no son fijas en ojo izquierdo y derecho, y calcular
  // la caja de cada grupo: el parpadeo y la respiración escalan cada ojo
  // alrededor de su propio centro.
  struct Grupo {
    float x0 = 9, y0 = 9, x1 = -9, y1 = -9;
    bool usado = false;
  } grupos[2];
  int grupoDe[MAX_FORMAS];
  for (int i = 0; i < e.nFormas; i++) {
    const Forma& f = e.formas[i];
    grupoDe[i] = -1;
    if (f.fijo || f.tipo == TipoForma::Franjas) continue;
    float x0, y0, x1, y1;
    cajaDeForma(f, &x0, &y0, &x1, &y1);
    int g = (x0 + x1) / 2 < 0 ? 0 : 1;
    grupoDe[i] = g;
    Grupo& gr = grupos[g];
    gr.usado = true;
    gr.x0 = minf(gr.x0, x0), gr.y0 = minf(gr.y0, y0);
    gr.x1 = maxf(gr.x1, x1), gr.y1 = maxf(gr.y1, y1);
  }

  Transformacion fija;
  fija.v = v;
  Transformacion ojo[2];
  for (int g = 0; g < 2; g++) {
    Transformacion& t = ojo[g];
    t.v = v;
    if (!grupos[g].usado) continue;
    const float cx = (grupos[g].x0 + grupos[g].x1) / 2;
    const float cy = (grupos[g].y0 + grupos[g].y1) / 2;
    const float alto = grupos[g].y1 - grupos[g].y0;
    float ky = mezclar(1.0f, alto > ALTO_CERRADO ? ALTO_CERRADO / alto : 1.0f, limitar(estado.cierre, 0, 1));
    float kx = 1.0f;
    const float resp = estado.respiracion * AMPLITUD_RESPIRACION;
    ky *= (1 + resp) * estado.tamano;
    kx *= (1 - resp * 0.5f) * estado.tamano;
    t.ax = kx;
    t.bx = cx - cx * kx + estado.miradaX;
    t.ay = ky;
    t.by = cy - cy * ky + estado.miradaY;
  }

  for (int i = 0; i < e.nFormas; i++) {
    const Forma& f = e.formas[i];
    dibujarForma(l, f, grupoDe[i] < 0 ? fija : ojo[grupoDe[i]], estado);
  }
}

}  // namespace sintetica
