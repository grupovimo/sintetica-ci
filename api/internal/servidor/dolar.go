package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// El dólar oficial de DolarApi (el valor de venta), para pasar los costos del
// control a una sola moneda. Se pide desde acá (no desde la web) y se guarda
// una hora; si DolarApi no contesta, sigue el último bueno.

type cotizacion struct {
	mu    sync.Mutex
	valor float64
	fecha time.Time // la de DolarApi
	en    time.Time // cuándo se pidió
}

const vigenciaDolar = time.Hour

var clienteDolar = &http.Client{Timeout: 6 * time.Second}

// dolar: el oficial de venta y de cuándo es.
func (s *Servidor) dolar(ctx context.Context) (float64, time.Time, error) {
	c := s.cotizacion
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.valor > 0 && time.Since(c.en) < vigenciaDolar {
		return c.valor, c.fecha, nil
	}
	v, f, err := s.pedirDolar(ctx)
	if err != nil {
		if c.valor > 0 {
			s.log.Warn("dolar: sigo con el último", "error", err)
			return c.valor, c.fecha, nil
		}
		return 0, time.Time{}, err
	}
	c.valor, c.fecha, c.en = v, f, time.Now()
	return v, f, nil
}

func (s *Servidor) pedirDolar(ctx context.Context) (float64, time.Time, error) {
	url := s.op.DolarURL
	if url == "" {
		url = "https://dolarapi.com/v1/dolares/oficial"
	}
	ctx, cancelar := context.WithTimeout(ctx, 6*time.Second)
	defer cancelar()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, time.Time{}, err
	}
	res, err := clienteDolar.Do(req)
	if err != nil {
		return 0, time.Time{}, errors.New("DolarApi no contestó")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, time.Time{}, fmt.Errorf("DolarApi contestó %d", res.StatusCode)
	}
	var d struct {
		Venta float64   `json:"venta"`
		Fecha time.Time `json:"fechaActualizacion"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&d); err != nil {
		return 0, time.Time{}, err
	}
	if d.Venta <= 0 {
		return 0, time.Time{}, errors.New("DolarApi no trajo el valor")
	}
	return d.Venta, d.Fecha, nil
}

// GET /v1/admin/dolar → {valor, fecha}.
func (s *Servidor) verDolar(w http.ResponseWriter, r *http.Request) {
	v, f, err := s.dolar(r.Context())
	if err != nil {
		fallar(w, http.StatusBadGateway, "no pude traer el dólar", err.Error())
		return
	}
	responder(w, http.StatusOK, map[string]any{"valor": v, "fecha": f})
}
