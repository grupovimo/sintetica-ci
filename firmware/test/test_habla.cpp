// Las frases (habla), el clima y el globo, sueltos: sin el sistema.
#include <cstring>
#include <string>

#include "clima.h"
#include "fuentes.h"
#include "globo.h"
#include "habla.h"
#include "prueba.h"
#include "texto.h"
#include "vida_base.h"

using namespace sintetica;

namespace {
bool tieneGlifo(const Fuente& f, uint32_t cp) {
  for (int i = 0; i < f.cantidad; i++)
    if (f.glifos[i].codigo == cp) return true;
  return false;
}

std::string completar(const char* p, const char* dueno, const char* robot = "Lucho", size_t cap = LARGO_FRASE) {
  char s[128];
  completarFrase(p, dueno, robot, s, cap);
  return s;
}
}  // namespace

PRUEBA("habla: las situaciones del núcleo y las de frases.json son las mismas") {
  VERIFICAR(CANTIDAD_SITUACIONES_BASE == N_SITUACIONES);
  for (int i = 0; i < N_SITUACIONES; i++) {
    bool esta = false;
    for (int k = 0; k < CANTIDAD_SITUACIONES_BASE; k++) esta = esta || std::strcmp(SITUACIONES_BASE[k].nombre, nombreSituacion(static_cast<Situacion>(i))) == 0;
    if (!esta) std::printf("    falta en frases.json: %s\n", nombreSituacion(static_cast<Situacion>(i)));
    VERIFICAR(esta);
    VERIFICAR(SITUACIONES_BASE[i].cantidad > 0);
  }
}

PRUEBA("habla: cada frase de fábrica entra en el globo y tiene sus letras en la fuente") {
  const char* presets[] = {"", "equilibrado", "timido", "picaro", "dormilon", "carinoso", "curioso", "grunon", "electrico"};
  for (int i = 0; i < CANTIDAD_FRASES_BASE; i++) {
    const FraseBase& f = FRASES_BASE[i];
    bool preset = false;
    for (const char* p : presets) preset = preset || std::strcmp(p, f.preset) == 0;
    VERIFICAR(preset);
    VERIFICAR(std::strlen(f.texto) >= 1 && std::strlen(f.texto) < LARGO_FRASE);
    // Con el nombre más largo posible (24 bytes) sigue entrando: se corta sin partir una letra.
    const std::string s = completar(f.texto, "Maximiliano Bartolomeo X", "Robotito de la Casa Azul");
    VERIFICAR(s.size() < LARGO_FRASE);
    const char* p = s.c_str();
    while (*p) {
      const uint32_t cp = siguienteCodigo(&p);
      if (!tieneGlifo(fuentes::VOZ_20, cp)) {
        std::printf("    sin glifo U+%04X en %s\n", cp, f.texto);
        VERIFICAR(false);
      }
    }
  }
}

PRUEBA("habla: {dueno} y {robot}; sin dueño se lleva la coma") {
  VERIFICAR(completar("¡Buen día, {dueno}!", "Nahuel") == "¡Buen día, Nahuel!");
  VERIFICAR(completar("¡Buen día, {dueno}!", "") == "¡Buen día!");
  VERIFICAR(completar("Soy {robot}.", "") == "Soy Lucho.");
  VERIFICAR(completar("{dueno}, ¿me cargás?", "") == "¿me cargás?");
  // Se corta sin partir una letra: "día" con la í en el corte.
  const std::string c = completar("Buen día", "", "", 7);
  VERIFICAR(c == "Buen d");
}

