#include "formato.h"
#include "motor_vida.h"
#include "prueba.h"

using namespace sintetica;

static Config g_c;

PRUEBA("motor: parpadea cada 3 a 7 s y dura 160 ms") {
  cargarBase(g_c);
  for (uint32_t semilla = 1; semilla <= 20; semilla++) {
    MotorVida m;
    m.iniciar(&g_c, semilla, 0);
    Contexto ctx;
    uint32_t ultimoInicio = 0;
    int parpadeos = 0;
    bool antes = false;
    for (uint32_t t = 0; t <= 60000; t += 10) {
      m.tick(t, ctx);
      bool ahora = m.parpadeando(t);
      if (ahora && !antes) {
        if (parpadeos > 0) {
          uint32_t intervalo = t - ultimoInicio;
          // Las sorpresas pueden meter un doble parpadeo (250 ms).
          VERIFICAR((intervalo >= 3000 && intervalo <= 7010) || intervalo == 250);
        }
        ultimoInicio = t;
        parpadeos++;
      }
      antes = ahora;
    }
    VERIFICAR(parpadeos >= 8 && parpadeos <= 21);
  }
}

PRUEBA("motor: el cierre del parpadeo sigue 0 → 0,5 → 1 → 0,5 → 0") {
  cargarBase(g_c);
  MotorVida m;
  m.iniciar(&g_c, 7, 0);
  Contexto ctx;
  uint32_t t0 = m.proximoParpadeo();
  m.tick(t0 - 1, ctx);
  VERIFICAR_CERCA(m.estado().cierre, 0, 1e-6);
  m.tick(t0, ctx);
  m.tick(t0 + 40, ctx);
  VERIFICAR_CERCA(m.estado().cierre, 0.5, 1e-4);
  m.tick(t0 + 80, ctx);
  VERIFICAR_CERCA(m.estado().cierre, 1.0, 1e-4);
  m.tick(t0 + 120, ctx);
  VERIFICAR_CERCA(m.estado().cierre, 0.5, 1e-4);
  m.tick(t0 + 160, ctx);
  VERIFICAR_CERCA(m.estado().cierre, 0, 1e-4);
}

PRUEBA("motor: cambio de expresión en 120 ms con un parpadeo en el medio") {
  cargarBase(g_c);
  MotorVida m;
  m.iniciar(&g_c, 3, 0);
  Contexto ctx;
  m.tick(100, ctx);
  const int mareado = g_c.indiceExpresion("mareado");
  m.reaccionar(mareado, 100);
  m.tick(100, ctx);
  VERIFICAR(m.expresionVisible() == g_c.indiceExpresion("neutral"));
  m.tick(130, ctx);
  VERIFICAR(m.estado().cierre > 0.4f);
  m.tick(160, ctx);
  VERIFICAR(m.expresionVisible() == mareado);
  VERIFICAR_CERCA(m.estado().cierre, 1.0, 1e-4);
  m.tick(220, ctx);
  VERIFICAR(m.expresionVisible() == mareado);
  VERIFICAR_CERCA(m.estado().cierre, 0, 1e-4);
  // A los 3 s vuelve a neutral.
  for (uint32_t t = 230; t < 3300; t += 10) m.tick(t, ctx);
  VERIFICAR(m.expresionVisible() == g_c.indiceExpresion("neutral"));
}

PRUEBA("motor: a los 10 min sin interacción se duerme; un toque lo despierta") {
  cargarBase(g_c);
  MotorVida m;
  m.iniciar(&g_c, 5, 0);
  Contexto ctx;
  for (uint32_t t = 0; t <= tiempos::SIESTA_MS + 1000; t += 50) m.tick(t, ctx);
  VERIFICAR(m.dormido());
  VERIFICAR(m.expresionVisible() == g_c.indiceExpresion("dormido"));
  uint32_t t = tiempos::SIESTA_MS + 1000;
  m.interaccion(t);
  for (uint32_t k = 0; k < 500; k += 10) m.tick(t + k, ctx);
  VERIFICAR(!m.dormido());
  VERIFICAR(m.expresionVisible() == g_c.indiceExpresion("neutral"));
}

PRUEBA("motor: duerme en su horario y no en otro") {
  cargarBase(g_c);
  g_c.suenoDesde = 23 * 60 + 30;
  g_c.suenoHasta = 8 * 60;
  MotorVida m;
  m.iniciar(&g_c, 9, 0);
  Contexto ctx;
  ctx.minutoDelDia = 2 * 60;
  for (uint32_t t = 0; t < 500; t += 10) m.tick(t, ctx);
  VERIFICAR(m.dormido());
  ctx.minutoDelDia = 15 * 60;
  for (uint32_t t = 500; t < 1000; t += 10) m.tick(t, ctx);
  VERIFICAR(!m.dormido());
}

PRUEBA("motor: la expresión de estado (batería baja) gana sobre la base") {
  cargarBase(g_c);
  MotorVida m;
  m.iniciar(&g_c, 11, 0);
  Contexto ctx;
  ctx.expresionEstado = g_c.indiceExpresion("bateria-baja");
  for (uint32_t t = 0; t < 400; t += 10) m.tick(t, ctx);
  VERIFICAR(m.expresionVisible() == ctx.expresionEstado);
}

PRUEBA("motor: una expresión fija no cambia sola") {
  cargarBase(g_c);
  MotorVida m;
  m.iniciar(&g_c, 13, 0);
  Contexto ctx;
  m.fijar(g_c.indiceExpresion("enamorado"));
  for (uint32_t t = 0; t < tiempos::SIESTA_MS + 5000; t += 100) m.tick(t, ctx);
  VERIFICAR(m.expresionVisible() == g_c.indiceExpresion("enamorado"));
}

PRUEBA("motor: mira a los costados y respira") {
  cargarBase(g_c);
  MotorVida m;
  m.iniciar(&g_c, 17, 0);
  Contexto ctx;
  float maxMirada = 0, minResp = 1, maxResp = -1;
  for (uint32_t t = 0; t < 30000; t += 20) {
    m.tick(t, ctx);
    maxMirada = maxMirada > m.estado().miradaX ? maxMirada : m.estado().miradaX;
    float r = m.estado().respiracion;
    minResp = r < minResp ? r : minResp;
    maxResp = r > maxResp ? r : maxResp;
  }
  VERIFICAR(maxMirada > 0.01f && maxMirada <= 0.071f);
  VERIFICAR(minResp < -0.95f && maxResp > 0.95f);
}
