// Puente JNI entre Kotlin (com.sinteticalabs.bicho.Nucleo) y la API C del
// núcleo. Todo se llama desde el hilo de GL: el núcleo es una instancia por
// proceso y no es de varios hilos.
#include <GLES2/gl2.h>
#include <android/log.h>
#include <jni.h>

#include <atomic>
#include <cctype>
#include <chrono>
#include <random>
#include <cstdio>
#include <cstring>
#include <string>

#include "formato.h"
#include "sintetica_c.h"

namespace {
JNIEnv* g_env = nullptr;
jobject g_anfitrion = nullptr;  // referencia global
jmethodID g_guardar, g_leer, g_registrar, g_notificar, g_notificarNube, g_redCambio, g_sonido;
char g_id[13] = "000000000000";
std::string g_carpeta;  // archivos grandes (imágenes, sprites): <filesDir>/archivos

// --- Archivos grandes por partes (imágenes y sprites), en archivos comunes ---
std::string rutaArchivo(const char* clave) {
  std::string r = g_carpeta + "/";
  for (const char* c = clave; *c; c++) r += (isalnum(static_cast<unsigned char>(*c)) || *c == '-') ? *c : '_';
  return r;
}
int escribirTrozo(void*, const char* clave, uint32_t off, const uint8_t* d, size_t n) {
  FILE* f = fopen(rutaArchivo(clave).c_str(), off == 0 ? "wb" : "r+b");
  if (!f) return 0;
  bool ok = fseek(f, static_cast<long>(off), SEEK_SET) == 0 && fwrite(d, 1, n, f) == n;
  return fclose(f) == 0 && ok ? 1 : 0;
}
int leerTrozo(void*, const char* clave, uint32_t off, uint8_t* d, size_t n) {
  FILE* f = fopen(rutaArchivo(clave).c_str(), "rb");
  if (!f) return -1;
  int r = fseek(f, static_cast<long>(off), SEEK_SET) == 0 ? static_cast<int>(fread(d, 1, n, f)) : 0;
  fclose(f);
  return r;
}
int borrar(void*, const char* clave) {
  remove(rutaArchivo(clave).c_str());
  return 1;
}
std::mt19937 g_azar{std::random_device{}()};
const auto g_inicio = std::chrono::steady_clock::now();
std::atomic<int> g_bateria{-1};  // la escribe el hilo principal
std::atomic<int> g_cargando{-1};
bool g_listo = false;             // hasta iniciar, el núcleo no acepta nada

uint32_t ahoraMs(void*) {
  return static_cast<uint32_t>(
      std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::steady_clock::now() - g_inicio).count());
}
uint32_t semilla(void*) { return g_azar(); }
// La hora del bicho: la real, o acelerada para probar el ritmo diario (×720 =
// un día en 2 minutos). Solo corre más rápido el reloj de pared; la lógica del
// motor sigue a 60 Hz con ahora_ms.
std::atomic<int> g_factor{1};
time_t g_baseReal = 0, g_baseBicho = 0;

time_t horaBicho() {
  time_t real = time(nullptr);
  if (g_factor.load() <= 1 || g_baseReal == 0) return real;
  return g_baseBicho + (real - g_baseReal) * g_factor.load();
}
int minuto(void*) {
  time_t t = horaBicho();
  struct tm l;
  localtime_r(&t, &l);
  return l.tm_hour * 60 + l.tm_min;
}
uint32_t epoch(void*) {
  time_t t = horaBicho();
  struct tm l;
  localtime_r(&t, &l);
  return static_cast<uint32_t>(t + l.tm_gmtoff);
}
int bateria(void*) { return g_bateria.load(); }
int cargando(void*) { return g_cargando.load(); }

int guardar(void*, const char* clave, const uint8_t* d, size_t n) {
  if (!g_anfitrion) return 0;
  jstring c = g_env->NewStringUTF(clave);
  jbyteArray a = g_env->NewByteArray(static_cast<jsize>(n));
  g_env->SetByteArrayRegion(a, 0, static_cast<jsize>(n), reinterpret_cast<const jbyte*>(d));
  jboolean ok = g_env->CallBooleanMethod(g_anfitrion, g_guardar, c, a);
  g_env->DeleteLocalRef(c);
  g_env->DeleteLocalRef(a);
  return ok ? 1 : 0;
}

