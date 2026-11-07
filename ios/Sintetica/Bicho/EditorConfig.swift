import Foundation

/// Lo que la app deja editar de la configuración (shared/formato/config.schema.json).
struct Ajustes: Equatable {
    var nombre = "bicho"
    var colorOjos: String? = nil   // "#RRGGBB"; nil = el naranja de la marca
    var tamano: Double = 1         // 0,5 a 1,5
    var energia: Double = 0.5      // 0 a 1
    var timidez: Double = 0.3      // 0 a 1
    var suenoDesde: String? = nil  // "HH:MM"
    var suenoHasta: String? = nil
}

/// Lee y escribe los ajustes sobre el JSON completo de la config, sin tocar lo
/// que la app no edita (expresiones propias, reglas, modo).
enum EditorConfig {
    private static func objeto(_ texto: String) -> [String: Any] {
        (try? JSONSerialization.jsonObject(with: Data(texto.utf8))) as? [String: Any] ?? [:]
    }

    static func leer(_ texto: String) -> Ajustes {
        let raiz = objeto(texto)
        let cara = raiz["cara"] as? [String: Any] ?? [:]
        let per = raiz["personalidad"] as? [String: Any] ?? [:]
        let sueno = (per["sueno"] as? String)?.split(separator: "-").map(String.init)
        return Ajustes(
            nombre: raiz["nombre"] as? String ?? "bicho",
            colorOjos: cara["color"] as? String,
            tamano: (cara["tamano"] as? NSNumber)?.doubleValue ?? 1,
            energia: (per["energia"] as? NSNumber)?.doubleValue ?? 0.5,
            timidez: (per["timidez"] as? NSNumber)?.doubleValue ?? 0.3,
            suenoDesde: sueno?.first, suenoHasta: sueno?.dropFirst().first
        )
    }

    static func escribir(_ texto: String, _ a: Ajustes) -> String {
        var raiz = objeto(texto)
        let redondo = { (d: Double) in (d * 100).rounded() / 100 }
        raiz["version"] = 1
        raiz["nombre"] = a.nombre
        var cara = raiz["cara"] as? [String: Any] ?? [:]
        if let c = a.colorOjos { cara["color"] = c } else { cara.removeValue(forKey: "color") }
        cara["tamano"] = redondo(a.tamano)
        raiz["cara"] = cara
        var per = raiz["personalidad"] as? [String: Any] ?? [:]
        per["energia"] = redondo(a.energia)
        per["timidez"] = redondo(a.timidez)
        if let d = a.suenoDesde, let h = a.suenoHasta { per["sueno"] = "\(d)-\(h)" } else { per.removeValue(forKey: "sueno") }
        raiz["personalidad"] = per
        let datos = (try? JSONSerialization.data(withJSONObject: raiz, options: [.sortedKeys, .withoutEscapingSlashes])) ?? Data("{\"version\":1}".utf8)
        return String(decoding: datos, as: UTF8.self)
    }

    static func hhmmValido(_ s: String) -> Bool { s.wholeMatch(of: /([01][0-9]|2[0-3]):[0-5][0-9]/) != nil }
}