PRUEBA("habla: elige de su preset y de todos, y las propias ganan") {
  Habla h;
  Azar a;
  a.sembrar(3);
  char s[LARGO_FRASE];
  int deGrunon = 0, total = 200;
  for (int i = 0; i < total; i++) {
    VERIFICAR(elegirFrase(h, Situacion::Despertar, "grunon", "Nahuel", "Lucho", a, s, sizeof s));
    if (std::string(s) == "Mmm. Buen día." || std::string(s) == "Ya va, ya va." || std::string(s) == "¿Tan temprano?") deGrunon++;
    // Nunca una de otro preset.
    VERIFICAR(std::string(s) != "Cinco minutitos más...");
  }
  VERIFICAR(deGrunon > 40 && deGrunon < 160);
  h.nPropias = 1;
  h.propias[0].situacion = Situacion::Despertar;
  std::strcpy(h.propias[0].texto, "¡Arriba, {dueno}!");
  for (int i = 0; i < 10; i++) {
    VERIFICAR(elegirFrase(h, Situacion::Despertar, "grunon", "Nahuel", "Lucho", a, s, sizeof s));
    VERIFICAR(std::string(s) == "¡Arriba, Nahuel!");
  }
  // Sin dueño no dice las que empiezan con {dueno}.
  std::strcpy(h.propias[0].texto, "{dueno}!");
  VERIFICAR(elegirFrase(h, Situacion::Despertar, "grunon", "", "Lucho", a, s, sizeof s));
  VERIFICAR(std::string(s) != "!");
}

PRUEBA("habla: el hablador no es pesado (enfriamiento, separación y tope por día)") {
  Hablador h;
  Azar a;
  a.sembrar(9);
  // Una caricia por minuto durante un día con la frecuencia normal: pocas frases.
  int dichas = 0;
  for (uint32_t s = 0; s < 86400; s += 60) dichas += h.quiere(Situacion::Caricia, s, 1, 50, a);
  VERIFICAR(dichas >= 1 && dichas <= 24);  // enfriamiento de 1 h
  // Con todas las situaciones a la vez, nunca más del tope del día.
  Hablador g;
  int total = 0;
  for (uint32_t s = 0; s < 86400; s += 30)
    for (int k = 0; k < N_SITUACIONES; k++) {
      const Situacion si = static_cast<Situacion>(k);
      if (si == Situacion::CumpleDueno || si == Situacion::CumpleRobot || si == Situacion::Despertador || si == Situacion::Posponer) continue;
      total += g.quiere(si, s, 1, 50, a);
    }
  VERIFICAR(total <= FRASES_POR_DIA);
  VERIFICAR(total >= 3);
  // Frecuencia 0: nunca habla solo, pero el cumple y el despertador sí.
  Hablador c;
  VERIFICAR(!c.quiere(Situacion::Volviste, 0, 1, 0, a));
  VERIFICAR(c.quiere(Situacion::CumpleDueno, 10, 1, 0, a));
  VERIFICAR(c.quiere(Situacion::Despertador, 20, 1, 0, a));
  // Al otro día, el tope vuelve a cero.
  Hablador d;
  int primero = 0, segundo = 0;
  for (uint32_t s = 0; s < 86400; s += 30) primero += d.quiere(Situacion::Aburrido, s, 5, 100, a) ? 1 : 0;
  for (uint32_t s = 86400; s < 2 * 86400; s += 30) segundo += d.quiere(Situacion::Aburrido, s, 6, 100, a) ? 1 : 0;
  VERIFICAR(primero > 0 && segundo > 0);
}

PRUEBA("habla: el preset más parecido") {
  Personalidad p;
  VERIFICAR(presetPersonalidad("grunon", &p));
  VERIFICAR(std::string(presetCercano(p)) == "grunon");
  p.malGenio = 0.85f, p.sociabilidad = 0.3f;
  VERIFICAR(std::string(presetCercano(p)) == "grunon");
  VERIFICAR(std::string(presetCercano(Personalidad())) == "equilibrado");
}

PRUEBA("habla: duración del globo según el largo") {
  VERIFICAR(duracionGlobo("Hola") == 3000);
  VERIFICAR(duracionGlobo("Una frase de unos cuarenta bytes, más o") > 4500);
  VERIFICAR(duracionGlobo("012345678901234567890123456789012345678901234567890123456789") <= 10000);
}