int leer(void*, const char* clave, uint8_t* d, size_t cap) {
  if (!g_anfitrion) return -1;
  jstring c = g_env->NewStringUTF(clave);
  auto a = static_cast<jbyteArray>(g_env->CallObjectMethod(g_anfitrion, g_leer, c));
  g_env->DeleteLocalRef(c);
  if (!a) return -1;
  jsize n = g_env->GetArrayLength(a);
  int r = -1;
  if (static_cast<size_t>(n) <= cap) {
    g_env->GetByteArrayRegion(a, 0, n, reinterpret_cast<jbyte*>(d));
    r = n;
  }
  g_env->DeleteLocalRef(a);
  return r;
}

void llamarConBytes(jmethodID m, int car, const uint8_t* d, size_t n) {
  if (!g_anfitrion) return;
  jbyteArray a = g_env->NewByteArray(static_cast<jsize>(n));
  g_env->SetByteArrayRegion(a, 0, static_cast<jsize>(n), reinterpret_cast<const jbyte*>(d));
  g_env->CallVoidMethod(g_anfitrion, m, car, a);
  g_env->DeleteLocalRef(a);
}
void notificar(void*, int car, const uint8_t* d, size_t n) { llamarConBytes(g_notificar, car, d, n); }
void notificarNube(void*, int car, const uint8_t* d, size_t n) { llamarConBytes(g_notificarNube, car, d, n); }
void redCambio(void*) {
  if (g_anfitrion) g_env->CallVoidMethod(g_anfitrion, g_redCambio);
}

void sonido(void*, const char* nombre) {
  if (!g_anfitrion) return;
  jstring s = g_env->NewStringUTF(nombre);
  g_env->CallVoidMethod(g_anfitrion, g_sonido, s);
  g_env->DeleteLocalRef(s);
}

void registrar(void*, const char* m) {
  __android_log_print(ANDROID_LOG_INFO, "SinteticaOS", "%s", m);
  if (!g_anfitrion) return;
  jstring s = g_env->NewStringUTF(m);
  g_env->CallVoidMethod(g_anfitrion, g_registrar, s);
  g_env->DeleteLocalRef(s);
}
}  // namespace

#define JNI(nombre) JNIEXPORT JNICALL Java_com_sinteticalabs_bicho_Nucleo_##nombre

extern "C" {

void JNI(iniciarNativo)(JNIEnv* env, jobject, jint ancho, jint alto, jstring id, jstring carpeta, jobject anfitrion) {
  g_env = env;
  if (g_anfitrion) env->DeleteGlobalRef(g_anfitrion);
  g_anfitrion = env->NewGlobalRef(anfitrion);
  jclass k = env->GetObjectClass(g_anfitrion);
  g_guardar = env->GetMethodID(k, "guardar", "(Ljava/lang/String;[B)Z");
  g_leer = env->GetMethodID(k, "leer", "(Ljava/lang/String;)[B");
  g_registrar = env->GetMethodID(k, "registrar", "(Ljava/lang/String;)V");
  g_notificar = env->GetMethodID(k, "notificar", "(I[B)V");
  g_notificarNube = env->GetMethodID(k, "notificarNube", "(I[B)V");
  g_redCambio = env->GetMethodID(k, "redCambio", "()V");
  g_sonido = env->GetMethodID(k, "sonido", "(Ljava/lang/String;)V");
  const char* cp = env->GetStringUTFChars(carpeta, nullptr);
  g_carpeta = cp;
  env->ReleaseStringUTFChars(carpeta, cp);
  const char* t = env->GetStringUTFChars(id, nullptr);
  if (strlen(t) == 12) memcpy(g_id, t, 13);
  env->ReleaseStringUTFChars(id, t);
  static sintetica_hal h{};
  h.ahora_ms = ahoraMs;
  h.semilla = semilla;
  h.minuto_del_dia = minuto;
  h.epoch = epoch;
  h.bateria = bateria;
  h.cargando = cargando;
  h.sonido = sonido;
  h.guardar = guardar;
  h.leer = leer;
  h.registrar = registrar;
  h.notificar = notificar;
  h.notificar_nube = notificarNube;
  h.red_cambio = redCambio;
  h.identificador = g_id;
  h.escribir_trozo = escribirTrozo;
  h.leer_trozo = leerTrozo;
  h.borrar = borrar;
  h.perfil = "celu";
  sintetica_iniciar(&h, ancho, alto);
  // En el celu, dejar el dedo es acariciarlo: el QR sale del menú secreto.
  sintetica_qr_por_mantener(0);
  g_listo = true;
}

jboolean JNI(listo)(JNIEnv*, jobject) { return g_listo ? JNI_TRUE : JNI_FALSE; }

void JNI(tick)(JNIEnv* env, jobject) {
  if (!g_listo) return;
  g_env = env;
  sintetica_tick();
}

// Sube el cuadro (RGB565, tal cual lo dibuja el núcleo) a la textura que está
// ligada. La primera vez la crea con glTexImage2D.
void JNI(subirCuadro)(JNIEnv*, jobject, jboolean crear) {
  const int w = sintetica_ancho(), h = sintetica_alto();
  glPixelStorei(GL_UNPACK_ALIGNMENT, 2);
  if (crear)
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGB, w, h, 0, GL_RGB, GL_UNSIGNED_SHORT_5_6_5, sintetica_pixeles());
  else
    glTexSubImage2D(GL_TEXTURE_2D, 0, 0, 0, w, h, GL_RGB, GL_UNSIGNED_SHORT_5_6_5, sintetica_pixeles());
}

