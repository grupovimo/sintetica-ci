import Observation
import PhotosUI
import SwiftUI

@MainActor
@Observable
final class ModeloBicho {
    enum Conexion: Equatable { case conectando, lista, error(String, revincular: Bool) }

    let id: String
    var conexion: Conexion = .conectando
    var ajustes = Ajustes()
    var guardando = false
    var aviso: String?
    var expresiones: [String] = []
    var espacios: [EspacioImagen] = []
    var miniaturas: [Int: UIImage] = [:]
    var progreso: Double?
    /// La pantalla del bicho (la del estado v3); la vista previa va al doble.
    private(set) var pantalla = Pantalla()
    private(set) var cliente: ClienteBicho?
    @ObservationIgnored private var configTexto = "{\"version\":1}"
    @ObservationIgnored private var espejo: Task<Void, Never>?
    /// El núcleo guarda al anfitrión con una referencia débil: el modelo lo mantiene vivo.
    @ObservationIgnored private let anfitrionVistaPrevia = AnfitrionEnMemoria()
    var esDemo: Bool { id.hasPrefix(ID_DEMO) }

    init(id: String) { self.id = id }

    func conectar() async {
        conexion = .conectando
        cliente?.cerrar()
        espejo?.cancel()
        do {
            let c: ClienteBicho
            if esDemo {
                c = ClienteBicho(TransporteVirtual(broche: id == ID_DEMO_BROCHE))
                try await c.conectar()
                pantalla = c.estado?.pantalla ?? pantalla
            } else {
                guard let v = Vinculados.compartidos.buscar(id) else { throw ErrorBicho.rechazo("Este bicho no está vinculado.") }
                let p = try await Central.compartida.buscar(id: id)
                c = ClienteBicho(TransporteBle(p))
                try await c.conectar()
                do {
                    try await c.autenticar(v.clave)
                } catch {
                    c.cerrar()
                    conexion = .error("Este bicho ya no reconoce a este celu. Vinculalo de nuevo.", revincular: true)
                    return
                }
                // La vista previa: un núcleo local con la misma config, al doble de la pantalla.
                pantalla = c.estado?.pantalla ?? pantalla
                Nucleo.iniciar(ancho: pantalla.ancho * 2, alto: pantalla.alto * 2, identificador: id,
                               anfitrion: anfitrionVistaPrevia, redonda: pantalla.redonda)
                Nucleo.vistaPrevia()
            }
            cliente = c
            configTexto = try await c.pedirConfig()
            ajustes = EditorConfig.leer(configTexto)
            if !esDemo { aplicarEnVistaPrevia(configTexto) }
            expresiones = Nucleo.expresiones
            await refrescarImagenes()
            conexion = .lista
            // Copiar en la vista previa la expresión que muestra el bicho.
            if !esDemo {
                espejo = Task { [weak self] in
                    var ultima = ""
                    while !Task.isCancelled {
                        if let e = self?.cliente?.estado?.expresion, e != ultima {
                            ultima = e
                            Nucleo.bleEscribir(Car.comando.rawValue, Data(Comando.expresion(e)))
                        }
                        try? await Task.sleep(for: .milliseconds(100))
                    }
                }
            }
        } catch {
            conexion = .error(error.localizedDescription, revincular: false)
        }
    }

    /// Los espacios ocupados; la vista previa recibe las que este celu tiene guardadas.
    func refrescarImagenes() async {
        guard let c = cliente else { return }
        espacios = (try? await c.listarImagenes()) ?? []
        var minis: [Int: UIImage] = [:]
        for e in espacios {
            guard let mandada = CacheImagenes.mandada(id, e.espacio), crc32(mandada) == e.crc else { continue }  // otro celu la cambió
            if !esDemo, let vista = CacheImagenes.vistaPrevia(id, e.espacio) {
                for w in escriturasDeImagen(e.espacio, vista, tamanoBloque: 512) { Nucleo.bleEscribir(Car.imagen.rawValue, Data(w)) }
            }
            minis[e.espacio] = CacheImagenes.miniatura(id, e.espacio)
        }
        miniaturas = minis
    }

