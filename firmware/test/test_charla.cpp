// La charla sin IA (charla.cpp) y la voz propia (voz_propia.cpp): frases
// reales, con variantes y errores de tipeo, y lo que tiene que entender.
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fstream>
#include <sstream>
#include <string>

#include "charla.h"
#include "charla_base.h"
#include "json.h"
#include "prueba.h"
#include "voz_propia.h"

using namespace sintetica;

namespace {

// Lunes 9 de noviembre de 2026, 14:00 (hora local).
constexpr uint32_t LUNES_1400 = 1794182400u + 14 * 3600;

const char* const HABITOS[] = {"Tomar agua", "Leer"};

ContextoCharla contexto() {
  ContextoCharla c;
  c.dueno = "Nahuel";
  c.robot = "Toto";
  c.minuto = 14 * 60;
  c.epoch = LUNES_1400;
  c.clima = "24° y despejado";
  c.climaManana = "18° y lluvia";
  c.habitos[0] = HABITOS[0];
  c.habitos[1] = HABITOS[1];
  c.nHabitos = 2;
  c.segundos = 1000;
  c.semilla = 7;
  return c;
}

struct Charla {
  ContextoCharla ctx = contexto();
  EstadoCharla estado;
  RespuestaCharla r;
  const RespuestaCharla& decir(const char* t) {
    charlar(t, std::strlen(t), ctx, estado, &r);
    ctx.segundos += 5;
    if (std::getenv("DEPURAR"))
      std::printf("    «%s» → %s (%d) [%s] «%s» min=%d en=%d dia=%d dias=%d dato=%d texto=«%s»%s%s\n", t, r.nombre(), r.confianza,
                  nombreAccionCharla(r.accion), r.frase, r.e.minuto, r.e.enMinutos, r.e.dia, r.e.dias, r.dato, r.e.texto,
                  r.noEntendi ? " NO_ENTENDI" : "", r.falta ? " FALTA" : "");
    return r;
  }
};

bool es(const RespuestaCharla& r, const char* intencion) {
  if (std::string(r.nombre()) == intencion) return true;
  std::printf("    esperaba «%s», entendió «%s» (%d) «%s»\n", intencion, r.nombre(), r.confianza, r.frase);
  return false;
}

bool contiene(const char* s, const char* parte) {
  if (std::strstr(s, parte)) return true;
  std::printf("    «%s» no tiene «%s»\n", s, parte);
  return false;
}

// Una frase suelta (sin contexto).
RespuestaCharla una(const char* t, ContextoCharla ctx = contexto()) {
  EstadoCharla e;
  RespuestaCharla r;
  charlar(t, std::strlen(t), ctx, e, &r);
  if (std::getenv("DEPURAR"))
    std::printf("    «%s» → %s (%d) [%s] «%s» min=%d en=%d dia=%d dato=%d texto=«%s»\n", t, r.nombre(), r.confianza,
                nombreAccionCharla(r.accion), r.frase, r.e.minuto, r.e.enMinutos, r.e.dia, r.dato, r.e.texto);
  return r;
}

}  // namespace

PRUEBA("charla: las acciones del núcleo son las de intenciones.json, en el mismo orden") {
  VERIFICAR(static_cast<int>(AccionCharla::Cantidad) == CANTIDAD_ACCIONES_CHARLA);
  for (int i = 0; i < CANTIDAD_ACCIONES_CHARLA; i++)
    VERIFICAR(std::string(nombreAccionCharla(static_cast<AccionCharla>(i))) == ACCIONES_CHARLA[i]);
  VERIFICAR(CANTIDAD_INTENCIONES_CHARLA >= 60);
  VERIFICAR(sizeof(EstadoCharla) <= 96);
}

PRUEBA("charla: normaliza minúsculas, tildes, signos, letras repetidas y abreviaturas") {
  char s[128];
  const char* h = "¡¡Holaaa, ÑANDÚ!! ¿Qué hora es?";
  normalizarCharla(h, std::strlen(h), s, sizeof s);
  VERIFICAR(std::string(s) == "hola nandu que hora es");
  const char* t = "q onda, xq no venís a las 7:30hs";
  normalizarCharla(t, std::strlen(t), s, sizeof s);
  VERIFICAR(std::string(s) == "que onda porque no venis a las 7:30 horas");
  t = "2+2";
  normalizarCharla(t, std::strlen(t), s, sizeof s);
  VERIFICAR(std::string(s) == "2 mas 2");
}

