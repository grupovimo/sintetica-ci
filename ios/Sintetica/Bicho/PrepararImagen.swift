import UIKit

/// Recorta y escala una imagen a la pantalla del bicho, y la codifica en QOI.
enum PrepararImagen {
    /// Cubre la pantalla centrada (como un fondo de pantalla); si es redonda, fuera del círculo va negro.
    static func recortar(_ fuente: UIImage, ancho: Int, alto: Int, redonda: Bool) -> UIImage {
        let formato = UIGraphicsImageRendererFormat()
        formato.scale = 1
        formato.opaque = true
        let tam = CGSize(width: ancho, height: alto)
        return UIGraphicsImageRenderer(size: tam, format: formato).image { ctx in
            UIColor(red: 11 / 255, green: 10 / 255, blue: 9 / 255, alpha: 1).setFill()
            ctx.fill(CGRect(origin: .zero, size: tam))
            if redonda { UIBezierPath(ovalIn: CGRect(origin: .zero, size: tam)).addClip() }
            let k = max(tam.width / fuente.size.width, tam.height / fuente.size.height)
            let w = fuente.size.width * k, h = fuente.size.height * k
            fuente.draw(in: CGRect(x: (tam.width - w) / 2, y: (tam.height - h) / 2, width: w, height: h))
        }
    }

    static func qoi(_ imagen: UIImage) -> [UInt8] {
        guard let cg = imagen.cgImage else { return [] }
        let w = cg.width, h = cg.height
        var rgba = [UInt8](repeating: 0, count: w * h * 4)
        let ctx = CGContext(data: &rgba, width: w, height: h, bitsPerComponent: 8, bytesPerRow: w * 4,
                            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue)
        ctx?.draw(cg, in: CGRect(x: 0, y: 0, width: w, height: h))
        var rgb = [UInt8](repeating: 0, count: w * h * 3)
        for i in 0 ..< w * h {
            rgb[i * 3] = rgba[i * 4]
            rgb[i * 3 + 1] = rgba[i * 4 + 1]
            rgb[i * 3 + 2] = rgba[i * 4 + 2]
        }
        return Qoi.codificar(rgb, ancho: w, alto: h)
    }
}

/// Las imágenes que este celu le mandó a cada bicho (el bicho no las devuelve):
/// la que se mandó, una al doble para la vista previa y una miniatura.
enum CacheImagenes {
    private static var raiz: URL {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("imagenes")
    }

    private static func url(_ id: String, _ espacio: Int, _ sufijo: String) -> URL {
        raiz.appendingPathComponent(id).appendingPathComponent("\(espacio)\(sufijo)")
    }

    static func guardar(_ id: String, _ espacio: Int, mandada: [UInt8], vistaPrevia: [UInt8], miniatura: UIImage) {
        let carpeta = raiz.appendingPathComponent(id)
        try? FileManager.default.createDirectory(at: carpeta, withIntermediateDirectories: true)
        try? Data(mandada).write(to: url(id, espacio, ".qoi"))
        try? Data(vistaPrevia).write(to: url(id, espacio, "-vista.qoi"))
        try? miniatura.pngData()?.write(to: url(id, espacio, ".png"))
    }

    static func mandada(_ id: String, _ espacio: Int) -> [UInt8]? { (try? Data(contentsOf: url(id, espacio, ".qoi"))).map { Array($0) } }
    static func vistaPrevia(_ id: String, _ espacio: Int) -> [UInt8]? { (try? Data(contentsOf: url(id, espacio, "-vista.qoi"))).map { Array($0) } }
    static func miniatura(_ id: String, _ espacio: Int) -> UIImage? { UIImage(contentsOfFile: url(id, espacio, ".png").path) }

    static func borrar(_ id: String, _ espacio: Int) {
        for s in [".qoi", "-vista.qoi", ".png"] { try? FileManager.default.removeItem(at: url(id, espacio, s)) }
    }
}
