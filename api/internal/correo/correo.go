// Package correo manda los links mágicos por SMTP (en local, a Mailpit).
package correo

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Correo es un servidor SMTP.
type Correo struct {
	Servidor string // host:puerto
	Usuario  string // vacío = sin autenticación (Mailpit)
	Clave    string
	De       string // "Sintética Labs <hola@sinteticalabs.com>"
}

// Mensaje es un mail con texto y HTML.
type Mensaje struct {
	Para, Asunto, Texto, HTML string
}

// Mandar manda un mail. Tarda como mucho 15 s.
func (c *Correo) Mandar(ctx context.Context, m Mensaje) error {
	ctx, cancelar := context.WithTimeout(ctx, 15*time.Second)
	defer cancelar()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", c.Servidor)
	if err != nil {
		return fmt.Errorf("SMTP: %w", err)
	}
	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl) //nolint:errcheck
	}
	host, _, _ := net.SplitHostPort(c.Servidor)
	cli, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMTP: %w", err)
	}
	defer cli.Close()
	if ok, _ := cli.Extension("STARTTLS"); ok {
		if err := cli.StartTLS(nil); err != nil {
			return fmt.Errorf("SMTP STARTTLS: %w", err)
		}
	}
	if c.Usuario != "" {
		if err := cli.Auth(smtp.PlainAuth("", c.Usuario, c.Clave, host)); err != nil {
			return fmt.Errorf("SMTP: %w", err)
		}
	}
	direccion := c.De
	if i := strings.LastIndex(direccion, "<"); i >= 0 {
		direccion = strings.Trim(direccion[i:], "<>")
	}
	if err := cli.Mail(direccion); err != nil {
		return err
	}
	if err := cli.Rcpt(m.Para); err != nil {
		return err
	}
	w, err := cli.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(armar(c.De, m)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return cli.Quit()
}

func armar(de string, m Mensaje) []byte {
	var b bytes.Buffer
	frontera := fmt.Sprintf("sintetica-%d", time.Now().UnixNano())
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\n",
		de, m.Para, mime.QEncoding.Encode("utf-8", m.Asunto), time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", frontera)
	for _, parte := range []struct{ tipo, cuerpo string }{{"text/plain", m.Texto}, {"text/html", m.HTML}} {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: %s; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", frontera, parte.tipo)
		q := quotedprintable.NewWriter(&b)
		q.Write([]byte(parte.cuerpo)) //nolint:errcheck
		q.Close()
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", frontera)
	return b.Bytes()
}
