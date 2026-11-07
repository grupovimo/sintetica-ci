import CoreBluetooth
import Foundation

/// Protocolo BLE de SintéticaOS v1.1, del lado de la app. Misma lógica que
/// sim/web/protocolo.js y la app de Android; los tests usan
/// shared/protocolo/vectores.json. Ver docs/protocolo-ble.md.
enum Uuids {
    static let servicio = CBUUID(string: "5157e71c-a000-4b1c-8a5e-000000000001")
    static let config = CBUUID(string: "5157e71c-a000-4b1c-8a5e-000000000002")
    static let comando = CBUUID(string: "5157e71c-a000-4b1c-8a5e-000000000003")
    static let estado = CBUUID(string: "5157e71c-a000-4b1c-8a5e-000000000004")
    static let ota = CBUUID(string: "5157e71c-a000-4b1c-8a5e-000000000005")
    static let imagen = CBUUID(string: "5157e71c-a000-4b1c-8a5e-000000000006")
}

enum Car: Int, CaseIterable, Sendable {
    case config = 1, comando = 2, estado = 3, ota = 4, imagen = 5

    var uuid: CBUUID {
        switch self {
        case .config: Uuids.config
        case .comando: Uuids.comando
        case .estado: Uuids.estado
        case .ota: Uuids.ota
        case .imagen: Uuids.imagen
        }
    }

    init?(uuid: CBUUID) {
        guard let c = Car.allCases.first(where: { $0.uuid == uuid }) else { return nil }
        self = c
    }
}

enum Op {
    static let configInicio: UInt8 = 0x01, configBloque: UInt8 = 0x02, configFin: UInt8 = 0x03, configPedir: UInt8 = 0x04
    static let expresion: UInt8 = 0x10, nombre: UInt8 = 0x11, evento: UInt8 = 0x12
    static let vincular: UInt8 = 0x13, autenticar: UInt8 = 0x14, desvincular: UInt8 = 0x15
    static let imgInicio: UInt8 = 0x20, imgBloque: UInt8 = 0x21, imgFin: UInt8 = 0x22, imgBorrar: UInt8 = 0x23, imgListar: UInt8 = 0x24
    static let respuesta: UInt8 = 0x80, configTrozo: UInt8 = 0x84, configCierre: UInt8 = 0x85
}

let EVENTOS = ["tacto", "doble-toque", "levantado", "sacudida", "boca-abajo", "vinculado", "app-abierta", "despertado"]
let MODOS = ["mascota", "reloj", "pomodoro", "clima", "notificaciones"]

private let MENSAJES: [Int: String] = [
    0: "ok", 1: "muy grande", 2: "fuera de orden", 3: "CRC distinto", 4: "JSON inválido", 5: "config inválida",
    6: "sin inicio", 7: "no existe", 8: "malformado", 9: "no autorizado", 10: "no está mostrando el QR", 11: "imagen inválida",
    255: "no implementado",
]

private func u32(_ v: UInt32) -> [UInt8] { [UInt8(v & 0xFF), UInt8(v >> 8 & 0xFF), UInt8(v >> 16 & 0xFF), UInt8(v >> 24 & 0xFF)] }

func leerU32(_ b: [UInt8], _ i: Int) -> UInt32 {
    UInt32(b[i]) | UInt32(b[i + 1]) << 8 | UInt32(b[i + 2]) << 16 | UInt32(b[i + 3]) << 24
}

func crc32(_ bytes: [UInt8]) -> UInt32 {
    var c: UInt32 = 0xFFFF_FFFF
    for b in bytes {
        c ^= UInt32(b)
        for _ in 0 ..< 8 { c = (c & 1) != 0 ? (c >> 1) ^ 0xEDB8_8320 : c >> 1 }
    }
    return c ^ 0xFFFF_FFFF
}