PRUEBA("charla: el parecido tolera errores de tipeo y raíces") {
  VERIFICAR(parecidoCharla("alarma", 6, false, "alarma", 6) == 100);
  VERIFICAR(parecidoCharla("alarma", 6, false, "alrama", 6) > 0);       // dos letras dadas vuelta
  VERIFICAR(parecidoCharla("recordame", 9, false, "recordame", 9) == 100);
  VERIFICAR(parecidoCharla("recordame", 9, false, "recordme", 8) > 0);  // le falta una
  VERIFICAR(parecidoCharla("recorda", 7, true, "recordatorio", 12) > 0);
  VERIFICAR(parecidoCharla("hora", 4, false, "ahora", 5) == 0);         // la primera letra no
  VERIFICAR(parecidoCharla("dado", 4, false, "todo", 4) == 0);
  VERIFICAR(parecidoCharla("si", 2, false, "se", 2) == 0);              // las cortas, exactas
}

PRUEBA("charla: saludos, cómo estás, quién sos y la hora") {
  VERIFICAR(es(una("hola"), "saludo"));
  VERIFICAR(es(una("Holaaa Toto!!"), "saludo"));
  VERIFICAR(es(una("buenas"), "saludo"));
  VERIFICAR(es(una("que onda"), "saludo"));
  VERIFICAR(es(una("buen día"), "buen-dia"));
  VERIFICAR(es(una("buenos dias toto"), "buen-dia"));
  VERIFICAR(es(una("¿cómo estás?"), "como-estas"));
  VERIFICAR(es(una("como andas"), "como-estas"));
  VERIFICAR(es(una("hola como estas"), "como-estas"));
  VERIFICAR(es(una("como tas"), "como-estas"));
  RespuestaCharla r = una("¿Cómo te llamás?");
  VERIFICAR(es(r, "nombre-robot") && contiene(r.frase, "Toto"));
  VERIFICAR(es(una("quien te hizo"), "quien-te-hizo"));
  r = una("q hora es");
  VERIFICAR(es(r, "hora") && contiene(r.frase, "14:00"));
  VERIFICAR(es(una("Toto, ¿qué hora es?"), "hora"));
  VERIFICAR(es(una("me decís la hora porfa"), "hora"));
  r = una("que dia es hoy");
  VERIFICAR(es(r, "fecha") && contiene(r.frase, "lunes 9 de noviembre"));
  // Sin hora, lo dice.
  ContextoCharla sin = contexto();
  sin.minuto = -1, sin.epoch = 0;
  r = una("que hora es", sin);
  VERIFICAR(es(r, "hora") && !std::strchr(r.frase, '{') && r.frase[0]);
  VERIFICAR(es(una("chau"), "despedida"));
  VERIFICAR(es(una("me voy a dormir"), "buenas-noches"));
  VERIFICAR(es(una("buenas noches toto"), "buenas-noches"));
  VERIFICAR(es(una("gracias!"), "gracias"));
  VERIFICAR(es(una("perdon"), "perdon"));
  VERIFICAR(es(una("qué podés hacer"), "que-podes-hacer"));
}

PRUEBA("charla: el clima y, con contexto, «¿y mañana?»") {
  Charla c;
  VERIFICAR(es(c.decir("¿qué tiempo hace?"), "clima") && contiene(c.r.frase, "24° y despejado"));
  VERIFICAR(es(c.decir("¿y mañana?"), "clima") && contiene(c.r.frase, "18° y lluvia") && c.r.e.dia == 1);
  VERIFICAR(es(una("va a llover"), "clima"));
  VERIFICAR(es(una("como esta el clima"), "clima"));
  // Sin el pronóstico, lo dice.
  ContextoCharla sin = contexto();
  sin.clima = nullptr;
  RespuestaCharla r = una("que tiempo hace", sin);
  VERIFICAR(es(r, "clima") && r.frase[0] && !std::strstr(r.frase, "{"));
  // Pasado el contexto, "¿y mañana?" no es nada.
  c.ctx.segundos += 600;
  c.decir("¿y mañana?");
  VERIFICAR(c.r.intencion < 0 || std::string(c.r.nombre()) != "clima");
}

