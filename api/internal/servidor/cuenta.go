package servidor

// Sumar un mail a la cuenta (docs/nube.md, "Cuentas", "Una sola cuenta por
// persona"): la que se creó con Telegram no tiene mail; con un código al mail
// se le suma. Si ese mail ya es de otra cuenta, se unen (la vacía se suma a
// la otra, almacen/unir.go). Y los administradores por Telegram
// (ADMIN_TELEGRAM).

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/correo"
)

// POST /v1/sesion/email {email}: manda un código de 8 letras a ese mail (vale
// 15 minutos). Hace falta SMTP.
func (s *Servidor) pedirSumarEmail(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Email string `json:"email"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	if s.correo == nil {
		fallar(w, http.StatusServiceUnavailable, "este servidor no manda mails todavía")
		return
	}
	u := usuarioDe(r).Usuario
	if u.Email != "" {
		fallar(w, http.StatusConflict, almacen.ErrYaTieneMail.Error())
		return
	}
	dir, err := mail.ParseAddress(strings.TrimSpace(p.Email))
	if err != nil || len(dir.Address) > 254 {
		fallar(w, http.StatusBadRequest, "ese email no parece válido")
		return
	}
	email := strings.ToLower(dir.Address)
	ok, err := s.alm.Tope(r.Context(), "sumar-email:"+u.ID, 0, 5, time.Hour)
	if err != nil {
		s.interno(w, err)
		return
	}
	if !ok || !s.limites.permitir("ip:"+ipDe(r), 20, time.Hour) {
		fallar(w, http.StatusTooManyRequests, "pediste muchos códigos: probá en un rato")
		return
	}
	codigo := codigoCorto()
	if err := s.alm.PedirEmail(r.Context(), u.ID, email, hash("sumar:"+codigo), time.Now().Add(duracionEnlace)); err != nil {
		s.interno(w, err)
		return
	}
	m := mailSumarEmail(email, codigo)
	go func() {
		ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancelar()
		if err := s.correo.Mandar(ctx, m); err != nil {
			s.log.Error("no pude mandar el código para sumar el mail", "email", email, "error", err)
		}
	}()
	if s.op.Desarrollo {
		s.log.Info("código para sumar el mail (desarrollo)", "email", email, "codigo", codigo)
	}
	responder(w, http.StatusAccepted, map[string]string{"mensaje": "Te mandamos un código a ese mail (vale 15 minutos)."})
}

func mailSumarEmail(email, codigo string) correo.Mensaje {
	lindo := codigo[:4] + "-" + codigo[4:]
	texto := fmt.Sprintf("Hola,\n\nPara sumar este mail a tu cuenta de Sintética, escribí este código: %s\n\n"+
		"Vale 15 minutos. Si no fuiste vos, ignorá este mail.\n\nSintética Labs\n", lindo)
	return correo.Mensaje{Para: email, Asunto: "Tu código para sumar el mail a Sintética", Texto: texto}
}

// POST /v1/sesion/email/confirmar {codigo}: la cuenta queda con el mail. Si
// ese mail era de otra cuenta, quedan unidas: {"usuario", "unida": true} (la
// sesión sigue abierta, ahora en la cuenta que quedó).
func (s *Servidor) confirmarSumarEmail(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Codigo string `json:"codigo"`
	}
	if !leerJSON(w, r, 4096, &p) {
		return
	}
	if !s.limites.permitir("canjear:"+ipDe(r), 30, 10*time.Minute) {
		fallar(w, http.StatusTooManyRequests, "demasiados intentos: probá en un rato")
		return
	}
	u := usuarioDe(r).Usuario
	res, err := s.alm.ConfirmarEmail(r.Context(), u.ID, hash("sumar:"+normalizarCodigo(p.Codigo)))
	switch {
	case errors.Is(err, almacen.ErrEnlaceInvalido), errors.Is(err, almacen.ErrDemasiadosIntentos):
		fallar(w, http.StatusUnauthorized, err.Error())
		return
	case errors.Is(err, almacen.ErrCuentasConDatos), errors.Is(err, almacen.ErrOtroTelegram), errors.Is(err, almacen.ErrYaTieneMail):
		fallar(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		s.interno(w, err)
		return
	}
	if !res.Usuario.Admin && s.esAdmin(res.Usuario.Email) {
		if err := s.alm.MarcarAdmin(r.Context(), res.Usuario.ID); err != nil {
			s.interno(w, err)
			return
		}
		res.Usuario.Admin = true
	}
	responder(w, http.StatusOK, map[string]any{"usuario": res.Usuario, "unida": res.Unida})
}

// adminPorTelegram: entró con un Telegram de ADMIN_TELEGRAM (queda admin,
// como con ADMIN_EMAILS).
func (s *Servidor) adminPorTelegram(ctx context.Context, u *almacen.Usuario, chat int64) {
	if u.Admin || chat == 0 || !slices.Contains(s.op.AdminsTelegram, chat) {
		return
	}
	if err := s.alm.MarcarAdmin(ctx, u.ID); err != nil {
		s.log.Warn("no pude marcar al admin de Telegram", "error", err)
		return
	}
	u.Admin = true
}
