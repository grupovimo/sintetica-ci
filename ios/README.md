# App de iOS (SwiftUI)

Escrita pero todavía **no compilada**: el entorno donde se hizo no tiene Xcode.
La app de Android hace lo mismo y sí compila y pasa sus tests; si algo falla
acá, lo más probable es un detalle de Swift, no de la lógica.

## Generar y correr

```sh
brew install xcodegen
cd ios
xcodegen                 # genera Sintetica.xcodeproj desde project.yml
open Sintetica.xcodeproj
```

1. En el target **Sintetica** › Signing & Capabilities, elegí tu equipo (o
   completá `DEVELOPMENT_TEAM` en `project.yml` y volvé a correr `xcodegen`).
2. **Product › Test** (⌘U) en un simulador: corre los tests de protocolo con
   `shared/protocolo/vectores.json`, el editor de config y el núcleo C++ desde Swift.
3. Corré la app en el simulador y tocá **Probar sin bicho**: es el núcleo
   corriendo en el celu, sin Bluetooth.
4. Con la placa grabada: corré la app en tu iPhone (el simulador no tiene
   Bluetooth), **Vincular un bicho**, escaneá el QR de la pantalla.

## Qué chequear

- [ ] `xcodegen` genera el proyecto sin errores.
- [ ] Compila (el núcleo C++ entra por `Sintetica/Nucleo/Puente.h`).
- [ ] Los tests pasan.
- [ ] Las fuentes se ven en Fraunces y JetBrains Mono (no en la del sistema).
- [ ] "Probar sin bicho": la cara parpadea, las caras cambian, guardar funciona.
- [ ] Con la placa: vincular, cerrar la app, volver a entrar (se autentica solo).
- [ ] Mantener el botón del bicho 5 s, vincular un segundo celu.

Si algo no compila, mandame el error tal cual lo muestra Xcode.
