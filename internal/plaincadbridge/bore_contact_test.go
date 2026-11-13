package plaincadbridge

import (
	"errors"
	"math"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestExactBlindBoreTopologyDiagnostic(t *testing.T) {
	for _, x := range []float64{1, math.Nextafter(1, 0), math.Nextafter(1, 2)} {
		b := New()
		stock := box(t, b, vector{0, 0, 0}, vector{20, 10, 10})
		tool := circle(t, b, vector{x, 5, 10}, vector{0, 0, -1}, 1, 3)
		base := b.shapes[stock.Handle].solid
		before := len(b.shapes)
		_, e := b.execute(request{Op: "cut", A: stock.Handle, B: tool.Handle})
		var contact *NonManifoldBoreContactError
		if x != 1 {
			if errors.As(e, &contact) {
				t.Fatal("near contact certified", x)
			}
			continue
		}
		if !errors.As(e, &contact) || !errors.Is(e, ErrNonManifoldBoreContact) {
			t.Fatalf("%T %v", e, e)
		}
		var failure *failureError
		if !errors.As(e, &failure) || failure.code != "UNSUPPORTED" {
			t.Fatal(e)
		}
		if len(b.shapes) != before || contact.ToolIndex != 0 || len(contact.SideFaces) != 1 {
			t.Fatal("publication or ancestry")
		}
		entry, _ := base.FacePlane(contact.EntryFace)
		side, _ := base.FacePlane(contact.SideFaces[0])
		if entry.Origin().Z != 10 || side.Origin().X != 0 {
			t.Fatal(entry, side)
		}
		g := contact.Contact.Generators[0]
		if g.Start[2].RatString() != "10" || g.End[2].RatString() != "7" || g.Start[0].Sign() != 0 {
			t.Fatal(g)
		}
		out := b.Call(`{"op":"cut","a":"` + stock.Handle + `","b":"` + tool.Handle + `"}`)
		if !strings.Contains(out, `"code":"UNSUPPORTED"`) || len(b.shapes) != before {
			t.Fatal(out)
		}
	}
}
func TestExactBlindBoreNoProofForObliqueFrame(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, 0, 0}, vector{20, 10, 10})
	f, _ := geom.NewFrame(geom.Vec3{X: 1, Y: 5, Z: 10}, geom.Vec3{X: 1, Y: 1}, geom.Vec3{Z: -1})
	c, e := newCylinder(f, 1, 3)
	if e != nil {
		t.Fatal(e)
	}
	if e := exactBlindBoreContact(b.shapes[stock.Handle].solid, c, 1, 0); e != nil {
		t.Fatal("oblique proof", e)
	}
}