PRUEBA("clima: el marco, el cielo y el texto") {
  const uint8_t m[] = {0xB8, 0x00, 3, 22, 12, 1, 6, 'C', 0xC3, 0xB3, 'r', 'd', 'o'};  // 18,4°, nublado, "Córdo"
  DatosClima c;
  VERIFICAR(leerClima(m, sizeof m, &c));
  VERIFICAR(c.hay && c.temperatura() == 18 && c.maxima == 22 && c.minima == 12 && c.dia);
  VERIFICAR(std::string(c.ciudad) == "Córdo");
  char t[40];
  textoClima(c, t, sizeof t);
  VERIFICAR(std::string(t) == "18° y nublado");
  VERIFICAR(!leerClima(m, sizeof m - 1, &c));  // el largo de la ciudad no cierra
  VERIFICAR(cieloDe(0) == Cielo::Despejado && cieloDe(2) == Cielo::Parcial && cieloDe(45) == Cielo::Niebla);
  VERIFICAR(cieloDe(63) == Cielo::Lluvia && cieloDe(81) == Cielo::Lluvia && cieloDe(53) == Cielo::Llovizna);
  VERIFICAR(cieloDe(73) == Cielo::Nieve && cieloDe(95) == Cielo::Tormenta);
  textoTemperatura(-3, t, sizeof t);
  VERIFICAR(std::string(t) == "-3°");
  DatosClima frio = c;
  frio.decimas = -25;
  VERIFICAR(frio.temperatura() == -3);
  VERIFICAR(std::string(caraClima(frio)) == "frio" && situacionClima(frio) == Situacion::ClimaFrio);
  DatosClima lluvia = c;
  lluvia.codigo = 63;
  VERIFICAR(std::string(caraClima(lluvia)) == "triste" && situacionClima(lluvia) == Situacion::ClimaLluvia);
  DatosClima sol = c;
  sol.codigo = 0;
  VERIFICAR(std::string(caraClima(sol)) == "contento" && situacionClima(sol) == Situacion::ClimaLindo);
}

PRUEBA("clima: cada ícono se dibuja adentro de su caja") {
  static uint16_t px[120 * 120];
  Lienzo l;
  l.iniciar(px, 120, 120);
  for (int c = 0; c <= static_cast<int>(Cielo::Tormenta); c++) {
    for (int dia = 0; dia < 2; dia++) {
      l.limpiar(paleta::PANTALLA);
      dibujarIconoClima(l, static_cast<Cielo>(c), dia, 60, 60, 60, 1.3f);
      int pintados = 0, afuera = 0;
      const uint16_t fondo = a565(paleta::PANTALLA);
      for (int y = 0; y < 120; y++)
        for (int x = 0; x < 120; x++) {
          if (px[y * 120 + x] == fondo) continue;
          pintados++;
          if (x < 24 || x > 96 || y < 20 || y > 100) afuera++;
        }
      VERIFICAR(pintados > 200);
      VERIFICAR(afuera == 0);
    }
  }
}

PRUEBA("globo: el texto entra en la pantalla rectangular y en la redonda") {
  TextoAjustado a;
  ajustarTexto("¡Feliz cumple, Nahuel!", 120, 290, 2, false, 320, 172, 0, &a);
  VERIFICAR(a.fuente == &fuentes::VOZ_20 && a.n == 1);
  ajustarTexto("Una frase bastante larga que no entra en una sola línea ni loca", 120, 290, 2, false, 320, 172, 0, &a);
  VERIFICAR(a.n >= 2 && a.ancho <= 290);
  // En la redonda (240): hasta 3 líneas, cada una adentro del círculo.
  ajustarTexto("Una frase bastante larga que no entra en una sola línea ni loca", 170, 220, 3, true, 240, 240, 14, &a);
  VERIFICAR(a.n >= 2 && a.n <= MAX_LINEAS_TEXTO);
  for (int i = 0; i < a.n; i++) {
    char linea[64];
    copiarTexto(linea, sizeof linea, a.texto + a.ini[i], static_cast<size_t>(a.fin[i] - a.ini[i]));
    const float w = anchoTexto(*a.fuente, linea);
    const float y0 = a.top + i * a.lh, y1 = y0 + a.lh;
    const float dy = std::fmax(std::fabs(y0 - 120), std::fabs(y1 - 120));
    VERIFICAR(w <= 2 * std::sqrt(114.0f * 114 - dy * dy));
  }
}