    func subirImagen(_ espacio: Int, _ fuente: UIImage) async {
        guard let c = cliente else { return }
        progreso = 0
        defer { progreso = nil }
        let p = pantalla
        let recortada = PrepararImagen.recortar(fuente, ancho: p.ancho, alto: p.alto, redonda: p.redonda)
        let qoi = PrepararImagen.qoi(recortada)
        // En el modo demo el núcleo ya está al doble: la misma imagen sirve para las dos.
        let vista = esDemo ? qoi : PrepararImagen.qoi(PrepararImagen.recortar(fuente, ancho: p.ancho * 2, alto: p.alto * 2, redonda: p.redonda))
        do {
            try await c.subirImagen(espacio, qoi) { [weak self] v in self?.progreso = v }
            CacheImagenes.guardar(id, espacio, mandada: qoi, vistaPrevia: vista, miniatura: recortada)
            await refrescarImagenes()
            aviso = "Imagen guardada en el espacio \(espacio)."
        } catch {
            aviso = error.localizedDescription
        }
    }

    func borrarImagen(_ espacio: Int) async {
        do { try await cliente?.borrarImagen(espacio) } catch { aviso = error.localizedDescription }
        CacheImagenes.borrar(id, espacio)
        if !esDemo { Nucleo.bleEscribir(Car.imagen.rawValue, Data(ComandoImagen.borrar(espacio))) }
        await refrescarImagenes()
    }

    private func aplicarEnVistaPrevia(_ json: String) {
        for e in escriturasDeConfig(json, tamanoBloque: 512) { Nucleo.bleEscribir(Car.config.rawValue, Data(e)) }
    }

    /// Cambia los ajustes en la vista previa; se mandan al bicho con guardar().
    func editar(_ a: Ajustes) {
        ajustes = a
        if !esDemo { aplicarEnVistaPrevia(EditorConfig.escribir(configTexto, a)) }
    }

    func guardar() async {
        guard let c = cliente else { return }
        guardando = true
        defer { guardando = false }
        do {
            let nueva = EditorConfig.escribir(configTexto, ajustes)
            try await c.mandarConfig(nueva)
            configTexto = nueva
            if !esDemo, var v = Vinculados.compartidos.buscar(id) {
                v.nombre = ajustes.nombre
                Vinculados.compartidos.guardar(v)
            }
            aviso = "Guardado en el bicho."
        } catch {
            aviso = error.localizedDescription
        }
    }

    func fijarExpresion(_ nombre: String?) async {
        do { try await cliente?.fijarExpresion(nombre) } catch { aviso = error.localizedDescription }
    }

    func tocar(_ apretado: Bool) {
        if esDemo { Nucleo.tacto(apretado) } else if apretado { Task { try? await cliente?.evento("tacto") } }
    }

    func desvincular() async {
        try? await cliente?.desvincular()
        if !esDemo { Vinculados.compartidos.olvidar(id) }
        cerrar()
    }

    func cerrar() {
        espejo?.cancel()
        cliente?.cerrar()
    }
}

private let COLORES_OJOS: [(String?, String)] = [
    (nil, "Naranja"), ("#F5F2EC", "Hueso"), ("#FFD23F", "Amarillo"), ("#7BD389", "Verde"), ("#5AC8FA", "Celeste"), ("#FF8FB1", "Rosa"),
]

private func colorDeHex(_ hex: String?) -> Color {
    guard let hex, let v = UInt32(hex.dropFirst(), radix: 16) else { return Color(hex: 0xFF6A33) }
    return Color(hex: v)
}

@MainActor
struct BichoVista: View {
    @Binding var ruta: [Ruta]
    @State private var modelo: ModeloBicho
    @State private var confirmar = false

    init(id: String, ruta: Binding<[Ruta]>) {
        _ruta = ruta
        _modelo = State(initialValue: ModeloBicho(id: id))
    }

