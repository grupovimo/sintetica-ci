package edgetts

import (
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTokenGEC(t *testing.T) {
	a := TokenGEC(1762600000)
	if !regexp.MustCompile(`^[0-9A-F]{16}-[0-9A-F]{64}$`).MatchString(a) {
		t.Fatalf("formato: %s", a)
	}
	// Cambia cada 5 minutos (de reloj de Windows), no antes.
	base := int64(1762600200) // + 11644473600 es múltiplo de 300
	if (base+epochWindows)%300 != 0 {
		t.Fatal("la base no cae justo")
	}
	if TokenGEC(base) != TokenGEC(base+299) || TokenGEC(base) == TokenGEC(base+300) || TokenGEC(base) == TokenGEC(base-1) {
		t.Fatal("ventanas de 5 minutos")
	}
	// Las palabras del CS64: rellena con ceros a un número par.
	if w := palabrasPares([]byte{1, 2, 3, 4, 5}); len(w) != 2 || w[0] != 0x04030201 || w[1] != 5 {
		t.Fatalf("%x", w)
	}
}

// Contra el servicio de verdad, solo a mano: EDGE_TTS_REAL=1 go test -run Real ./api/internal/edgetts/
func TestReal(t *testing.T) {
	if os.Getenv("EDGE_TTS_REAL") == "" {
		t.Skip("sin EDGE_TTS_REAL")
	}
	c := Nuevo()
	ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelar()
	a, err := c.Hablar(ctx, VozPorDefecto, "Hola, ¿cómo andás?")
	if err != nil || len(a) < 1000 {
		t.Fatalf("hablar: %d bytes, %v", len(a), err)
	}
	vs, err := c.Voces(ctx, "es-AR")
	if err != nil || len(vs) == 0 {
		t.Fatalf("voces: %v %v", vs, err)
	}
	t.Logf("%d bytes de mp3; voces: %+v", len(a), vs)
}

// falso: un servidor como el de Edge (lo que describe communicate.py).
func falso(t *testing.T, rechazos *atomic.Int32) *httptest.Server {
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		return r.Header.Get("Origin") == "chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold"
	}}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path == "/voces" {
			if q.Get("trustedclienttoken") != TokenCliente {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Write([]byte(`[{"ShortName":"es-AR-ElenaNeural","FriendlyName":"Microsoft Elena Online (Natural) - Spanish (Argentina)","Locale":"es-AR","Gender":"Female"},
				{"ShortName":"es-AR-TomasNeural","FriendlyName":"Microsoft Tomas Online (Natural) - Spanish (Argentina)","Locale":"es-AR","Gender":"Male"},
				{"ShortName":"en-US-EmmaMultilingualNeural","FriendlyName":"Microsoft Emma","Locale":"en-US","Gender":"Female"}]`)) //nolint:errcheck
			return
		}
		if q.Get("TrustedClientToken") != TokenCliente || q.Get("ConnectionId") == "" || q.Get("Sec-MS-GEC-Version") != "2-"+VersionChromium ||
			!strings.HasPrefix(r.Header.Get("Cookie"), "muid=") || !strings.Contains(r.Header.Get("User-Agent"), "Edg/") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// La primera vez dice que la hora está mal (403 con su Date).
		if rechazos != nil && rechazos.Add(1) == 1 {
			w.Header().Set("Date", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusForbidden)
			return
		}
		ws, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		_, cfg, err := ws.ReadMessage()
		if err != nil || !strings.Contains(string(cfg), "Path:speech.config") || !strings.Contains(string(cfg), Formato) {
			t.Errorf("config: %q", cfg)
			return
		}
		_, p, err := ws.ReadMessage()
		if err != nil || !strings.Contains(string(p), "Path:ssml") {
			t.Errorf("ssml: %q", p)
			return
		}
		_, ssml, _ := strings.Cut(string(p), "\r\n\r\n")
		ws.WriteMessage(websocket.TextMessage, []byte("X-RequestId:x\r\nPath:turn.start\r\n\r\n{}")) //nolint:errcheck
		for _, trozo := range []string{"ID3", ssml} {
			cab := "X-RequestId:x\r\nContent-Type:audio/mpeg\r\nPath:audio\r\n"
			m := binary.BigEndian.AppendUint16(nil, uint16(len(cab)))
			ws.WriteMessage(websocket.BinaryMessage, append(append(m, cab...), trozo...)) //nolint:errcheck
		}
		cab := "X-RequestId:x\r\nPath:audio\r\n"
		ws.WriteMessage(websocket.BinaryMessage, append(binary.BigEndian.AppendUint16(nil, uint16(len(cab))), cab...)) //nolint:errcheck
		ws.WriteMessage(websocket.TextMessage, []byte("X-RequestId:x\r\nPath:turn.end\r\n\r\n{}"))                     //nolint:errcheck
	}))
	t.Cleanup(s.Close)
	return s
}

func clienteFalso(s *httptest.Server) *Cliente {
	c := Nuevo()
	c.URLWS = "ws" + strings.TrimPrefix(s.URL, "http") + "/edge/v1"
	c.URLVoces = s.URL + "/voces"
	return c
}

func TestHablarConServidorFalso(t *testing.T) {
	var rechazos atomic.Int32
	c := clienteFalso(falso(t, &rechazos))
	ctx := context.Background()
	a, err := c.Hablar(ctx, "es-AR-TomasNeural", "Hola <Ana> & \x0bvos")
	if err != nil {
		t.Fatal(err)
		t.Fatal(err)
	}
	s := string(a)
	if !strings.HasPrefix(s, "ID3<speak") || !strings.Contains(s, "<voice name='es-AR-TomasNeural'>") ||
		!strings.Contains(s, "Hola &lt;Ana&gt; &amp;  vos") {
		t.Fatalf("audio: %q", s)
	}
	if rechazos.Load() != 2 {
		t.Fatalf("con el 403 tiene que corregir la hora y reintentar: %d", rechazos.Load())
	}
	c.mu.Lock()
	d := c.desfase
	c.mu.Unlock()
	if d < 59*time.Minute || d > 61*time.Minute {
		t.Fatalf("desfase: %s", d)
	}
	for _, v := range []string{"", "es-AR-Elena'><x", "es-AR-Elena", strings.Repeat("a", 70) + "Neural"} {
		if _, err := c.Hablar(ctx, v, "hola"); err != ErrVoz {
			t.Errorf("voz %q: %v", v, err)
		}
	}
	if _, err := c.Hablar(ctx, VozPorDefecto, "  "); err != ErrTexto {
		t.Fatalf("vacío: %v", err)
	}
	vs, err := c.Voces(ctx, "es-")
	if err != nil || len(vs) != 2 || vs[0].Nombre != "es-AR-ElenaNeural" || vs[0].Titulo != "Elena Online (Natural)" || vs[1].Genero != "Male" {
		t.Fatalf("voces: %+v %v", vs, err)
	}
}

func TestHablarSinServicio(t *testing.T) {
	c := Nuevo()
	c.URLWS = "ws://127.0.0.1:1/edge/v1"
	ctx, cancelar := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelar()
	if _, err := c.Hablar(ctx, VozPorDefecto, "hola"); err == nil {
		t.Fatal("sin servicio tiene que fallar")
	}
}