enum Comando {
    static func expresion(_ nombre: String = "") -> [UInt8] { [Op.expresion] + Array(nombre.utf8) }
    static func nombre(_ nombre: String) -> [UInt8] { [Op.nombre] + Array(nombre.utf8) }
    static func evento(_ nombre: String) -> [UInt8] { [Op.evento, UInt8(EVENTOS.firstIndex(of: nombre) ?? 0)] }
    static func vincular(_ codigo: String) -> [UInt8] { [Op.vincular] + Array(codigo.utf8) }
    static func autenticar(_ clave: [UInt8]) -> [UInt8] { [Op.autenticar] + clave }
    static func desvincular() -> [UInt8] { [Op.desvincular] }
    static func pedirConfig() -> [UInt8] { [Op.configPedir] }
}

/// Una imagen QOI (del tamaño de la pantalla) en las escrituras de la característica Imagen.
func escriturasDeImagen(_ espacio: Int, _ qoi: [UInt8], tamanoBloque: Int = 180) -> [[UInt8]] {
    let porBloque = tamanoBloque - 5
    var salida: [[UInt8]] = [[Op.imgInicio, UInt8(espacio)] + u32(UInt32(qoi.count)) + u32(crc32(qoi))]
    var off = 0
    while off < qoi.count {
        let fin = min(qoi.count, off + porBloque)
        salida.append([Op.imgBloque] + u32(UInt32(off)) + Array(qoi[off ..< fin]))
        off = fin
    }
    salida.append([Op.imgFin])
    return salida
}

enum ComandoImagen {
    static func borrar(_ espacio: Int) -> [UInt8] { [Op.imgBorrar, UInt8(espacio)] }
    static func listar() -> [UInt8] { [Op.imgListar] }
}

struct EspacioImagen: Equatable, Sendable { let espacio: Int; let largo: UInt32; let crc: UInt32 }

/// La respuesta a ComandoImagen.listar(): los espacios ocupados.
func leerListaImagenes(_ b: [UInt8]) -> [EspacioImagen] {
    var l: [EspacioImagen] = []
    var i = 3
    while i + 9 <= b.count {
        l.append(EspacioImagen(espacio: Int(b[i]), largo: leerU32(b, i + 1), crc: leerU32(b, i + 5)))
        i += 9
    }
    return l
}

/// Parte la config (texto JSON) en las escrituras de la característica Config.
func escriturasDeConfig(_ json: String, tamanoBloque: Int = 180) -> [[UInt8]] {
    let bytes = Array(json.utf8)
    let porBloque = tamanoBloque - 5
    var salida: [[UInt8]] = [[Op.configInicio] + u32(UInt32(bytes.count)) + u32(crc32(bytes))]
    var off = 0
    while off < bytes.count {
        let fin = min(bytes.count, off + porBloque)
        salida.append([Op.configBloque] + u32(UInt32(off)) + Array(bytes[off ..< fin]))
        off = fin
    }
    salida.append([Op.configFin])
    return salida
}

enum Respuesta: Equatable {
    case resultado(op: UInt8, codigo: UInt8, detalle: String, clave: [UInt8]?)
    case trozo(desplazamiento: Int, datos: [UInt8])
    case cierre(largo: Int, crc: UInt32)

    var ok: Bool { if case let .resultado(_, codigo, _, _) = self { return codigo == 0 }; return true }
    var op: UInt8? { if case let .resultado(op, _, _, _) = self { return op }; return nil }
    var mensaje: String {
        guard case let .resultado(_, codigo, detalle, _) = self else { return "" }
        return detalle.isEmpty ? (MENSAJES[Int(codigo)] ?? "código \(codigo)") : detalle
    }
}

func leerRespuesta(_ b: [UInt8]) -> Respuesta {
    if b[0] == Op.configTrozo { return .trozo(desplazamiento: Int(leerU32(b, 1)), datos: Array(b[5...])) }
    if b[0] == Op.configCierre { return .cierre(largo: Int(leerU32(b, 1)), crc: leerU32(b, 5)) }
    let op = b[0] & 0x7F
    let codigo = b.count > 1 ? b[1] : 255
    if op == Op.vincular, codigo == 0, b.count >= 18 { return .resultado(op: op, codigo: 0, detalle: "", clave: Array(b[2 ..< 18])) }
    let detalle = b.count > 2 ? String(decoding: b[2...], as: UTF8.self) : ""
    return .resultado(op: op, codigo: codigo, detalle: detalle, clave: nil)
}