// Cambia la velocidad del reloj de pared desde la hora que tiene ahora el bicho.
void JNI(acelerar)(JNIEnv*, jobject, jint factor) {
  time_t ahora = horaBicho();
  g_baseReal = time(nullptr);
  g_baseBicho = ahora;
  g_factor.store(factor < 1 ? 1 : factor);
}
jint JNI(minutoDelDia)(JNIEnv*, jobject) { return minuto(nullptr); }

// --- Lo que en una placa es BLE y WiFi (acá, HTTP en la red local y MQTT) ---
void JNI(bleConexion)(JNIEnv* env, jobject, jboolean c) {
  if (!g_listo) return;
  g_env = env;
  sintetica_ble_conexion(c ? 1 : 0);
}
void JNI(bleEscribir)(JNIEnv* env, jobject, jint car, jbyteArray datos) {
  if (!g_listo) return;
  g_env = env;
  jsize n = env->GetArrayLength(datos);
  jbyte* d = env->GetByteArrayElements(datos, nullptr);
  sintetica_ble_escribir(car, reinterpret_cast<const uint8_t*>(d), static_cast<size_t>(n));
  env->ReleaseByteArrayElements(datos, d, JNI_ABORT);
}
void JNI(nubeEscribir)(JNIEnv* env, jobject, jint car, jbyteArray datos) {
  if (!g_listo) return;
  g_env = env;
  jsize n = env->GetArrayLength(datos);
  jbyte* d = env->GetByteArrayElements(datos, nullptr);
  sintetica_nube_escribir(car, reinterpret_cast<const uint8_t*>(d), static_cast<size_t>(n));
  env->ReleaseByteArrayElements(datos, d, JNI_ABORT);
}
void JNI(conexiones)(JNIEnv* env, jobject, jboolean wifi, jboolean nube) {
  if (!g_listo) return;
  g_env = env;
  sintetica_conexiones(wifi ? 1 : 0, nube ? 1 : 0);
}
jbyteArray JNI(estado)(JNIEnv* env, jobject) {
  uint8_t b[160];
  size_t n = g_listo ? sintetica_ble_estado(b, sizeof b) : 0;
  jbyteArray a = env->NewByteArray(static_cast<jsize>(n));
  env->SetByteArrayRegion(a, 0, static_cast<jsize>(n), reinterpret_cast<const jbyte*>(b));
  return a;
}
// {url, usuario, clave} de la nube, o null si no tiene.
jobjectArray JNI(nube)(JNIEnv* env, jobject) {
  char u[161], us[65], k[129];
  if (!g_listo || !sintetica_nube(u, sizeof u, us, sizeof us, k, sizeof k)) return nullptr;
  jobjectArray a = env->NewObjectArray(3, env->FindClass("java/lang/String"), nullptr);
  const char* v[3] = {u, us, k};
  for (int i = 0; i < 3; i++) {
    jstring s = env->NewStringUTF(v[i]);
    env->SetObjectArrayElement(a, i, s);
    env->DeleteLocalRef(s);
  }
  return a;
}
jstring JNI(qr)(JNIEnv* env, jobject) { return env->NewStringUTF(g_listo ? sintetica_qr() : ""); }

