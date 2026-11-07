#include "motor_vida.h"

namespace sintetica {

void MotorVida::iniciar(const Config* config, uint32_t semilla, uint32_t ahora) {
  config_ = config;
  azar_.sembrar(semilla);
  estado_ = EstadoCara();
  estado_.tamano = config->tamano;
  estado_.hayColorOjos = config->hayColorOjos;
  estado_.colorOjos = config->colorOjos;
  int neutral = indice("neutral");
  actual_ = neutral >= 0 ? neutral : 0;
  destino_ = -1;
  reaccion_ = -1;
  ultimaInteraccion_ = ahora;
  ultimoTick_ = ahora;
  inicio_ = ahora;
  dormido_ = false;
  inicioParpadeo_ = ahora - 10 * tiempos::PARPADEO_MS;
  programarParpadeo(ahora);
  programarMirada(ahora);
  programarSorpresa(ahora);
}

void MotorVida::programarParpadeo(uint32_t ahora) {
  proximoParpadeo_ = ahora + azar_.entre(tiempos::PARPADEO_MIN_MS, tiempos::PARPADEO_MAX_MS);
}

void MotorVida::programarMirada(uint32_t ahora) {
  // Más energía, más seguido mira para otro lado: de 1,2 a 5 s.
  const float e = config_->energia;
  const uint32_t lo = static_cast<uint32_t>(2500 - 1300 * e), hi = static_cast<uint32_t>(6000 - 2500 * e);
  proximaMirada_ = ahora + azar_.entre(lo, hi);
}

void MotorVida::programarSorpresa(uint32_t ahora) {
  const float e = config_->energia;
  const uint32_t lo = static_cast<uint32_t>(90000 - 60000 * e), hi = static_cast<uint32_t>(240000 - 150000 * e);
  proximaSorpresa_ = ahora + azar_.entre(lo, hi);
}

void MotorVida::reaccionar(int expresion, uint32_t ahora, uint32_t duracion) {
  if (expresion < 0 || expresion >= config_->nExpresiones) return;
  reaccion_ = expresion;
  reaccionHasta_ = ahora + duracion;
}

void MotorVida::interaccion(uint32_t ahora) {
  ultimaInteraccion_ = ahora;
  // La timidez frena cuánto lo alegra cada toque.
  humor_ = limitar(humor_ + 0.25f * (1.0f - 0.6f * config_->timidez), -1, 1);
  if (dormido_) despiertoHasta_ = ahora + tiempos::DESPIERTO_POR_TOQUE_MS;
}

void MotorVida::fijar(int expresion) {
  fija_ = (expresion >= 0 && expresion < config_->nExpresiones) ? expresion : -1;
}

bool MotorVida::enHorarioDeSueno(int m) const {
  const int d = config_->suenoDesde, h = config_->suenoHasta;
  if (m < 0 || d < 0 || h < 0 || d == h) return false;
  return d < h ? (m >= d && m < h) : (m >= d || m < h);
}

int MotorVida::elegirObjetivo(uint32_t ahora, const Contexto& ctx) {
  if (fija_ >= 0) return fija_;
  if (reaccion_ >= 0) {
    if (static_cast<int32_t>(ahora - reaccionHasta_) < 0) return reaccion_;
    reaccion_ = -1;
  }
  if (ctx.expresionEstado >= 0) return ctx.expresionEstado;

  const uint32_t quieto = ahora - ultimaInteraccion_;
  const bool despiertoPorToque = static_cast<int32_t>(ahora - despiertoHasta_) < 0;
  dormido_ = !despiertoPorToque && (quieto >= tiempos::SIESTA_MS || enHorarioDeSueno(ctx.minutoDelDia));
  if (dormido_) {
    int d = indice("dormido");
    if (d >= 0) return d;
  }
  if (quieto >= tiempos::ABANDONO_MS) {
    int t = indice("triste");
    if (t >= 0) return t;
  }
  int n = indice("neutral");
  return n >= 0 ? n : 0;
}

void MotorVida::tick(uint32_t ahora, const Contexto& ctx) {
  const uint32_t dt = ahora - ultimoTick_;
  ultimoTick_ = ahora;

  // Humor: vuelve despacio a un punto que depende de cuánto lo usan.
  const float quietoHoras = (ahora - ultimaInteraccion_) / 3600000.0f;
  const float reposo = quietoHoras < 1 ? 0.2f : (quietoHoras < 6 ? 0.0f : -0.4f);
  humor_ += (reposo - humor_) * limitar(dt / 300000.0f, 0, 1);

  // Sorpresas: un guiño, un glitch o un doble parpadeo cada tanto.
  if (static_cast<int32_t>(ahora - proximaSorpresa_) >= 0) {
    programarSorpresa(ahora);
    if (!dormido_ && fija_ < 0 && reaccion_ < 0 && ctx.expresionEstado < 0) {
      uint32_t cual = azar_.entre(0, 2);
      if (cual == 0) reaccionar(indice("guino"), ahora, 700);
      else if (cual == 1) reaccionar(indice("glitch"), ahora, 250);
      else dobleParpadeo_ = true, proximoParpadeo_ = ahora;
    }
  }

  // Transiciones: cerrar 60 ms, cambiar, abrir 60 ms.
  const int objetivo = elegirObjetivo(ahora, ctx);
  if (destino_ < 0 && objetivo != actual_) {
    destino_ = objetivo;
    inicioTransicion_ = ahora;
  }
  float cierreTransicion = 0;
  if (destino_ >= 0) {
    const uint32_t t = ahora - inicioTransicion_;
    const uint32_t mitad = tiempos::TRANSICION_MS / 2;
    if (t < mitad) {
      cierreTransicion = suave(static_cast<float>(t) / mitad);
    } else {
      actual_ = destino_;
      if (t < tiempos::TRANSICION_MS) {
        cierreTransicion = suave(static_cast<float>(tiempos::TRANSICION_MS - t) / mitad);
      } else {
        destino_ = -1;
        // Si el objetivo cambió durante la transición, se encadena otra.
        if (objetivo != actual_) destino_ = objetivo, inicioTransicion_ = ahora;
      }
    }
  }

  // Parpadeo: 72 → 40 → 8 → 40 → 72 px en 160 ms (ease-in-out de ida y vuelta).
  const Expresion& e = config_->expresiones[actual_];
  float cierreParpadeo = 0;
  if (destino_ < 0 && e.parpadea && static_cast<int32_t>(ahora - proximoParpadeo_) >= 0) {
    inicioParpadeo_ = ahora;
    if (dobleParpadeo_) {
      dobleParpadeo_ = false;
      proximoParpadeo_ = ahora + tiempos::PARPADEO_MS + 90;
    } else {
      programarParpadeo(ahora);
    }
  }
  const uint32_t tp = ahora - inicioParpadeo_;
  if (tp < tiempos::PARPADEO_MS) {
    const float p = static_cast<float>(tp) / tiempos::PARPADEO_MS;
    cierreParpadeo = p < 0.5f ? suave(p * 2) : suave((1 - p) * 2);
  }
  if (!e.parpadea && destino_ < 0 && static_cast<int32_t>(ahora - proximoParpadeo_) >= 0) programarParpadeo(ahora);
  estado_.cierre = maxf(cierreTransicion, cierreParpadeo);

  // Mirada: elige un punto, va en 200 ms y se queda ahí.
  if (static_cast<int32_t>(ahora - proximaMirada_) >= 0) {
    programarMirada(ahora);
    miradaDesdeX_ = estado_.miradaX;
    miradaDesdeY_ = estado_.miradaY;
    inicioMirada_ = ahora;
    if (dormido_ || azar_.real() < 0.35f) {
      miradaHaciaX_ = 0, miradaHaciaY_ = 0;
    } else {
      miradaHaciaX_ = (azar_.real() * 2 - 1) * 0.07f;
      miradaHaciaY_ = (azar_.real() * 2 - 1) * 0.025f;
      // Tímido: tiende a mirar para abajo.
      miradaHaciaY_ += config_->timidez * 0.02f;
    }
  }
  const float tm = limitar(static_cast<float>(ahora - inicioMirada_) / tiempos::MIRADA_MS, 0, 1);
  estado_.miradaX = mezclar(miradaDesdeX_, miradaHaciaX_, suave(tm));
  estado_.miradaY = mezclar(miradaDesdeY_, miradaHaciaY_, suave(tm));

  // Respiración: los ojos "respiran" apenas; dormido, más lento y más hondo.
  const float periodo = dormido_ ? tiempos::RESPIRACION_MS * 1.6f : static_cast<float>(tiempos::RESPIRACION_MS);
  const float fase = static_cast<float>((ahora - inicio_) % static_cast<uint32_t>(periodo)) / periodo;
  estado_.respiracion = seno(fase * DOS_PI) * (dormido_ ? 1.6f : 1.0f);

  // Barras de audio de "escuchando": se mueven solas hasta que haya micrófono.
  for (int i = 0; i < 8; i++) {
    const float f = (ahora - inicio_) / 1000.0f;
    estado_.audio[i] = 0.55f + 0.45f * seno(f * (7.0f + i * 1.7f) + i * 1.3f);
  }
}

}  // namespace sintetica
