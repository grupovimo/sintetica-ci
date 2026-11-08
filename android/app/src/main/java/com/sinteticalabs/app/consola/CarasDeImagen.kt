package com.sinteticalabs.app.consola

import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.put

/**
 * Las caras de imagen (docs/motor-de-vida.md, "Caras de imagen"): los ojos y
 * la boca son sprites (0 a 11) que el motor anima como a cualquier cara. Igual
 * que web/src/lib/formato.ts y routes/robots/[id]/expresiones/DeImagen.svelte.
 */
object CarasDeImagen {
    const val MAX_SPRITES = 12
    val PARTES = listOf("ojo", "ojo_cerrado", "boca", "ojo_der", "ojo_der_cerrado")
    /** Las que se editan en la consola, con su rótulo. */
    val PARTES_EDITABLES = listOf("ojo" to "Ojo", "ojo_cerrado" to "Cerrado", "boca" to "Boca")
    /** clave, rótulo, mínimo, máximo, de fábrica. */
    val PERILLAS = listOf(
        Perilla("tamano", "Tamaño", 0.08, 0.8, 0.36), Perilla("sep", "Separación", 0.15, 0.45, 0.3),
        Perilla("ey", "Altura", -0.25, 0.25, 0.0), Perilla("boca_y", "Boca", -0.7, 0.3, -0.36),
    )

    data class Perilla(val clave: String, val nombre: String, val min: Double, val max: Double, val defecto: Double)

    fun esImagen(e: JsonObject?) = e?.obj("sprites") != null

    fun sprites(e: JsonObject?): Map<String, Int> =
        e?.obj("sprites").orEmpty().mapNotNull { (k, v) -> (v as? JsonPrimitive)?.intOrNull?.let { k to it } }.toMap()

    fun deImagen(c: JsonObject): Map<String, JsonObject> =
        c.obj("expresiones").orEmpty().mapNotNull { (k, v) -> (v as? JsonObject)?.takeIf { esImagen(it) }?.let { k to it } }.toMap()

    /** Los sprites que usa cada cara de imagen de la config (menos la que se está editando). */
    fun enUso(c: JsonObject, menos: String = ""): Set<Int> =
        deImagen(c).filterKeys { it != menos }.values.flatMap { sprites(it).values }.toSet()

    /** Un sprite libre: ni de otra cara de imagen ni de otra parte de esta. null si no hay lugar. */
    fun libre(c: JsonObject, inicial: String, doc: JsonObject): Int? {
        val usados = enUso(c, inicial) + sprites(doc).values
        return (0 until MAX_SPRITES).firstOrNull { it !in usados }
    }

    fun nueva(): JsonObject = buildJsonObject { put("sprites", JsonObject(emptyMap())); put("tamano", 0.4) }

    fun conParte(doc: JsonObject, parte: String, sprite: Int?): JsonObject =
        doc.en("sprites") { s -> if (sprite == null) s.sin(parte) else s.con(parte, sprite) }

    fun valor(doc: JsonObject, p: Perilla) = doc.numero(p.clave) ?: p.defecto

    /** Las perillas: en el de fábrica no se escribe (salvo el tamaño). */
    fun conPerilla(doc: JsonObject, p: Perilla, v: Double): JsonObject =
        if (Math.abs(v - p.defecto) < 0.0005 && p.clave != "tamano") doc.sin(p.clave) else doc.con(p.clave, redondo(v, 3))

    /** La cara de siempre ("cara.expresion"); "" = la de fábrica. */
    fun siempre(c: JsonObject) = c.obj("cara")?.texto("expresion").orEmpty()

    fun ponerSiempre(c: JsonObject, n: String): JsonObject = c.en("cara") { o ->
        val r = if (n.isEmpty()) o.sin("expresion") else o.con("expresion", n)
        if (r.isEmpty()) null else r
    }

    /** La config con esta cara guardada (renombrándola si cambió el nombre) y, si va, de cara de siempre. */
    fun guardar(c: JsonObject, inicial: String, nombre: String, doc: JsonObject, deSiempre: Boolean): JsonObject {
        var ex = c.obj("expresiones") ?: VACIO
        if (inicial.isNotEmpty() && inicial != nombre) ex = ex.sin(inicial)
        var r = c.con("expresiones", ex.con(nombre, doc)).con("version", 2)
        val actual = siempre(r)
        r = if (deSiempre) ponerSiempre(r, nombre) else if (actual == inicial || actual == nombre) ponerSiempre(r, "") else r
        return r
    }

    fun borrar(c: JsonObject, inicial: String): JsonObject {
        var r = Lab.ponerExpresion(c, inicial, null)
        if (siempre(r) == inicial) r = ponerSiempre(r, "")
        return r
    }

    /** Los sprites que dejó de usar la cara (y no usa otra): para borrarlos. */
    fun sobrantes(antes: JsonObject?, despues: JsonObject): Set<Int> = sprites(antes).values.toSet() - enUso(despues)

    /** La vista previa: la config con esta cara como la de siempre (si ya tiene ojo). */
    fun vista(c: JsonObject, nombre: String, doc: JsonObject): JsonObject {
        if (sprites(doc)["ojo"] == null) return c.con("version", 2)
        val n = if (nombreValido(nombre)) nombre else "vista-previa"
        return ponerSiempre(Lab.ponerExpresion(c, n, doc), n).con("version", 2)
    }
}
