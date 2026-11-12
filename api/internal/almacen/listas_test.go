package almacen

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Al unir cuentas, las listas pasan; las de nombre repetido se juntan.
func TestUnirListas(t *testing.T) {
	a := abrirPrueba(t)
	ctx := context.Background()
	ana, _ := a.UsuarioPorEmail(ctx, "ana-listas@ejemplo.com", false)
	beto, _ := a.UsuarioPorEmail(ctx, "beto-listas@ejemplo.com", false)
	for _, x := range []struct{ u, lista, item string }{{ana.ID, "súper", "yerba"}, {ana.ID, "regalos", "libro"}, {beto.ID, "Súper", "pan"}} {
		l, _, err := a.AsegurarLista(ctx, x.u, x.lista)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := a.AgregarItems(ctx, x.u, l.ID, []string{x.item}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.enTx(ctx, func(tx pgx.Tx) error { return unirListas(ctx, tx, ana.ID, beto.ID) }); err != nil {
		t.Fatal(err)
	}
	ls, _ := a.Listas(ctx, beto.ID)
	if len(ls) != 2 || ls[0].Nombre != "regalos" || ls[1].Nombre != "Súper" || len(ls[1].Items) != 2 {
		t.Fatalf("unidas: %+v", ls)
	}
	if ls, _ := a.Listas(ctx, ana.ID); len(ls) != 0 {
		t.Fatalf("quedaron: %+v", ls)
	}
}
