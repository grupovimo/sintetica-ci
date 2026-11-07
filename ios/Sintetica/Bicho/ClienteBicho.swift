import Foundation
import Observation

enum ErrorBicho: LocalizedError {
    case rechazo(String)
    case tiempo
    var errorDescription: String? {
        switch self {
        case let .rechazo(m): m
        case .tiempo: "El bicho no contestó a tiempo."
        }
    }
}

/// Corre `operacion` con un límite de tiempo.
@MainActor
func conTiempo<T: Sendable>(_ segundos: Double, _ operacion: @escaping @MainActor () async throws -> T) async throws -> T {
    try await withThrowingTaskGroup(of: T.self) { g in
        g.addTask { try await operacion() }
        g.addTask { try await Task.sleep(for: .seconds(segundos)); throw ErrorBicho.tiempo }
        let r = try await g.next()!
        g.cancelAll()
        return r
    }
}

/// Habla con un bicho por un transporte: comandos, estado y configuración.
@MainActor
@Observable
final class ClienteBicho {
    private(set) var estado: Estado?
    @ObservationIgnored let transporte: Transporte
    @ObservationIgnored private var escucha: Task<Void, Never>?

    init(_ transporte: Transporte) {
        self.transporte = transporte
        let flujo = transporte.difusor.escuchar()
        escucha = Task { [weak self] in
            for await (car, b) in flujo where car == .estado {
                if let e = try? leerEstado(b) { self?.estado = e }
            }
        }
    }

    func conectar() async throws {
        try await transporte.conectar()
        try await actualizarEstado()
    }

    @discardableResult
    func actualizarEstado() async throws -> Estado {
        let e = try leerEstado(try await transporte.leerEstado())
        estado = e
        return e
    }

    /// Escribe y espera la respuesta a esa misma operación.
    private func pedir(_ car: Car, _ datos: [UInt8]) async throws -> Respuesta {
        let op = datos[0] & 0x7F
        let flujo = transporte.difusor.escuchar()
        try await transporte.escribir(car, datos)
        return try await conTiempo(5) {
            for await (c, b) in flujo where c == car {
                let r = leerRespuesta(b)
                if r.op == op { return r }
            }
            throw ErrorBicho.tiempo
        }
    }

    @discardableResult
    private func exigir(_ car: Car, _ datos: [UInt8]) async throws -> Respuesta {
        let r = try await pedir(car, datos)
        guard r.ok else { throw ErrorBicho.rechazo(r.mensaje) }
        return r
    }

    /// Vincula con el código del QR y devuelve la clave que hay que guardar.
    func vincular(_ codigo: String) async throws -> [UInt8] {
        let r = try await exigir(.comando, Comando.vincular(codigo))
        try await actualizarEstado()
        guard case let .resultado(_, _, _, clave?) = r else { throw ErrorBicho.rechazo("El bicho no mandó la clave.") }
        return clave
    }

    func autenticar(_ clave: [UInt8]) async throws {
        try await exigir(.comando, Comando.autenticar(clave))
        try await actualizarEstado()
    }

    func fijarExpresion(_ nombre: String?) async throws { try await exigir(.comando, Comando.expresion(nombre ?? "")); try await actualizarEstado() }
    func cambiarNombre(_ nombre: String) async throws { try await exigir(.comando, Comando.nombre(nombre)); try await actualizarEstado() }
    func evento(_ nombre: String) async throws { try await exigir(.comando, Comando.evento(nombre)) }
    func desvincular() async throws { try await exigir(.comando, Comando.desvincular()) }

    /// Pide la configuración completa (JSON).
    func pedirConfig() async throws -> String {
        let flujo = transporte.difusor.escuchar()
        try await transporte.escribir(.config, Comando.pedirConfig())
        return try await conTiempo(10) {
            var armador = ArmadorDeConfig()
            for await (c, b) in flujo where c == .config {
                let r = leerRespuesta(b)
                if !r.ok { throw ErrorBicho.rechazo(r.mensaje) }
                if let texto = try armador.agregar(r) { return texto }
            }
            throw ErrorBicho.tiempo
        }
    }

    /// Manda una configuración entera; si el bicho la rechaza, explica por qué.
    func mandarConfig(_ json: String) async throws {
        let flujo = transporte.difusor.escuchar()
        for e in escriturasDeConfig(json) { try await transporte.escribir(.config, e) }
        let r: Respuesta = try await conTiempo(10) {
            for await (c, b) in flujo where c == .config {
                let r = leerRespuesta(b)
                if r.op == Op.configFin || !r.ok { return r }
            }
            throw ErrorBicho.tiempo
        }
        guard r.ok else { throw ErrorBicho.rechazo("El bicho no aceptó la configuración: \(r.mensaje)") }
        try await actualizarEstado()
    }

    func cerrar() {
        escucha?.cancel()
        transporte.cerrar()
    }
}
