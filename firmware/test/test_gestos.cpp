#include "gestos.h"
#include "prueba.h"

using namespace sintetica;

static int contar(Gestos& g, Evento buscado) {
  Evento e;
  int n = 0;
  while (g.sacar(&e)) n += e == buscado;
  return n;
}

PRUEBA("gestos: toque simple, doble toque y caricia") {
  Gestos g;
  g.tacto(true, 1000);
  g.tacto(false, 1100);
  g.tick(1200);
  VERIFICAR(contar(g, Evento::Tacto) == 0);  // espera por si viene otro
  g.tick(1500);
  VERIFICAR(contar(g, Evento::Tacto) == 1);

  g.tacto(true, 2000);
  g.tacto(false, 2080);
  g.tacto(true, 2200);
  g.tacto(false, 2280);
  g.tick(3000);
  Evento e;
  VERIFICAR(g.sacar(&e) && e == Evento::DobleToque);
  VERIFICAR(!g.sacar(&e));

  g.tacto(true, 4000);
  g.tick(4300);
  g.tick(4700);
  VERIFICAR(g.sacar(&e) && e == Evento::Caricia);
  g.tacto(false, 5000);
  g.tick(6000);
  VERIFICAR(!g.sacar(&e));
}

PRUEBA("gestos: sacudida") {
  Gestos g;
  uint32_t t = 0;
  for (int i = 0; i < 20; i++, t += 20) g.imu(0, 0, 1, t);
  for (int i = 0; i < 8; i++, t += 100) g.imu(i % 2 ? 2.4f : -1.5f, 0, 1, t);
  VERIFICAR(contar(g, Evento::SacudidaSuave) == 1);
  // Con un pico de más de 2 g, es fuerte.
  Gestos f;
  t = 0;
  for (int i = 0; i < 20; i++, t += 20) f.imu(0, 0, 1, t);
  for (int i = 0; i < 8; i++, t += 100) f.imu(i % 2 ? 3.5f : -2.0f, 0, 1, t);
  VERIFICAR(contar(f, Evento::SacudidaFuerte) == 1);
}

PRUEBA("gestos: muchos toques, caída libre e inclinado") {
  Gestos g;
  uint32_t t = 0;
  for (int i = 0; i < 4; i++, t += 300) {
    g.tacto(true, t);
    g.tacto(false, t + 60);
  }
  g.tick(t + 1000);
  VERIFICAR(contar(g, Evento::MuchosToques) == 1);
  Gestos c;
  t = 0;
  for (; t < 500; t += 20) c.imu(0, 0, 1, t);
  for (int i = 0; i < 10; i++, t += 20) c.imu(0, 0, 0.1f, t);
  VERIFICAR(contar(c, Evento::Caida) == 1);
  Gestos n;
  t = 0;
  for (; t < 500; t += 20) n.imu(0.6f, 0, 0.8f, t);
  float x = 0, y = 0;
  VERIFICAR(n.inclinacion(&x, &y) && x > 0.5f);
  VERIFICAR(contar(n, Evento::Inclinado) == 1);
}

PRUEBA("gestos: levantado después de estar quieto") {
  Gestos g;
  uint32_t t = 0;
  for (; t < 2500; t += 20) g.imu(0, 0, 1, t);
  g.imu(0, 0, 1.3f, t);
  VERIFICAR(contar(g, Evento::Levantado) == 1);
}

PRUEBA("gestos: boca abajo un segundo") {
  Gestos g;
  uint32_t t = 0;
  for (; t < 900; t += 20) g.imu(0, 0, -1, t);
  VERIFICAR(contar(g, Evento::BocaAbajo) == 0);
  for (; t < 1500; t += 20) g.imu(0, 0, -1, t);
  VERIFICAR(contar(g, Evento::BocaAbajo) == 1);
  for (; t < 3000; t += 20) g.imu(0, 0, -1, t);
  VERIFICAR(contar(g, Evento::BocaAbajo) == 0);
}
