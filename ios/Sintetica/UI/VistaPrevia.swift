import SwiftUI

/// La cara del bicho dibujada por el mismo núcleo C++ que corre en la placa,
/// a 640 × 344 (el doble de la pantalla real, con la misma geometría).
struct VistaPrevia: View {
    let descripcion: String
    let alApretar: (Bool) -> Void
    @State private var apretado = false

    var body: some View {
        TimelineView(.animation) { _ in
            let imagen = cuadro()
            Group {
                if let imagen { Image(decorative: imagen, scale: 1).resizable().interpolation(.high) } else { Color.pantalla }
            }
            .aspectRatio(640.0 / 344.0, contentMode: .fit)
            .clipShape(RoundedRectangle(cornerRadius: 10))
        }
        .padding(14)
        .background(Color.carcasa, in: RoundedRectangle(cornerRadius: 24))
        .accessibilityLabel(descripcion)
        .gesture(DragGesture(minimumDistance: 0)
            .onChanged { _ in if !apretado { apretado = true; alApretar(true) } }
            .onEnded { _ in apretado = false; alApretar(false) })
    }

    private func cuadro() -> CGImage? {
        Nucleo.tick()
        return Nucleo.imagen()
    }
}