PRUEBA("charla: alarmas con horas dichas de muchas formas") {
  RespuestaCharla r = una("poneme una alarma a las 7");
  VERIFICAR(es(r, "alarma") && r.accion == AccionCharla::Alarma && r.e.minuto == 7 * 60 && r.e.dia == 1);
  VERIFICAR(contiene(r.frase, "7:00"));
  r = una("despertame mañana a las 7 y media");
  VERIFICAR(es(r, "alarma") && r.e.minuto == 7 * 60 + 30 && r.e.dia == 1);
  r = una("despertame a las 8 menos cuarto");
  VERIFICAR(es(r, "alarma") && r.e.minuto == 7 * 60 + 45);
  r = una("poné el despertador a las siete y cuarto");
  VERIFICAR(es(r, "alarma") && r.e.minuto == 7 * 60 + 15);
  r = una("pone una alrama a las 6");  // con error de tipeo
  VERIFICAR(es(r, "alarma") && r.e.minuto == 6 * 60);
  r = una("despertame a las 6:45");
  VERIFICAR(es(r, "alarma") && r.e.minuto == 6 * 60 + 45);
  r = una("levantame 7 y media");
  VERIFICAR(es(r, "alarma") && r.e.minuto == 7 * 60 + 30);
  r = una("despertame todos los días a las 7");
  VERIFICAR(es(r, "alarma") && r.e.minuto == 7 * 60 && r.e.dias == 0x7F && contiene(r.frase, "todos los d"));
  r = una("despertame de lunes a viernes a las 6:30");
  VERIFICAR(es(r, "alarma") && r.e.minuto == 6 * 60 + 30 && r.e.dias == 0x1F);
  r = una("poneme la alarma a las 8 de la noche");
  VERIFICAR(es(r, "alarma") && r.e.minuto == 20 * 60 && r.e.dia == 0);
  r = una("alarma en media hora");
  VERIFICAR(es(r, "alarma") && r.e.enMinutos == 30 && r.e.minuto == 14 * 60 + 30);
  VERIFICAR(es(una("sacame la alarma"), "cancelar-alarma"));
}

PRUEBA("charla: lo que falta se pregunta y se completa con la respuesta") {
  Charla c;
  c.decir("poneme una alarma");
  VERIFICAR(es(c.r, "alarma") && c.r.falta && c.r.accion == AccionCharla::Ninguna && contiene(c.r.frase, "hora"));
  c.decir("a las 9");
  VERIFICAR(es(c.r, "alarma") && !c.r.falta && c.r.accion == AccionCharla::Alarma && c.r.e.minuto == 9 * 60);
  // Y corregirla: "mejor a las 10".
  c.decir("no, mejor a las 10");
  VERIFICAR(es(c.r, "alarma") && c.r.e.minuto == 10 * 60);
  // Un recordatorio en tres pasos.
  Charla d;
  d.decir("recordame");
  VERIFICAR(es(d.r, "recordatorio") && d.r.falta && contiene(d.r.frase, "recuerd"));
  d.decir("comprar leche");
  VERIFICAR(es(d.r, "recordatorio") && d.r.falta && contiene(d.r.frase, "?"));
  d.decir("a las 5");
  VERIFICAR(es(d.r, "recordatorio") && d.r.accion == AccionCharla::Recordatorio && d.r.e.minuto == 17 * 60 &&
            std::string(d.r.e.texto) == "Comprar leche");
}

