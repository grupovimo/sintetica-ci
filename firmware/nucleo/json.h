// Parser JSON sin memoria dinámica: tokeniza en un arreglo que da quien llama.
// Cada token guarda dónde termina su subárbol, así recorrer es O(1) por salto.
#pragma once

#include "mate.h"

namespace sintetica {

enum class TipoJson : uint8_t { Objeto, Arreglo, Texto, Numero, Verdadero, Falso, Nulo };

struct TokenJson {
  TipoJson tipo;
  uint16_t hijos;      // objeto: pares clave/valor; arreglo: elementos
  int32_t ini, fin;    // rango en el texto (texto: sin las comillas)
  int32_t siguiente;   // índice del primer token después de este subárbol
};

enum class ErrorJson : int8_t {
  Ninguno = 0,
  Sintaxis = -1,
  SinLugar = -2,     // no alcanzan los tokens
  MuyProfundo = -3,
  Vacio = -4,
};

class Json {
 public:
  // Devuelve la cantidad de tokens (>0) o un ErrorJson (<0).
  int parsear(const char* texto, size_t largo, TokenJson* tokens, int maxTokens);

  // Navegación (índices de token). -1 = no está.
  int buscar(int objeto, const char* clave) const;
  int elemento(int arreglo, int k) const;
  int cantidad(int i) const { return valido(i) ? toks_[i].hijos : 0; }
  TipoJson tipo(int i) const { return toks_[i].tipo; }
  bool valido(int i) const { return i >= 0 && i < n_; }

  float numero(int i, float porDefecto) const;
  bool booleano(int i, bool porDefecto) const;
  // Copia el texto desescapado (UTF-8). Devuelve el largo o -1 si no es texto.
  int texto(int i, char* destino, size_t cap) const;
  bool textoIgual(int i, const char* s) const;

  // Para recorrer: el token que sigue al subárbol de i (hermano siguiente).
  // Objeto: la primera clave es objeto+1, su valor clave+1, y la clave
  // siguiente es saltar(valor).
  int saltar(int i) const { return toks_[i].siguiente; }

  int errorEn() const { return errorEn_; }

 private:
  int valor(int profundidad);
  int nuevo(TipoJson t, int ini);
  void espacios();

  const char* s_ = nullptr;
  int32_t largo_ = 0;
  int32_t pos_ = 0;
  TokenJson* toks_ = nullptr;
  int max_ = 0;
  int n_ = 0;
  int errorEn_ = -1;
};

}  // namespace sintetica
