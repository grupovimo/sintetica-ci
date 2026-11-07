import SwiftUI

struct Inicio: View {
    @Binding var ruta: [Ruta]
    private var vinculados = Vinculados.compartidos

    init(ruta: Binding<[Ruta]>) { _ruta = ruta }

    var body: some View {
        VStack(alignment: .leading, spacing: 24) {
            VStack(alignment: .leading, spacing: 4) {
                Etiqueta("Sintética Labs")
                Text(vinculados.todos.isEmpty ? "Hola." : "Tus bichos").font(Tipos.display).foregroundStyle(Color.tinta)
            }
            if vinculados.todos.isEmpty {
                Text("Prendé tu bicho: en la pantalla va a aparecer un QR. Escanealo para vincularlo con este celu.")
                    .font(Tipos.cuerpo).foregroundStyle(Color.tintaSuave)
            }
            ScrollView {
                VStack(spacing: 12) {
                    ForEach(vinculados.todos) { b in
                        Button { ruta.append(.bicho(b.id)) } label: {
                            VStack(alignment: .leading, spacing: 4) {
                                Text("UNIDAD #\(b.serie.isEmpty ? "—" : b.serie)").font(Tipos.serie).foregroundStyle(Color.naranjaProfundo)
                                Text(b.nombre).font(Tipos.titulo).foregroundStyle(Color.tinta)
                                Text(b.id).font(Tipos.dato).foregroundStyle(Color.tintaSuave)
                            }
                            .frame(maxWidth: .infinity, alignment: .leading).padding(20)
                            .background(Color.papelAlto, in: RoundedRectangle(cornerRadius: 24))
                            .overlay(RoundedRectangle(cornerRadius: 24).stroke(Color.linea, lineWidth: 1))
                        }
                    }
                }
            }
            BotonPrimario(texto: "Vincular un bicho") { ruta.append(.escanear) }
            BotonSecundario(texto: "Probar sin bicho") { ruta.append(.bicho(ID_DEMO)) }
        }
        .padding(24)
        .background(Color.papel.ignoresSafeArea())
        .toolbar(.hidden, for: .navigationBar)
    }
}