PRUEBA("charla: recordatorios con el texto, la hora y el día") {
  RespuestaCharla r = una("recordame comprar pan a las 6");
  VERIFICAR(es(r, "recordatorio") && r.e.minuto == 18 * 60 && r.e.dia == 0 && std::string(r.e.texto) == "Comprar pan");
  r = una("recordame que tengo turno con el dentista mañana a las 10");
  VERIFICAR(es(r, "recordatorio") && r.e.minuto == 10 * 60 && r.e.dia == 1 &&
            std::string(r.e.texto) == "Tengo turno con el dentista");
  r = una("acordame de llamar a mamá en 20 minutos");
  VERIFICAR(es(r, "recordatorio") && r.e.enMinutos == 20 && r.e.minuto == 14 * 60 + 20 &&
            std::string(r.e.texto) == "Llamar a mamá");
  r = una("recordame el sábado a las 10 regar las plantas");
  VERIFICAR(es(r, "recordatorio") && r.e.semana == 5 && r.e.dia == 5 && r.e.minuto == 10 * 60 &&
            std::string(r.e.texto) == "Regar las plantas");
  r = una("recordame a la noche sacar la basura");
  VERIFICAR(es(r, "recordatorio") && r.e.minuto == 21 * 60 && std::string(r.e.texto) == "Sacar la basura");
  r = una("recordame sacar la basura a las 9 de la noche");
  VERIFICAR(es(r, "recordatorio") && r.e.minuto == 21 * 60);
  r = una("avisame en 10 minutos");
  VERIFICAR(es(r, "temporizador") && r.accion == AccionCharla::Recordatorio && r.e.enMinutos == 10 && r.e.texto[0]);
  r = una("temporizador de 5 minutos");
  VERIFICAR(es(r, "temporizador") && r.e.enMinutos == 5);
  r = una("recordame tomar la pastilla en una hora");
  VERIFICAR(es(r, "recordatorio") && r.e.enMinutos == 60 && std::string(r.e.texto) == "Tomar la pastilla");
}

PRUEBA("charla: juegos, modos, chistes y la música") {
  RespuestaCharla r = una("juguemos a piedra papel o tijera");
  VERIFICAR(es(r, "jugar") && r.accion == AccionCharla::Jugar && r.dato == 0);
  r = una("queres jugar a los reflejos?");
  VERIFICAR(es(r, "jugar") && r.dato == 1);
  Charla c;
  c.decir("juguemos");
  VERIFICAR(es(c.r, "jugar") && c.r.falta);
  c.decir("a los dados");
  VERIFICAR(es(c.r, "jugar") && c.r.accion == AccionCharla::Jugar && c.r.dato == 3);
  r = una("tirá un dado");
  VERIFICAR(es(r, "dado") && r.accion == AccionCharla::Jugar && r.dato == 3);
  r = una("cara o ceca");
  VERIFICAR(es(r, "moneda") && r.dato == 4);
  r = una("modo reloj");
  VERIFICAR(es(r, "modo-reloj") && r.accion == AccionCharla::Modo && r.dato == 1);
  r = una("quiero concentrarme");
  VERIFICAR(es(r, "modo-pomodoro") && r.dato == 2);
  Charla ch;
  ch.decir("contame un chiste");
  VERIFICAR(es(ch.r, "chiste"));
  const std::string primero = ch.r.frase;
  ch.decir("otro");
  VERIFICAR(es(ch.r, "chiste") && primero != ch.r.frase);
  r = una("poneme musica de Charly");
  VERIFICAR(es(r, "musica") && r.accion == AccionCharla::Musica && std::string(r.e.texto) == "Charly");
  VERIFICAR(es(una("bailemos"), "bailar"));
}

PRUEBA("charla: el ánimo, el cariño y los insultos con dignidad") {
  RespuestaCharla r = una("estoy re cansado");
  VERIFICAR(es(r, "cansado") && r.accion == AccionCharla::Animo && r.dato == 1);
  r = una("estoy triste");
  VERIFICAR(es(r, "triste") && r.dato == 2 && std::string(r.expresion) == "triste");
  r = una("tuve un mal dia");
  VERIFICAR(es(r, "triste"));
  VERIFICAR(es(una("estoy aburrido"), "aburrido"));
  VERIFICAR(es(una("que embole"), "aburrido"));
  r = una("estoy feliz");
  VERIFICAR(es(r, "feliz") && r.dato == 0);
  VERIFICAR(es(una("te quiero mucho"), "te-quiero"));
  VERIFICAR(es(una("tqm"), "te-quiero"));
  VERIFICAR(es(una("sos un tonto"), "insulto"));
  VERIFICAR(es(una("callate"), "callate"));
  VERIFICAR(es(una("jajaja"), "risa"));
  VERIFICAR(es(una("estoy nervioso"), "nervioso"));
  VERIFICAR(es(una("me duele la cabeza"), "enfermo"));
  VERIFICAR(es(una("tengo hambre"), "hambre"));
  VERIFICAR(es(una("hoy es mi cumple!"), "cumple-dueno"));
  // Después del "¿cómo estás?" del robot: "bien" y "más o menos".
  r = una("bien");
  VERIFICAR(es(r, "estoy-bien") && r.accion == AccionCharla::Animo && r.dato == 0);
  r = una("mas o menos");
  VERIFICAR(es(r, "estoy-medio") && r.dato == 1);
}