    var body: some View {
        Group {
            switch modelo.conexion {
            case .conectando:
                VStack(alignment: .leading, spacing: 20) {
                    Etiqueta(modelo.esDemo ? "Bicho virtual" : modelo.id)
                    Text(modelo.esDemo ? "Despertando al bicho virtual…" : "Buscando el bicho…").font(Tipos.display).foregroundStyle(Color.tinta)
                    ProgressView().tint(Color.naranja)
                    BotonSecundario(texto: "Volver") { ruta = [] }
                    Spacer()
                }
                .padding(24)
            case let .error(mensaje, revincular):
                VStack(alignment: .leading, spacing: 20) {
                    Etiqueta(modelo.id)
                    Text("No me pude conectar.").font(Tipos.display).foregroundStyle(Color.tinta)
                    Aviso(texto: mensaje)
                    if revincular {
                        BotonPrimario(texto: "Vincular de nuevo") { Vinculados.compartidos.olvidar(modelo.id); ruta = [.escanear] }
                    } else {
                        BotonPrimario(texto: "Probar de nuevo") { Task { await modelo.conectar() } }
                    }
                    BotonSecundario(texto: "Volver") { ruta = [] }
                    Spacer()
                }
                .padding(24)
            case .lista:
                contenido
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .background(Color.papel.ignoresSafeArea())
        .navigationBarBackButtonHidden()
        .task { await modelo.conectar() }
        .onDisappear { modelo.cerrar() }
        .confirmationDialog(modelo.esDemo ? "¿Reiniciar el bicho virtual?" : "¿Desvincular?", isPresented: $confirmar, titleVisibility: .visible) {
            Button(modelo.esDemo ? "Reiniciar" : "Desvincular", role: .destructive) { Task { await modelo.desvincular(); ruta = [] } }
        } message: {
            Text("El bicho olvida a todos los celus vinculados y vuelve a mostrar su QR.")
        }
    }

    private var contenido: some View {
        let e = modelo.cliente?.estado
        return ScrollView {
            VStack(alignment: .leading, spacing: 28) {
                HStack {
                    Button { ruta = [] } label: { Text("← BICHOS").font(Tipos.boton).foregroundStyle(Color.tintaSuave) }
                    Spacer()
                    Etiqueta(modelo.esDemo ? "Bicho virtual" : "Conectado")
                }
                VStack(alignment: .leading, spacing: 4) {
                    Text("UNIDAD #\((e?.serie ?? "").isEmpty ? "—" : e!.serie)").font(Tipos.serie).foregroundStyle(Color.naranjaProfundo)
                    Text(e?.nombre ?? modelo.ajustes.nombre).font(Tipos.display).foregroundStyle(Color.tinta)
                }
                VistaPrevia(descripcion: "Cara de \(e?.nombre ?? "el bicho"): \(e?.expresion ?? "")", redonda: modelo.pantalla.redonda) { modelo.tocar($0) }

                VStack(spacing: 0) {
                    FilaDato(etiqueta: "Expresión", valor: (e?.expresion ?? "—") + ((e?.fija ?? false) ? " (fija)" : ""))
                    FilaDato(etiqueta: "Humor", valor: e.map { String(format: "%+.2f", $0.humor) } ?? "—")
                    FilaDato(etiqueta: "Batería", valor: e?.bateria.map { "\($0)%" } ?? "sin batería")
                    FilaDato(etiqueta: "Estado", valor: (e?.dormido ?? false) ? "durmiendo" : "despierto")
                }

                Seccion(titulo: "Caras") {
                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 104), spacing: 8)], alignment: .leading, spacing: 8) {
                        Chip(texto: "Automático", elegido: e?.fija == false) { Task { await modelo.fijarExpresion(nil) } }
                        ForEach(modelo.expresiones, id: \.self) { x in
                            Chip(texto: x, elegido: e?.fija == true && e?.expresion == x) { Task { await modelo.fijarExpresion(x) } }
                        }
                    }
                }

                SeccionImagenes(modelo: modelo)
                SeccionLista(modelo: modelo)

                Seccion(titulo: "Nombre") {
                    TextField("Nombre", text: Binding(
                        get: { modelo.ajustes.nombre },
                        set: { n in if n.utf8.count <= 24 { var a = modelo.ajustes; a.nombre = n; modelo.editar(a) } }
                    ))
                    .font(Tipos.cuerpo).padding(12)
                    .overlay(RoundedRectangle(cornerRadius: 12).stroke(Color.linea, lineWidth: 1))
                }

                Seccion(titulo: "Ojos") {
                    HStack(spacing: 12) {
                        ForEach(COLORES_OJOS, id: \.1) { hex, nombre in
                            let elegido = modelo.ajustes.colorOjos == hex
                            Button { var a = modelo.ajustes; a.colorOjos = hex; modelo.editar(a) } label: {
                                VStack(spacing: 4) {
                                    ZStack {
                                        Circle().fill(Color.pantalla).frame(width: 44, height: 44)
                                            .overlay(Circle().stroke(elegido ? Color.naranja : Color.linea, lineWidth: elegido ? 3 : 1))
                                        RoundedRectangle(cornerRadius: 9).fill(colorDeHex(hex)).frame(width: 18, height: 28)
                                    }
                                    Etiqueta(nombre)
                                }
                            }
                            .accessibilityLabel("Ojos \(nombre)")
                        }
                    }
                    Deslizador(etiqueta: "Tamaño", valor: modelo.ajustes.tamano, rango: 0.5 ... 1.5) { var a = modelo.ajustes; a.tamano = $0; modelo.editar(a) }
                }

                Seccion(titulo: "Personalidad") {
                    Deslizador(etiqueta: "Energía", valor: modelo.ajustes.energia, rango: 0 ... 1) { var a = modelo.ajustes; a.energia = $0; modelo.editar(a) }
                    Deslizador(etiqueta: "Timidez", valor: modelo.ajustes.timidez, rango: 0 ... 1) { var a = modelo.ajustes; a.timidez = $0; modelo.editar(a) }
                    Toggle(isOn: Binding(
                        get: { modelo.ajustes.suenoDesde != nil },
                        set: { si in var a = modelo.ajustes; a.suenoDesde = si ? "23:30" : nil; a.suenoHasta = si ? "08:00" : nil; modelo.editar(a) }
                    )) { Text("Duerme de noche").font(Tipos.cuerpo).foregroundStyle(Color.tinta) }
                        .tint(Color.naranja)
                    if let desde = modelo.ajustes.suenoDesde, let hasta = modelo.ajustes.suenoHasta {
                        HStack(spacing: 12) {
                            CampoHora(etiqueta: "Desde", valor: desde) { var a = modelo.ajustes; a.suenoDesde = $0; modelo.editar(a) }
                            CampoHora(etiqueta: "Hasta", valor: hasta) { var a = modelo.ajustes; a.suenoHasta = $0; modelo.editar(a) }
                        }
                    }
                }

                if let aviso = modelo.aviso { Aviso(texto: aviso).onTapGesture { modelo.aviso = nil } }
                BotonPrimario(texto: modelo.guardando ? "Guardando…" : "Guardar en el bicho", habilitado: !modelo.guardando) {
                    Task { await modelo.guardar() }
                }
                Button { confirmar = true } label: {
                    Text(modelo.esDemo ? "REINICIAR EL BICHO VIRTUAL" : "DESVINCULAR ESTE BICHO").font(Tipos.boton).foregroundStyle(Color.naranjaProfundo)
                        .frame(maxWidth: .infinity)
                }
            }
            .padding(24)
        }
    }
}

