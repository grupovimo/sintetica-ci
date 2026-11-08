// Puente JNI entre Kotlin (com.sinteticalabs.app.nucleo.Nucleo) y la API C del
// núcleo. Todo se llama desde el hilo principal de la app.
#include <android/log.h>
#include <jni.h>

#include <chrono>
#include <random>
#include <string>

#include "sintetica_c.h"

namespace {
JNIEnv* g_env = nullptr;
jobject g_anfitrion = nullptr;  // referencia global
jmethodID g_guardar, g_leer, g_notificar, g_registrar;
std::string g_id;
std::mt19937 g_azar{std::random_device{}()};
const auto g_inicio = std::chrono::steady_clock::now();
// Para sacar fotos de caras sin esperar los fundidos: solo crece (el tiempo
// del núcleo nunca vuelve atrás).
int64_t g_adelanto = 0;

uint32_t ahoraMs(void*) {
  return static_cast<uint32_t>(
      std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::steady_clock::now() - g_inicio).count() +
      g_adelanto);
}
uint32_t semilla(void*) { return g_azar(); }
int minuto(void*) {
  time_t t = time(nullptr);
  struct tm l;
  localtime_r(&t, &l);
  return l.tm_hour * 60 + l.tm_min;
}

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

void notificar(void*, int car, const uint8_t* d, size_t n) {
  if (!g_anfitrion) return;
  jbyteArray a = g_env->NewByteArray(static_cast<jsize>(n));
  g_env->SetByteArrayRegion(a, 0, static_cast<jsize>(n), reinterpret_cast<const jbyte*>(d));
  g_env->CallVoidMethod(g_anfitrion, g_notificar, car, a);
  g_env->DeleteLocalRef(a);
}

void registrar(void*, const char* m) {
  __android_log_print(ANDROID_LOG_INFO, "SinteticaOS", "%s", m);
  if (!g_anfitrion) return;
  jstring s = g_env->NewStringUTF(m);
  g_env->CallVoidMethod(g_anfitrion, g_registrar, s);
  g_env->DeleteLocalRef(s);
}
}  // namespace

#define JNI(nombre) JNIEXPORT JNICALL Java_com_sinteticalabs_app_nucleo_Nucleo_##nombre