PRUEBA("charla: gustos, el nombre, los hábitos y los toques") {
  RespuestaCharla r = una("me gusta el mate");
  VERIFICAR(es(r, "gusto") && r.accion == AccionCharla::Gusto && std::string(r.e.texto) == "el mate");
  r = una("no me gusta el brócoli");
  VERIFICAR(es(r, "no-gusta") && r.accion == AccionCharla::OlvidarGusto && std::string(r.e.texto) == "el brócoli");
  ContextoCharla g = contexto();
  g.gustos = "el mate\nla pizza";
  r = una("¿qué me gusta?", g);
  VERIFICAR(es(r, "que-me-gusta") && contiene(r.frase, "el mate y la pizza"));
  r = una("que me gusta");
  VERIFICAR(es(r, "que-me-gusta") && r.frase[0] && !std::strchr(r.frase, '{'));
  r = una("me llamo Nahuel");
  VERIFICAR(es(r, "me-llamo") && r.accion == AccionCharla::Dueno && std::string(r.e.texto) == "Nahuel");
  r = una("ya tomé agua");
  VERIFICAR(es(r, "habito-hecho") && r.accion == AccionCharla::Habito && r.dato == 0 && contiene(r.frase, "Tomar agua"));
  r = una("ya lei");
  VERIFICAR(es(r, "habito-hecho") && r.dato == 1);
  ContextoCharla sinHabitos = contexto();
  sinHabitos.nHabitos = 0;
  r = una("ya tomé agua", sinHabitos);
  VERIFICAR(r.accion == AccionCharla::Ninguna);
  r = una("mandale un abrazo a Ana");
  VERIFICAR(es(r, "toque-amigo") && r.accion == AccionCharla::Toque && r.dato == 1 && std::string(r.e.texto) == "Ana");
  r = una("dale un beso a mamá");
  VERIFICAR(es(r, "toque-amigo") && r.dato == 2 && std::string(r.e.texto) == "mamá");
}

PRUEBA("charla: las cuentas, la alarma que no suena y lo que no entiende") {
  RespuestaCharla r = una("cuánto es 2 más 2");
  VERIFICAR(es(r, "cuenta") && contiene(r.frase, "4") && !r.noEntendi);
  r = una("cuanto es 7 x 8");
  VERIFICAR(es(r, "cuenta") && contiene(r.frase, "56"));
  r = una("cuanto es 5 dividido 2");
  VERIFICAR(es(r, "cuenta") && contiene(r.frase, "2,5"));
  r = una("posponer");
  VERIFICAR(es(r, "posponer") && r.accion == AccionCharla::Ninguna);
  ContextoCharla sonando = contexto();
  sonando.alarmaSonando = true;
  r = una("cinco minutos más", sonando);
  VERIFICAR(es(r, "posponer") && r.accion == AccionCharla::Posponer);
  r = una("ya me desperté", sonando);
  VERIFICAR(es(r, "apagar-alarma") && r.accion == AccionCharla::ApagarAlarma);
  r = una("asdfgh qwerty zxcv");
  VERIFICAR(r.intencion < 0 && r.noEntendi && r.frase[0]);
  r = una("");
  VERIFICAR(r.noEntendi && r.frase[0]);
  // Las preguntas difíciles: contesta algo, pero que pruebe la IA si hay.
  r = una("¿por qué el cielo es azul?");
  VERIFICAR(es(r, "pregunta-dificil") && r.noEntendi && r.frase[0]);
  r = una("cual es la capital de Francia");
  VERIFICAR(r.noEntendi);
}

PRUEBA("charla: la personalidad elige sus variantes y la misma semilla da lo mismo") {
  ContextoCharla g = contexto();
  g.preset = "grunon";
  int propias = 0;
  for (uint32_t s = 1; s <= 30; s++) {
    g.semilla = s;
    RespuestaCharla r = una("hola", g);
    if (std::string(r.frase) == "Hola. ¿Qué querés?" || std::string(r.frase) == "Mmm, hola.") propias++;
  }
  VERIFICAR(propias >= 8);  // pesan el doble
  RespuestaCharla a = una("contame un chiste"), b = una("contame un chiste");
  VERIFICAR(std::string(a.frase) == b.frase);
  // Sin dueño no saluda con "{dueno}" suelto.
  ContextoCharla sinDueno = contexto();
  sinDueno.dueno = nullptr;
  for (uint32_t s = 1; s <= 20; s++) {
    sinDueno.semilla = s;
    RespuestaCharla r = una("hola", sinDueno);
    VERIFICAR(!std::strchr(r.frase, '{') && r.frase[0] && r.frase[0] != ',');
  }
}