private struct Chip: View {
    let texto: String
    let elegido: Bool
    let accion: () -> Void
    var body: some View {
        Button(action: accion) {
            Text(texto).font(Tipos.dato).foregroundStyle(Color.tinta).frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 12).padding(.vertical, 8)
                .background(elegido ? Color.naranjaClaro : Color.papelAlto, in: RoundedRectangle(cornerRadius: 12))
                .overlay(RoundedRectangle(cornerRadius: 12).stroke(elegido ? Color.naranja : Color.linea, lineWidth: 1))
        }
    }
}

private struct Deslizador: View {
    let etiqueta: String
    let valor: Double
    let rango: ClosedRange<Double>
    var texto: ((Double) -> String)? = nil
    let alCambiar: (Double) -> Void
    var body: some View {
        VStack(spacing: 4) {
            HStack {
                Etiqueta(etiqueta)
                Spacer()
                Text(texto?(valor) ?? "\(Int((valor * 100).rounded()))%").font(Tipos.dato).foregroundStyle(Color.tinta)
            }
            Slider(value: Binding(get: { valor }, set: alCambiar), in: rango).tint(Color.naranja)
        }
    }
}

private struct CampoHora: View {
    let etiqueta: String
    let valor: String
    let alCambiar: (String) -> Void
    @State private var texto = ""
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Etiqueta(etiqueta)
            TextField("HH:MM", text: $texto)
                .font(Tipos.dato).keyboardType(.numbersAndPunctuation).padding(12)
                .overlay(RoundedRectangle(cornerRadius: 12).stroke(EditorConfig.hhmmValido(texto) ? Color.linea : Color.naranja, lineWidth: 1))
                .onChange(of: texto) { _, t in if EditorConfig.hhmmValido(t) { alCambiar(t) } }
        }
        .onAppear { texto = valor }
    }
}

private struct SeccionImagenes: View {
    let modelo: ModeloBicho
    @State private var elegido = 0
    @State private var foto: PhotosPickerItem?