jint JNI(ancho)(JNIEnv*, jobject) { return sintetica_ancho(); }
jint JNI(alto)(JNIEnv*, jobject) { return sintetica_alto(); }
void JNI(tacto)(JNIEnv* env, jobject, jboolean a) {
  if (!g_listo) return;
  g_env = env;
  sintetica_tacto(a ? 1 : 0);
}
void JNI(imu)(JNIEnv* env, jobject, jfloat ax, jfloat ay, jfloat az) {
  if (!g_listo) return;
  g_env = env;
  sintetica_imu(ax, ay, az);
}
void JNI(evento)(JNIEnv* env, jobject, jint e) {
  if (!g_listo) return;
  g_env = env;
  sintetica_evento(e);
}
// Un evento por su nombre del JSON ("sacudida-fuerte"); false si no existe.
jboolean JNI(eventoPorNombre)(JNIEnv* env, jobject, jstring nombre) {
  if (!g_listo) return JNI_FALSE;
  g_env = env;
  const char* s = env->GetStringUTFChars(nombre, nullptr);
  int encontrado = -1;
  for (int i = 0; i < static_cast<int>(sintetica::Evento::Cantidad); i++)
    if (strcmp(s, sintetica::nombreEvento(static_cast<sintetica::Evento>(i))) == 0) encontrado = i;
  env->ReleaseStringUTFChars(nombre, s);
  if (encontrado < 0) return JNI_FALSE;
  sintetica_evento(encontrado);
  return JNI_TRUE;
}

// Para desarrollar: lo vincula como si un celu escaneara el QR que muestra
// (SINTETICA:V1/<id>/<código>) y mandara el código por el canal de comandos.
jboolean JNI(vincularSolo)(JNIEnv* env, jobject) {
  if (!g_listo) return JNI_FALSE;
  g_env = env;
  const char* qr = sintetica_qr();
  const char* barra = qr ? strrchr(qr, '/') : nullptr;
  if (!barra || strlen(barra + 1) != 6) return JNI_FALSE;
  uint8_t cmd[7] = {0x13};  // cmdVincular
  memcpy(cmd + 1, barra + 1, 6);
  sintetica_ble_conexion(1);
  sintetica_ble_escribir(2, cmd, sizeof cmd);  // característica de comandos
  sintetica_ble_conexion(0);
  return JNI_TRUE;
}

void JNI(bateria)(JNIEnv*, jobject, jint nivel) { g_bateria.store(nivel); }
void JNI(cargando)(JNIEnv*, jobject, jboolean si) { g_cargando.store(si ? 1 : 0); }
jstring JNI(comportamiento)(JNIEnv* env, jobject) { return env->NewStringUTF(g_listo ? sintetica_comportamiento() : ""); }
jfloatArray JNI(estadoInterno)(JNIEnv* env, jobject) {
  float e[16] = {};
  const int n = g_listo ? sintetica_estado_interno(e, 16) : 0;
  jfloatArray a = env->NewFloatArray(n);
  env->SetFloatArrayRegion(a, 0, n, e);
  return a;
}
// {volumen, micrófono, tipo, conversa}
jintArray JNI(voz)(JNIEnv* env, jobject) {
  jint v[4] = {70, 60, 1, 0};
  if (g_listo) v[3] = sintetica_voz(&v[0], &v[1], &v[2], nullptr);
  jintArray a = env->NewIntArray(4);
  env->SetIntArrayRegion(a, 0, 4, v);
  return a;
}
jstring JNI(palabra)(JNIEnv* env, jobject) {
  const char* p = "";
  if (g_listo) sintetica_voz(nullptr, nullptr, nullptr, &p);
  return env->NewStringUTF(p);
}
void JNI(mostrarQr)(JNIEnv*, jobject) {
  if (g_listo) sintetica_mostrar_qr();
}
jint JNI(fpsSugerido)(JNIEnv*, jobject) { return g_listo ? sintetica_fps_sugerido() : 30; }
void JNI(seguir)(JNIEnv* env, jobject, jfloat x, jfloat y) {
  if (!g_listo) return;
  g_env = env;
  sintetica_seguir(x, y);
}
void JNI(depurar)(JNIEnv*, jobject, jboolean si) {
  if (g_listo) sintetica_depurar(si ? 1 : 0);
}
jstring JNI(expresionVisible)(JNIEnv* env, jobject) { return env->NewStringUTF(sintetica_expresion_visible()); }
jstring JNI(version)(JNIEnv* env, jobject) { return env->NewStringUTF(sintetica_version()); }
}