PRUEBA("charla: todas las respuestas entran en el globo y no quedan plantillas sin llenar") {
  const char* const frases[] = {"hola", "que hora es", "poneme una alarma a las 7", "recordame comprar pan a las 6",
                                "contame un chiste", "estoy triste", "me gusta el mate", "mandale un abrazo a Ana",
                                "ya tomé agua", "que dia es hoy", "como te llamas", "cuantos años tenes"};
  for (const char* f : frases)
    for (uint32_t s = 1; s <= 12; s++) {
      ContextoCharla c = contexto();
      c.semilla = s;
      c.diasJuntos = 42;
      RespuestaCharla r = una(f, c);
      VERIFICAR(r.frase[0] && std::strlen(r.frase) <= 60 && !std::strchr(r.frase, '{'));
    }
}

PRUEBA("charla: los gustos se suman sin repetir, se van los viejos y se sacan") {
  char l[LARGO_LISTA_GUSTOS] = {};
  sumarGusto(l, sizeof l, "el mate");
  sumarGusto(l, sizeof l, "la pizza");
  sumarGusto(l, sizeof l, "El Mate");
  VERIFICAR(std::string(l) == "la pizza\nEl Mate");
  sumarGusto(l, sizeof l, "el rock");
  sumarGusto(l, sizeof l, "los perros");
  sumarGusto(l, sizeof l, "el helado");
  VERIFICAR(std::string(l) == "El Mate\nel rock\nlos perros\nel helado");
  sacarGusto(l, sizeof l, "perros");
  VERIFICAR(std::string(l) == "El Mate\nel rock\nel helado");
}

// ---------------------------------------------------------------- los casos

namespace {

struct ResultadoCasos {
  int total = 0, bien = 0;
  int accionesDeMas = 0;  // hizo algo que no tenía que hacer (lo peor)
  int iaPerdida = 0;      // una pregunta difícil que no le pasó a la IA
};

// shared/formato/charla/casos.json: cada frase con lo que tiene que entender.
ResultadoCasos correrCasos(bool mostrar) {
  ResultadoCasos res;
  std::ifstream archivo(std::string(RAIZ_REPO) + "/shared/formato/charla/casos.json");
  std::stringstream ss;
  ss << archivo.rdbuf();
  const std::string texto = ss.str();
  static TokenJson toks[12000];
  Json j;
  if (j.parsear(texto.data(), texto.size(), toks, 12000) <= 0) return res;
  const int casos = j.buscar(0, "casos");
  auto cadena = [&](int obj, const char* clave, std::string* v) {
    const int t = j.buscar(obj, clave);
    char b[256];
    if (t < 0 || j.texto(t, b, sizeof b) < 0) return false;
    *v = b;
    return true;
  };
  auto numero = [&](int obj, const char* clave, int* v) {
    const int t = j.buscar(obj, clave);
    if (t < 0) return false;
    *v = static_cast<int>(j.numero(t, 0));
    return true;
  };
  for (int k = 0; k < j.cantidad(casos); k++) {
    const int c = j.elemento(casos, k);
    std::string di, s;
    cadena(c, "di", &di);
    Charla ch;
    const int sonando = j.buscar(c, "sonando");
    if (sonando >= 0) ch.ctx.alarmaSonando = j.booleano(sonando, false);
    const int antes = j.buscar(c, "antes");
    for (int a = 0; a < j.cantidad(antes); a++) {
      char b[256];
      j.texto(j.elemento(antes, a), b, sizeof b);
      ch.decir(b);
    }
    const RespuestaCharla& r = ch.decir(di.c_str());
    std::string mal;
    int n;
    if (cadena(c, "es", &s) && s != r.nombre()) mal += " es=" + std::string(r.nombre());
    if (cadena(c, "accion", &s) && s != nombreAccionCharla(r.accion)) {
      mal += " accion=" + std::string(nombreAccionCharla(r.accion));
      if (s == "ninguna") res.accionesDeMas++;
    }
    const int conf = j.buscar(c, "confirma");
    if (conf >= 0 && j.booleano(conf, false) != r.confirma) mal += r.confirma ? " confirma" : " no-confirma";
    const int ia = j.buscar(c, "ia");
    if (ia >= 0 && j.booleano(ia, false) != r.noEntendi) {
      mal += r.noEntendi ? " no_entendi" : " entendio";
      if (j.booleano(ia, false)) res.iaPerdida++;
    }
    if (cadena(c, "texto", &s) && s != r.e.texto) mal += " texto=«" + std::string(r.e.texto) + "»";
    if (numero(c, "minuto", &n) && n != r.e.minuto) mal += " minuto=" + std::to_string(r.e.minuto);
    if (numero(c, "en", &n) && n != r.e.enMinutos) mal += " en=" + std::to_string(r.e.enMinutos);
    if (numero(c, "dia", &n) && n != r.e.dia) mal += " dia=" + std::to_string(r.e.dia);
    if (numero(c, "dias", &n) && n != r.e.dias) mal += " dias=" + std::to_string(r.e.dias);
    if (numero(c, "dato", &n) && n != r.dato) mal += " dato=" + std::to_string(r.dato);
    res.total++;
    if (mal.empty()) res.bien++;
    else if (mostrar) std::printf("    «%s»:%s (%d) «%s»\n", di.c_str(), mal.c_str(), r.confianza, r.frase);
  }
  return res;
}

}  // namespace

