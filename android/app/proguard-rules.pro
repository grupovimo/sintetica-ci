# El puente JNI busca estos métodos por nombre.
-keep class com.sinteticalabs.app.nucleo.Anfitrion { *; }
-keepclasseswithmembernames class com.sinteticalabs.app.nucleo.Nucleo { native <methods>; }