enum ErrorProtocolo: LocalizedError {
    case crcDistinto, estadoDesconocido(UInt8)
    var errorDescription: String? {
        switch self {
        case .crcDistinto: "La config llegó con otro CRC."
        case let .estadoDesconocido(v): "Versión de estado desconocida: \(v)."
        }
    }
}

/// Junta la config que el bicho manda en trozos después de pedirla.
struct ArmadorDeConfig {
    private var partes: [(Int, [UInt8])] = []
    mutating func agregar(_ r: Respuesta) throws -> String? {
        switch r {
        case let .trozo(d, datos): partes.append((d, datos)); return nil
        case let .cierre(largo, crc):
            var bytes = [UInt8](repeating: 0, count: largo)
            for (d, datos) in partes where d + datos.count <= largo { bytes.replaceSubrange(d ..< d + datos.count, with: datos) }
            partes.removeAll()
            guard crc32(bytes) == crc else { throw ErrorProtocolo.crcDistinto }
            return String(decoding: bytes, as: UTF8.self)
        default: return nil
        }
    }
}

struct Pantalla: Equatable, Sendable {
    var ancho = 320, alto = 172
    var redonda = false
}

struct Estado: Equatable, Sendable {
    var pantalla = Pantalla()
    var bateria: Int?
    var humor: Float
    var modo: String
    var dormido, fija, emparejando, autenticado: Bool
    var expresion, nombre, serie, id: String
}

func leerEstado(_ b: [UInt8]) throws -> Estado {
    guard let v = b.first, (1 ... 3).contains(v) else { throw ErrorProtocolo.estadoDesconocido(b.first ?? 0) }
    var i = 5
    func campo() -> String {
        let n = Int(b[i])
        let s = String(decoding: b[(i + 1) ..< (i + 1 + n)], as: UTF8.self)
        i += 1 + n
        return s
    }
    let expresion = campo(), nombre = campo()
    let serie = b[0] >= 2 ? campo() : ""
    let id = b[0] >= 2 ? campo() : ""
    let flags = b[4]
    let pantalla = b[0] >= 3
        ? Pantalla(ancho: Int(b[i]) | Int(b[i + 1]) << 8, alto: Int(b[i + 2]) | Int(b[i + 3]) << 8, redonda: b[i + 4] == 1)
        : Pantalla()
    return Estado(
        pantalla: pantalla,
        bateria: b[1] == 255 ? nil : Int(b[1]), humor: Float(Int8(bitPattern: b[2])) / 100,
        modo: Int(b[3]) < MODOS.count ? MODOS[Int(b[3])] : "?",
        dormido: flags & 1 != 0, fija: flags & 2 != 0, emparejando: flags & 4 != 0, autenticado: flags & 8 != 0,
        expresion: expresion, nombre: nombre, serie: serie, id: id
    )
}

struct DatosQr: Equatable, Hashable { let id: String; let codigo: String }

/// El QR del bicho: "SINTETICA:V1/<id de 12>/<código de 6>".
func leerQr(_ texto: String) -> DatosQr? {
    let t = texto.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let m = t.wholeMatch(of: #/SINTETICA:V1\/([0-9A-F]{12})\/([2-9A-HJKMNP-Z]{6})/#) else { return nil }
    return DatosQr(id: String(m.1), codigo: String(m.2))
}

/// El id en los datos de fabricante del anuncio (CoreBluetooth los da completos: 0xFFFF + la MAC).
func idDeAnuncio(_ datos: [UInt8]?) -> String? {
    guard let d = datos, d.count >= 8, d[0] == 0xFF, d[1] == 0xFF else { return nil }
    return d[2 ..< 8].map { String(format: "%02X", $0) }.joined()
}

func hex(_ b: [UInt8]) -> String { b.map { String(format: "%02x", $0) }.joined(separator: " ") }
func deHex(_ s: String) -> [UInt8] { s.split(separator: " ").compactMap { UInt8($0, radix: 16) } }