    var body: some View {
        let ocupados = Set(modelo.espacios.map(\.espacio))
        let forma = modelo.pantalla.redonda ? AnyShape(Circle()) : AnyShape(RoundedRectangle(cornerRadius: 10))
        Seccion(titulo: "Imágenes") {
            Text("Hasta 8. Se recortan a la forma de la pantalla del bicho.").font(Tipos.chico).foregroundStyle(Color.tintaSuave)
            LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 8), count: 4), spacing: 8) {
                ForEach(0 ..< 8, id: \.self) { i in
                    Button { elegido = i } label: {
                        Color.pantalla
                            .aspectRatio(1, contentMode: .fit)
                            .overlay {
                                if let mini = modelo.miniaturas[i] {
                                    Image(uiImage: mini).resizable().scaledToFill()
                                } else {
                                    Text(ocupados.contains(i) ? "\(i) ·" : "\(i)").font(Tipos.dato).foregroundStyle(Color.tintaSuave)
                                }
                            }
                            .clipShape(forma)
                        .overlay(forma.stroke(elegido == i ? Color.naranja : Color.linea, lineWidth: elegido == i ? 3 : 1))
                    }
                    .accessibilityLabel("Espacio \(i)")
                }
            }
            if let p = modelo.progreso { ProgressView(value: p).tint(Color.naranja) }
            HStack(spacing: 8) {
                PhotosPicker(selection: $foto, matching: .images) {
                    Text("ELEGIR DE LA GALERÍA").font(Tipos.boton).frame(maxWidth: .infinity).frame(height: 48)
                        .foregroundStyle(Color.tinta).overlay(RoundedRectangle(cornerRadius: 12).stroke(Color.linea, lineWidth: 1))
                }
                .disabled(modelo.progreso != nil)
                if ocupados.contains(elegido) {
                    BotonSecundario(texto: "Borrar") { Task { await modelo.borrarImagen(elegido) } }
                }
            }
        }
        .onChange(of: foto) { _, item in
            guard let item else { return }
            Task {
                if let datos = try? await item.loadTransferable(type: Data.self), let imagen = UIImage(data: datos) {
                    await modelo.subirImagen(elegido, imagen)
                }
                foto = nil
            }
        }
    }
}

private struct SeccionLista: View {
    let modelo: ModeloBicho
    @State private var agregando = false

    var body: some View {
        let a = modelo.ajustes
        Seccion(titulo: "Lista") {
            Text(modelo.pantalla.redonda ? "Un toque en el broche pasa a lo siguiente." : "Lo que muestra, en orden; puede rotar solo.")
                .font(Tipos.chico).foregroundStyle(Color.tintaSuave)
            ForEach(Array(a.lista.enumerated()), id: \.offset) { i, item in
                HStack {
                    Text("\(i + 1). " + item.replacingOccurrences(of: "cara:", with: "").replacingOccurrences(of: "imagen:", with: "imagen "))
                        .font(Tipos.dato).foregroundStyle(Color.tinta)
                    Spacer()
                    Button("↑") { var b = a; b.lista.swapAt(i, i - 1); modelo.editar(b) }.disabled(i == 0)
                    Button("↓") { var b = a; b.lista.swapAt(i, i + 1); modelo.editar(b) }.disabled(i == a.lista.count - 1)
                    Button("QUITAR") { var b = a; b.lista.remove(at: i); modelo.editar(b) }.font(Tipos.boton).foregroundStyle(Color.naranjaProfundo)
                }
                .font(Tipos.boton)
            }
            if a.lista.isEmpty {
                Text(modelo.pantalla.redonda ? "Vacía: recorre las 16 caras." : "Vacía: se maneja solo.").font(Tipos.dato).foregroundStyle(Color.tintaSuave)
            }
            if agregando {
                LazyVGrid(columns: [GridItem(.adaptive(minimum: 104), spacing: 8)], alignment: .leading, spacing: 8) {
                    ForEach(modelo.espacios, id: \.espacio) { e in
                        Chip(texto: "imagen \(e.espacio)", elegido: false) { var b = a; b.lista.append("imagen:\(e.espacio)"); modelo.editar(b); agregando = false }
                    }
                    ForEach(modelo.expresiones, id: \.self) { x in
                        Chip(texto: x, elegido: false) { var b = a; b.lista.append("cara:\(x)"); modelo.editar(b); agregando = false }
                    }
                }
            } else if a.lista.count < 16 {
                BotonSecundario(texto: "Agregar") { agregando = true }
            }
            Deslizador(etiqueta: "Rotar sola", valor: Double(a.rotar), rango: 0 ... 60,
                       texto: { $0 < 0.5 ? "no" : "cada \(Int($0.rounded())) min" }) { var b = a; b.rotar = Int($0.rounded()); modelo.editar(b) }
        }
    }
}