extern "C" {

void JNI(iniciarNativo)(JNIEnv* env, jobject, jint ancho, jint alto, jstring id, jobject anfitrion, jboolean redonda,
                  jboolean broche) {
  g_env = env;
  if (g_anfitrion) env->DeleteGlobalRef(g_anfitrion);
  g_anfitrion = anfitrion ? env->NewGlobalRef(anfitrion) : nullptr;
  if (g_anfitrion) {
    jclass k = env->GetObjectClass(g_anfitrion);
    g_guardar = env->GetMethodID(k, "guardar", "(Ljava/lang/String;[B)Z");
    g_leer = env->GetMethodID(k, "leer", "(Ljava/lang/String;)[B");
    g_notificar = env->GetMethodID(k, "notificar", "(I[B)V");
    g_registrar = env->GetMethodID(k, "registrar", "(Ljava/lang/String;)V");
  }
  const char* s = env->GetStringUTFChars(id, nullptr);
  g_id = s;
  env->ReleaseStringUTFChars(id, s);
  sintetica_hal h{};
  h.ahora_ms = ahoraMs;
  h.semilla = semilla;
  h.minuto_del_dia = minuto;
  h.guardar = guardar;
  h.leer = leer;
  h.notificar = notificar;
  h.registrar = registrar;
  h.identificador = g_id.c_str();
  sintetica_iniciar_perfil(&h, ancho, alto, redonda ? 1 : 0, broche ? 1 : 0);
}

void JNI(vistaPrevia)(JNIEnv*, jobject) { sintetica_vista_previa(); }
void JNI(tick)(JNIEnv* env, jobject) {
  g_env = env;
  sintetica_tick();
}

// Copia el cuadro como ARGB_8888 (lo que pide Bitmap.setPixels).
void JNI(pixeles)(JNIEnv* env, jobject, jintArray destino) {
  const int n = sintetica_ancho() * sintetica_alto();
  if (env->GetArrayLength(destino) < n) return;
  jint* d = env->GetIntArrayElements(destino, nullptr);
  const uint16_t* p = sintetica_pixeles();
  for (int i = 0; i < n; i++) {
    uint32_t c = p[i];
    uint32_t r = (c >> 11) & 0x1F, g = (c >> 5) & 0x3F, b = c & 0x1F;
    r = (r << 3) | (r >> 2), g = (g << 2) | (g >> 4), b = (b << 3) | (b >> 2);
    d[i] = static_cast<jint>(0xFF000000u | (r << 16) | (g << 8) | b);
  }
  env->ReleaseIntArrayElements(destino, d, 0);
}

jint JNI(ancho)(JNIEnv*, jobject) { return sintetica_ancho(); }
jint JNI(alto)(JNIEnv*, jobject) { return sintetica_alto(); }
void JNI(tacto)(JNIEnv* env, jobject, jboolean a) {
  g_env = env;
  sintetica_tacto(a ? 1 : 0);
}
void JNI(evento)(JNIEnv* env, jobject, jint e) {
  g_env = env;
  sintetica_evento(e);
}
void JNI(bleConexion)(JNIEnv* env, jobject, jboolean c) {
  g_env = env;
  sintetica_ble_conexion(c ? 1 : 0);
}
void JNI(bleEscribir)(JNIEnv* env, jobject, jint car, jbyteArray datos) {
  g_env = env;
  jsize n = env->GetArrayLength(datos);
  jbyte* d = env->GetByteArrayElements(datos, nullptr);
  sintetica_ble_escribir(car, reinterpret_cast<const uint8_t*>(d), static_cast<size_t>(n));
  env->ReleaseByteArrayElements(datos, d, JNI_ABORT);
}
jbyteArray JNI(estado)(JNIEnv* env, jobject) {
  uint8_t b[128];
  size_t n = sintetica_ble_estado(b, sizeof b);
  jbyteArray a = env->NewByteArray(static_cast<jsize>(n));
  env->SetByteArrayRegion(a, 0, static_cast<jsize>(n), reinterpret_cast<const jbyte*>(b));
  return a;
}
jobjectArray JNI(expresiones)(JNIEnv* env, jobject) {
  int n = sintetica_cantidad_expresiones();
  jobjectArray a = env->NewObjectArray(n, env->FindClass("java/lang/String"), nullptr);
  for (int i = 0; i < n; i++) {
    jstring s = env->NewStringUTF(sintetica_nombre_expresion(i));
    env->SetObjectArrayElement(a, i, s);
    env->DeleteLocalRef(s);
  }
  return a;
}
jstring JNI(expresionVisible)(JNIEnv* env, jobject) { return env->NewStringUTF(sintetica_expresion_visible()); }
jstring JNI(qr)(JNIEnv* env, jobject) { return env->NewStringUTF(sintetica_qr()); }
jint JNI(imagenVisible)(JNIEnv*, jobject) { return sintetica_imagen_visible(); }
jstring JNI(version)(JNIEnv* env, jobject) { return env->NewStringUTF(sintetica_version()); }

void JNI(adelantar)(JNIEnv*, jobject, jint ms) {
  if (ms > 0) g_adelanto += ms;
}

// Valida una config con el mismo núcleo del robot: null si está bien; si no, por qué.
jstring JNI(validar)(JNIEnv* env, jobject, jbyteArray json) {
  jsize n = env->GetArrayLength(json);
  jbyte* d = env->GetByteArrayElements(json, nullptr);
  char detalle[256] = {0};
  int r = sintetica_validar_config(reinterpret_cast<const char*>(d), static_cast<size_t>(n), detalle, sizeof detalle);
  env->ReleaseByteArrayElements(json, d, JNI_ABORT);
  if (r == 0) return nullptr;
  return env->NewStringUTF(detalle[0] ? detalle : "No la aceptaría.");
}

// Dibuja una cara paramétrica sin tocar la instancia (ARGB, como Bitmap.setPixels).
// null si no hay una cara paramétrica con ese nombre o la config no es válida.
jintArray JNI(dibujarNativo)(JNIEnv* env, jobject, jbyteArray json, jstring nombre, jint ancho, jint alto, jboolean redonda,
                             jfloat t, jfloat cierre) {
  if (ancho < 16 || alto < 16 || ancho > 640 || alto > 640) return nullptr;
  jsize n = json ? env->GetArrayLength(json) : 0;
  jbyte* d = json ? env->GetByteArrayElements(json, nullptr) : nullptr;
  const char* nom = env->GetStringUTFChars(nombre, nullptr);
  std::string rgba(static_cast<size_t>(ancho) * alto * 4, '\0');
  char detalle[128] = {0};
  // `cierre` (el parpadeo) queda para cuando el núcleo traiga sintetica_dibujar_expresion_cierre.
  (void)cierre;
  int r = sintetica_dibujar_expresion(n > 0 ? reinterpret_cast<const char*>(d) : nullptr, static_cast<size_t>(n), nom, ancho, alto,
                                      redonda ? 1 : 0, t, reinterpret_cast<uint8_t*>(&rgba[0]), detalle, sizeof detalle);
  env->ReleaseStringUTFChars(nombre, nom);
  if (d) env->ReleaseByteArrayElements(json, d, JNI_ABORT);
  if (r != 0) return nullptr;
  const int total = ancho * alto;
  jintArray a = env->NewIntArray(total);
  jint* p = env->GetIntArrayElements(a, nullptr);
  const uint8_t* q = reinterpret_cast<const uint8_t*>(rgba.data());
  for (int i = 0; i < total; i++)
    p[i] = static_cast<jint>(0xFF000000u | (uint32_t(q[i * 4]) << 16) | (uint32_t(q[i * 4 + 1]) << 8) | q[i * 4 + 2]);
  env->ReleaseIntArrayElements(a, p, 0);
  return a;
}

// Lo de fábrica del motor de vida: tipo 0 = expresiones, 1 = comportamientos.
// [nombre, json, nombre, json, …]
jobjectArray JNI(vidaBase)(JNIEnv* env, jobject, jint tipo) {
  const char* nombre = nullptr;
  const char* json = nullptr;
  int n = sintetica_vida_base(tipo, -1, &nombre, &json);
  if (n < 0) n = 0;
  jobjectArray a = env->NewObjectArray(n * 2, env->FindClass("java/lang/String"), nullptr);
  for (int i = 0; i < n; i++) {
    nombre = nullptr;
    json = nullptr;
    sintetica_vida_base(tipo, i, &nombre, &json);
    jstring k = env->NewStringUTF(nombre ? nombre : "");
    jstring v = env->NewStringUTF(json ? json : "{}");
    env->SetObjectArrayElement(a, i * 2, k);
    env->SetObjectArrayElement(a, i * 2 + 1, v);
    env->DeleteLocalRef(k);
    env->DeleteLocalRef(v);
  }
  return a;
}
}