PRUEBA("charla: los casos de casos.json (frases reales): acierta, no hace lo que no le pidieron y deja a la IA lo difícil") {
  const ResultadoCasos r = correrCasos(true);
  std::printf("    casos: %d de %d (%d%%), acciones de más: %d, IA perdida: %d\n", r.bien, r.total,
              r.total ? r.bien * 100 / r.total : 0, r.accionesDeMas, r.iaPerdida);
  VERIFICAR(r.total >= 150);
  // Antes de la v1.11 (sin confirmar ni no_empieza): 242 de 297 (81 %), 31 acciones de más.
  VERIFICAR(r.bien * 100 >= r.total * 97);
  VERIFICAR(r.accionesDeMas == 0);
  VERIFICAR(r.iaPerdida == 0);
}

PRUEBA("charla: lo que cambia algo y es dudoso se pregunta antes; con «sí» se hace, con «no», nada") {
  Charla c;
  c.decir("decime Zoe");
  VERIFICAR(es(c.r, "me-llamo") && c.r.confirma && c.r.falta && c.r.accion == AccionCharla::Ninguna &&
            contiene(c.r.frase, "Zoe") && contiene(c.r.frase, "?"));
  c.decir("si");
  VERIFICAR(es(c.r, "me-llamo") && c.r.accion == AccionCharla::Dueno && !c.r.confirma && std::string(c.r.e.texto) == "Zoe");
  // Con "no", no hace nada (y lo dice).
  Charla d;
  d.decir("llamame Nacho");
  VERIFICAR(d.r.confirma);
  d.decir("no");
  VERIFICAR(d.r.accion == AccionCharla::Ninguna && !d.r.confirma && d.r.frase[0]);
  d.decir("si");  // ya no espera nada
  VERIFICAR(d.r.accion == AccionCharla::Ninguna);
  // Una alarma dudosa guarda la hora para el "sí".
  Charla a;
  a.decir("mi alarma sono a las 6");
  VERIFICAR(a.r.confirma && a.r.accion == AccionCharla::Ninguna && a.r.e.minuto == 6 * 60);
  a.decir("dale");
  VERIFICAR(es(a.r, "alarma") && a.r.accion == AccionCharla::Alarma && a.r.e.minuto == 6 * 60 && a.r.e.dia == 1);
  // Pasado el contexto, el "sí" ya no confirma.
  Charla v;
  v.decir("decime Zoe");
  v.ctx.segundos += 600;
  v.decir("si");
  VERIFICAR(v.r.accion == AccionCharla::Ninguna);
  // "Decime de Zoe" no es un nombre: ni lo cambia ni pregunta (que conteste la IA).
  RespuestaCharla r = una("decime de Zoe");
  VERIFICAR(r.accion == AccionCharla::Ninguna && !r.confirma && r.noEntendi);
  // Lo seguro se hace sin preguntar.
  r = una("me llamo Zoe");
  VERIFICAR(r.accion == AccionCharla::Dueno && !r.confirma);
  r = una("olvidate que me gusta el mate");
  VERIFICAR(r.accion == AccionCharla::OlvidarGusto && std::string(r.e.texto) == "el mate" && !r.confirma);
}

