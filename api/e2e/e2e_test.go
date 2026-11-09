//go:build e2e

// Prueba de punta a punta contra el docker compose levantado:
//
//	ADMIN_EMAILS=@ejemplo.com docker compose up -d --build
//	go test -tags e2e -count=1 -v ./api/e2e/
//
// Hace lo que haría una persona: entra con el link mágico (lo lee de
// Mailpit), suma el bicho virtual, le cambia la config desde la web y desde
// "otro celu", le manda un comando y una imagen, le cambia algo por BLE (el
// bicho manda y la nube lo trae), y lo actualiza por OTA (una buena y una que
// falla y vuelve atrás).
package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"sintetica/api/internal/protocolo"
	"sintetica/shared/formato"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

var (
	api     = env("E2E_API", "http://localhost:8080")
	mailpit = env("E2E_MAILPIT", "http://localhost:8025")
	virtual = env("E2E_VIRTUAL", "http://localhost:8090")
	email   = env("E2E_EMAIL", fmt.Sprintf("prueba-%d@ejemplo.com", time.Now().Unix()))
)

type cliente struct {
	t      *testing.T
	http   *http.Client
	bearer string
	web    bool
}

func (c *cliente) pedir(metodo, ruta string, cuerpo any, salida any) int {
	c.t.Helper()
	var r io.Reader
	tipo := "application/json"
	switch v := cuerpo.(type) {
	case nil:
	case []byte:
		r = bytes.NewReader(v)
		tipo = "application/octet-stream"
	case *bytes.Buffer:
		r = v
	default:
		b, _ := json.Marshal(v)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(metodo, api+ruta, r)
	req.Header.Set("Content-Type", tipo)
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	if c.web {
		req.Header.Set("X-Sintetica", "1")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", metodo, ruta, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if salida != nil && len(b) > 0 {
		if err := json.Unmarshal(b, salida); err != nil {
			c.t.Fatalf("%s %s: respuesta rara %q", metodo, ruta, b)
		}
	}
	if resp.StatusCode >= 400 {
		c.t.Logf("%s %s → %d %s", metodo, ruta, resp.StatusCode, b)
	}
	return resp.StatusCode
}

// mailDe espera el último mail a `para` que llegó después de `desde`.
func mailDe(t *testing.T, para string, desde time.Time, contiene string) string {
	t.Helper()
	for fin := time.Now().Add(15 * time.Second); time.Now().Before(fin); time.Sleep(300 * time.Millisecond) {
		r, err := http.Get(mailpit + "/api/v1/messages?limit=5")
		if err != nil {
			continue
		}
		var l struct {
			Messages []struct {
				ID      string
				Created time.Time
				To      []struct{ Address string }
			}
		}
		json.NewDecoder(r.Body).Decode(&l)
		r.Body.Close()
		for _, m := range l.Messages {
			if len(m.To) > 0 && m.To[0].Address == para && m.Created.After(desde.Add(-2*time.Second)) {
				r, err := http.Get(mailpit + "/api/v1/message/" + m.ID)
				if err != nil {
					continue
				}
				var msg struct{ Text string }
				json.NewDecoder(r.Body).Decode(&msg)
				r.Body.Close()
				if strings.Contains(msg.Text, contiene) {
					return msg.Text
				}
			}
		}
	}
	t.Fatal("no llegó el mail")
	return ""
}

func entrar(t *testing.T, cli string) *cliente {
	t.Helper()
	return entrarComo(t, email, cli)
}

// entrarComo entra con el link mágico (o el código) de ese email.
func entrarComo(t *testing.T, email, cli string) *cliente {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &cliente{t: t, http: &http.Client{Jar: jar, Timeout: 60 * time.Second}, web: cli == "web"}
	desde := time.Now()
	if s := c.pedir("POST", "/v1/sesion/enlace", map[string]string{"email": email, "cliente": cli}, nil); s != http.StatusAccepted {
		t.Fatalf("pedir el link: %d", s)
	}
	texto := mailDe(t, email, desde, "en la "+cli)
	var resp struct {
		Token   string
		Usuario struct{ Email string }
	}
	if cli == "web" {
		m := regexp.MustCompile(`t=([A-Za-z0-9_-]+)`).FindStringSubmatch(texto)
		if m == nil {
			t.Fatalf("el mail no trae el link: %s", texto)
		}
		if s := c.pedir("POST", "/v1/sesion/canjear", map[string]string{"token": m[1]}, &resp); s != 200 {
			t.Fatalf("canjear el link: %d", s)
		}
	} else {
		m := regexp.MustCompile(`código: ([A-Z0-9]{4}-[A-Z0-9]{4})`).FindStringSubmatch(texto)
		if m == nil {
			t.Fatalf("el mail no trae el código: %s", texto)
		}
		if s := c.pedir("POST", "/v1/sesion/canjear", map[string]string{"email": email, "codigo": strings.ToLower(m[1])}, &resp); s != 200 {
			t.Fatalf("canjear el código: %d", s)
		}
		c.bearer = resp.Token
	}
	if resp.Usuario.Email != email {
		t.Fatalf("entré como %q", resp.Usuario.Email)
	}
	return c
}

type infoVirtual struct {
	ID        string
	QR        string
	Version   string
	Visible   string
	Nube      bool
	CrcConfig uint32
	Imagenes  map[string]int64
	Estado    protocolo.Estado
}

func leerVirtual(t *testing.T) infoVirtual {
	t.Helper()
	r, err := http.Get(virtual + "/info")
	if err != nil {
		t.Fatalf("el bicho virtual no responde: %v", err)
	}
	defer r.Body.Close()
	var i infoVirtual
	json.NewDecoder(r.Body).Decode(&i)
	return i
}

func esperar(t *testing.T, que string, plazo time.Duration, cond func() bool) {
	t.Helper()
	for fin := time.Now().Add(plazo); time.Now().Before(fin); time.Sleep(250 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("no pasó a tiempo: %s", que)
}

// qoi arma un QOI de w × h con solo operaciones RGB (el peor caso de tamaño).
func qoi(w, h int, r, g, b byte) []byte {
	out := []byte("qoif")
	out = binary.BigEndian.AppendUint32(out, uint32(w))
	out = binary.BigEndian.AppendUint32(out, uint32(h))
	out = append(out, 3, 0)
	for i := 0; i < w*h; i++ {
		out = append(out, 0xFE, r+byte(i%7), g, b)
	}
	return append(out, 0, 0, 0, 0, 0, 0, 0, 1)
}

func TestDePuntaAPunta(t *testing.T) {
	web := entrar(t, "web")
	celu := entrar(t, "app") // "otro celu", con la misma cuenta

	// --- Sumar el bicho virtual ---
	var virtuales struct {
		Virtuales []struct {
			ID     string
			Sumado bool
		}
	}
	web.pedir("GET", "/v1/bichos/virtuales", nil, &virtuales)
	if len(virtuales.Virtuales) == 0 {
		t.Fatal("no aparecen los bichos virtuales")
	}
	id := leerVirtual(t).ID
	var vista struct {
		ID        string
		Conectado bool
	}
	if s := web.pedir("POST", "/v1/bichos/reclamar-virtual", map[string]string{"id": id}, &vista); s != 200 || vista.ID != id {
		t.Fatalf("sumar el bicho virtual: %d", s)
	}
	esperar(t, "que el bicho se conecte a la nube", 20*time.Second, func() bool { return leerVirtual(t).Nube })
	type vistaBicho struct {
		Conectado bool
		Estado    *protocolo.Estado
		Sincro    struct{ Config, Imagenes string }
	}
	var vb vistaBicho
	esperar(t, "que la API vea su estado", 20*time.Second, func() bool {
		web.pedir("GET", "/v1/bichos/"+id, nil, &vb)
		return vb.Conectado && vb.Estado != nil
	})

	// --- Config desde la web ---
	var cfg map[string]any
	json.Unmarshal(formato.Ejemplo(), &cfg)
	cfg["nombre"] = "Toto"
	var guardada struct {
		Version int
		Crc     uint32
	}
	if s := web.pedir("PUT", "/v1/bichos/"+id+"/config", cfg, &guardada); s != 200 {
		t.Fatalf("guardar la config: %d", s)
	}
	esperar(t, "que la config llegue al bicho", 20*time.Second, func() bool {
		v := leerVirtual(t)
		return v.CrcConfig == guardada.Crc && v.Estado.Nombre == "Toto"
	})
	// Una config inválida no se guarda.
	var e struct{ Error, Detalle string }
	if s := web.pedir("PUT", "/v1/bichos/"+id+"/config", map[string]any{"version": 1, "lista": []string{"animacion:nada"}}, &e); s != 422 {
		t.Fatalf("una config inválida: %d", s)
	}

	// --- "Otro celu": los mismos bytes del protocolo, por la nube ---
	cfg["nombre"] = "Ñandú"
	json2, _ := json.Marshal(cfg)
	var respFin []byte
	for _, m := range protocolo.EscriturasDeConfig(json2, 180) {
		var r struct {
			Respuestas []struct {
				Car   byte
				Datos []byte
			}
		}
		celu.pedir("POST", "/v1/bichos/"+id+"/marcos", map[string]any{"car": protocolo.CarConfig, "datos": m}, &r)
		for _, x := range r.Respuestas {
			respFin = x.Datos
		}
	}
	if len(respFin) < 2 || respFin[0] != protocolo.ConfigFin|protocolo.Respuesta || respFin[1] != protocolo.Ok {
		t.Fatalf("la config por marcos: %x", respFin)
	}
	esperar(t, "que la config del otro celu llegue al bicho", 20*time.Second, func() bool { return leerVirtual(t).Estado.Nombre == "Ñandú" })

	// --- Comandos ---
	if s := celu.pedir("POST", "/v1/bichos/"+id+"/comando", map[string]string{"reproducir": "saludo"}, nil); s != 202 {
		t.Fatalf("reproducir: %d", s)
	}
	esperar(t, "que se vea la animación", 5*time.Second, func() bool { return leerVirtual(t).Visible == "animacion:saludo" })
	web.pedir("POST", "/v1/bichos/"+id+"/comando", map[string]string{"expresion": "enamorado"}, nil)
	esperar(t, "que quede enamorado", 5*time.Second, func() bool { return leerVirtual(t).Estado.Fija && leerVirtual(t).Estado.Expresion == "enamorado" })
	web.pedir("POST", "/v1/bichos/"+id+"/comando", map[string]string{"expresion": ""}, nil)

	// --- Imagen ---
	img := qoi(320, 172, 200, 80, 30)
	if s := web.pedir("PUT", "/v1/bichos/"+id+"/imagenes/3", img, nil); s != 200 {
		t.Fatalf("subir la imagen: %d", s)
	}
	esperar(t, "que la imagen llegue al bicho", 60*time.Second, func() bool { return leerVirtual(t).Imagenes["img3"] == int64(len(img)) })
	if s := web.pedir("PUT", "/v1/bichos/"+id+"/imagenes/4", qoi(10, 10, 1, 2, 3), nil); s != 422 {
		t.Fatalf("una imagen de otro tamaño: %d", s)
	}

	// --- Un cambio hecho cerca, por BLE: la nube se lo trae ---
	r, err := http.Post(virtual+"/mostrar-qr", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	var q struct{ QR string }
	json.NewDecoder(r.Body).Decode(&q)
	r.Body.Close()
	datosQr, ok := protocolo.LeerQr(q.QR)
	if !ok {
		t.Fatalf("QR %q", q.QR)
	}
	local := `{"version":1,"nombre":"Cerca","cara":{"color":"#33AAFF"}}`
	var escrituras []map[string]any
	escrituras = append(escrituras, map[string]any{"car": protocolo.CarComando, "datos": protocolo.Vincular(datosQr.Codigo)})
	for _, m := range protocolo.EscriturasDeConfig([]byte(local), 180) {
		escrituras = append(escrituras, map[string]any{"car": protocolo.CarConfig, "datos": m})
	}
	b, _ := json.Marshal(map[string]any{"escrituras": escrituras})
	if r, err := http.Post(virtual+"/ble", "application/json", bytes.NewReader(b)); err != nil || r.StatusCode != 200 {
		t.Fatalf("BLE: %v", err)
	}
	esperar(t, "que la API traiga la config que cambió en el bicho", 30*time.Second, func() bool {
		var c struct {
			Autor  string
			Config map[string]any
		}
		web.pedir("GET", "/v1/bichos/"+id+"/config", nil, &c)
		return c.Autor == "bicho" && c.Config["nombre"] == "Cerca"
	})

	// --- OTA (la cuenta tiene que estar en ADMIN_EMAILS) ---
	subir := func(version string, contenido []byte) int64 {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		mw.WriteField("perfil", "virtual")
		mw.WriteField("version", version)
		fw, _ := mw.CreateFormFile("binario", "firmware.bin")
		fw.Write(contenido)
		mw.Close()
		req, _ := http.NewRequest("POST", api+"/v1/firmwares", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("X-Sintetica", "1")
		resp, err := web.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var f struct{ ID int64 }
		cuerpo, _ := io.ReadAll(resp.Body)
		json.Unmarshal(cuerpo, &f)
		if resp.StatusCode == 403 {
			t.Skip("para la OTA, levantá el compose con ADMIN_EMAILS=@ejemplo.com")
		}
		if resp.StatusCode != 201 {
			t.Fatalf("subir el firmware: %d %s", resp.StatusCode, cuerpo)
		}
		return f.ID
	}
	estadoOta := func() (string, string) {
		var l struct {
			Actualizaciones []struct{ Estado, Detalle, Version string }
		}
		web.pedir("GET", "/v1/bichos/"+id+"/actualizaciones", nil, &l)
		if len(l.Actualizaciones) == 0 {
			return "", ""
		}
		return l.Actualizaciones[0].Estado, l.Actualizaciones[0].Detalle
	}
	sufijo := fmt.Sprintf("%d", time.Now().Unix()%100000)
	buena := subir("0.9."+sufijo, []byte("un firmware virtual que anda "+sufijo))
	if s := web.pedir("POST", "/v1/bichos/"+id+"/actualizar", map[string]int64{"firmware": buena}, nil); s != 202 {
		t.Fatalf("mandar la OTA: %d", s)
	}
	esperar(t, "que se instale la versión nueva", 60*time.Second, func() bool { return leerVirtual(t).Version == "0.9."+sufijo })
	esperar(t, "que la OTA quede confirmada", 90*time.Second, func() bool { e, _ := estadoOta(); return e == "confirmada" })

	mala := subir("1.0."+sufijo, []byte("FALLAR: un firmware roto"))
	web.pedir("POST", "/v1/bichos/"+id+"/actualizar", map[string]int64{"firmware": mala}, nil)
	esperar(t, "que la OTA mala vuelva atrás", 90*time.Second, func() bool {
		e, d := estadoOta()
		return e == "fallida" && strings.Contains(d, "anterior")
	})
	if v := leerVirtual(t).Version; v != "0.9."+sufijo {
		t.Fatalf("después de la vuelta atrás tiene %s", v)
	}

	// --- Sacarlo de la cuenta ---
	if s := web.pedir("DELETE", "/v1/bichos/"+id, nil, nil); s != 204 {
		t.Fatalf("sacarlo: %d", s)
	}
	esperar(t, "que se desconecte", 20*time.Second, func() bool { return !leerVirtual(t).Nube })
	_ = base64.StdEncoding
}

// El motor de vida en la nube: la memoria llega a la API (el diario) y se muda
// de un bicho a otro (cambio de placa).
func TestMemoriaYMudanza(t *testing.T) {
	web := entrar(t, "web")
	var virtuales struct {
		Virtuales []struct{ ID string }
	}
	web.pedir("GET", "/v1/bichos/virtuales", nil, &virtuales)
	if len(virtuales.Virtuales) < 2 {
		t.Skip("hacen falta los dos bichos virtuales")
	}
	viejo, nuevo := virtuales.Virtuales[0].ID, virtuales.Virtuales[1].ID
	for _, id := range []string{viejo, nuevo} {
		if s := web.pedir("POST", "/v1/bichos/reclamar-virtual", map[string]string{"id": id}, nil); s != 200 {
			t.Fatalf("sumar %s: %d", id, s)
		}
		defer web.pedir("DELETE", "/v1/bichos/"+id, nil, nil)
	}
	conectado := func(id string) bool {
		var v struct{ Conectado bool }
		web.pedir("GET", "/v1/bichos/"+id, nil, &v)
		return v.Conectado
	}
	esperar(t, "que se conecten los dos", 30*time.Second, func() bool { return conectado(viejo) && conectado(nuevo) })

	// Tres caricias al viejo (por la nube, como desde la web).
	time.Sleep(2 * time.Second)
	for i := 0; i < 3; i++ {
		if s := web.pedir("POST", "/v1/bichos/"+viejo+"/comando", map[string]string{"evento": "caricia"}, nil); s != 202 {
			t.Fatalf("caricia: %d", s)
		}
		time.Sleep(300 * time.Millisecond)
	}
	type memoria struct {
		Memoria *struct {
			Caricias int
			Dias     int
		}
		MudanzaPendiente bool
	}
	// Pedirle la memoria ya (si no, la API la pide cada 5 minutos).
	web.pedir("POST", "/v1/bichos/"+viejo+"/marcos", map[string]any{"car": protocolo.CarComando, "datos": protocolo.PedirMemoria()}, nil)
	esperar(t, "que la memoria del viejo llegue a la API", 20*time.Second, func() bool {
		var m memoria
		web.pedir("GET", "/v1/bichos/"+viejo+"/memoria", nil, &m)
		return m.Memoria != nil && m.Memoria.Caricias >= 3 && m.Memoria.Dias >= 1
	})

	// La mudanza: la memoria del viejo pasa al nuevo.
	if s := web.pedir("POST", "/v1/bichos/"+nuevo+"/mudar", map[string]string{"desde": viejo}, nil); s != 202 {
		t.Fatalf("mudar: %d", s)
	}
	esperar(t, "que el nuevo confirme la mudanza", 90*time.Second, func() bool {
		var m memoria
		web.pedir("GET", "/v1/bichos/"+nuevo+"/memoria", nil, &m)
		return !m.MudanzaPendiente && m.Memoria != nil && m.Memoria.Caricias >= 3
	})
	// Y de verdad la tiene: se la pido al bicho.
	web.pedir("POST", "/v1/bichos/"+nuevo+"/marcos", map[string]any{"car": protocolo.CarComando, "datos": protocolo.PedirMemoria()}, nil)
	time.Sleep(2 * time.Second)
	var m memoria
	web.pedir("GET", "/v1/bichos/"+nuevo+"/memoria", nil, &m)
	if m.Memoria == nil || m.Memoria.Caricias < 3 {
		t.Fatalf("el bicho nuevo no tiene la memoria del viejo: %+v", m.Memoria)
	}
}
