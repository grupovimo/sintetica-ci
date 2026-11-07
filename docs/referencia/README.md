# Referencias

- `motor-de-caras-v0/index.html`: el prototipo "Motor de caras v0" (WebGL) que la
  especificación del motor de vida usa como referencia viva. Se abre directo en
  el navegador. Sus parámetros, sus SDF y sus resortes son los que portó
  `firmware/nucleo/rostro.cpp`; la diferencia es que el núcleo los dibuja en la
  CPU (por bloques, sin GPU) y con la misma matemática en todos lados.