// ---------------------------------------------------------------- la voz

namespace {
uint32_t hashMuestras(const int16_t* m, size_t n) {
  uint32_t h = 2166136261u;
  for (size_t i = 0; i < n; i++) h = (h ^ static_cast<uint16_t>(m[i])) * 16777619u;
  return h;
}
size_t decir(const char* texto, const ParamsVoz& p, int16_t* m, size_t cap) {
  VozPropia v;
  v.empezar(texto, p);
  size_t n = 0;
  while (n < cap) {
    const size_t k = v.generar(m + n, cap - n < 256 ? cap - n : 256);
    n += k;
    if (k < 256) break;
  }
  return n;
}
int16_t g_muestras[16000 * 15];
}  // namespace

PRUEBA("voz: genera muestras acotadas, deterministas y del largo que dice") {
  ParamsVoz p;
  const char* t = "¡Hola, Nahuel! ¿Jugamos a algo?";
  const size_t n = decir(t, p, g_muestras, sizeof g_muestras / 2);
  VERIFICAR(n > 16000 / 2 && n < 16000 * 6);  // entre medio segundo y 6 s
  VERIFICAR(n == VozPropia::duracion(t, p));
  int pico = 0;
  double energia = 0;
  for (size_t i = 0; i < n; i++) {
    const int a = g_muestras[i] < 0 ? -g_muestras[i] : g_muestras[i];
    if (a > pico) pico = a;
    energia += static_cast<double>(g_muestras[i]) * g_muestras[i];
  }
  const double rms = std::sqrt(energia / n);
  if (std::getenv("DEPURAR")) std::printf("    %zu muestras, pico %d, rms %.0f\n", n, pico, rms);
  VERIFICAR(pico <= 30000 && pico > 2000);
  VERIFICAR(rms > 300);
  const uint32_t h = hashMuestras(g_muestras, n);
  VERIFICAR(decir(t, p, g_muestras, sizeof g_muestras / 2) == n && hashMuestras(g_muestras, n) == h);
  // Otra voz suena distinto; sin volumen, silencio; sin texto, nada.
  ParamsVoz robot = p;
  robot.robot = true;
  robot.tono = 150;
  const size_t nr = decir(t, robot, g_muestras, sizeof g_muestras / 2);
  VERIFICAR(hashMuestras(g_muestras, nr) != h);
  ParamsVoz muda = p;
  muda.volumen = 0;
  const size_t nm = decir(t, muda, g_muestras, sizeof g_muestras / 2);
  bool silencio = true;
  for (size_t i = 0; i < nm; i++)
    if (g_muestras[i]) silencio = false;
  VERIFICAR(silencio);
  VERIFICAR(decir("", p, g_muestras, 100) == 0);
  // El texto más largo del globo (60 bytes) dura menos de 12 s, aun lento.
  ParamsVoz lento = p;
  lento.velocidad = 60;
  VERIFICAR(VozPropia::duracion("Aaaaaaaaaa eeeeeeeeee iiiiiiiiii oooooooooo uuuuuuuuuu aaaaaa", lento) < 16000u * 12);
}

PRUEBA("voz: el ánimo y la personalidad cambian el tono y la velocidad") {
  Personalidad base;
  const ParamsVoz triste = paramsVoz(1, 70, base, 0.2f, 0.1f), feliz = paramsVoz(1, 70, base, 0.9f, 0.9f);
  VERIFICAR(feliz.tono > triste.tono && feliz.velocidad > triste.velocidad);
  const ParamsVoz grave = paramsVoz(0, 70, base, 0.5f, 0.5f), aguda = paramsVoz(2, 70, base, 0.5f, 0.5f);
  VERIFICAR(aguda.tono > grave.tono);
  VERIFICAR(paramsVoz(3, 70, base, 0.5f, 0.5f).robot);
  Personalidad timido = base;
  timido.timidez = 1;
  VERIFICAR(paramsVoz(1, 70, timido, 0.5f, 0.5f).volumen < paramsVoz(1, 70, base, 0.5f, 0.5f).volumen);
}
