package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestFilletedConeEccentricThroughBore(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	for _, descending := range []bool{false, true} {
		R, delta, lo, hi := 8., 8., 15./16, 15./8
		if descending {
			R, delta, lo, hi = 16, -8, hi, lo
		}
		w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, R, 6, o)
		if e != nil {
			t.Fatal(e)
		}
		w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, delta, o)
		if e != nil {
			t.Fatal(e)
		}
		source, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		stock, _, e := FilletWrappedFrustumRims(ctx, source, lo, hi, o)
		if e != nil {
			t.Fatal(e)
		}
		out, h, e := FilletAndBoreWrappedFrustum(ctx, source, lo, hi, geom.Vec2{X: 1, Y: .5}, 1, o)
		if e != nil {
			t.Fatal(descending, e)
		}
		if out.Topology().Faces != 6 || out.Topology().EulerCharacteristic != 0 || len(out.Topology().Shells) != 1 {
			t.Fatal(out.Topology())
		}
		if !reflect.DeepEqual(stock.Vertices(), out.Vertices()[:len(stock.Vertices())]) {
			t.Fatal("original vertices")
		}
		for i, f := range stock.Faces() {
			g := out.Faces()[i]
			if !reflect.DeepEqual(f.Surface, g.Surface) || !reflect.DeepEqual(f.Outer, g.Outer) || f.Reversed != g.Reversed || !reflect.DeepEqual(h.StockFaces[i], []int{i}) {
				t.Fatal("original support/owner", i)
			}
		}
		for i, edge := range stock.Edges() {
			if !reflect.DeepEqual(edge, out.Edges()[i]) {
				t.Fatal("original edge", i)
			}
		}
		for end, cap := range h.Fillets.Caps {
			if len(out.Faces()[cap].Holes) != 1 || len(h.BoreRims[end]) != 4 {
				t.Fatal("cap holes", h)
			}
		}
		a, e := stock.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		b, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs((a.Volume-b.Volume)-6*math.Pi) > 1e-7 || math.Abs((b.Area-a.Area)-10*math.Pi) > 1e-7 {
			t.Fatal(a, b)
		}
		var buf bytes.Buffer
		if e = WriteJSON(&buf, out); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &buf, JSONLimits{})
		if e != nil || back.Topology().EulerCharacteristic != 0 {
			t.Fatal(e)
		}
		// Exact smaller flat-cap tangent disk is excluded, despite clear original cone.
		minimum := h.Fillets.Circles[0].Center[0]
		if h.Fillets.Circles[1].Center[0].Cmp(minimum) < 0 {
			minimum = h.Fillets.Circles[1].Center[0]
		}
		m, _ := minimum.Float64()
		bad, bh, e := FilletAndBoreWrappedFrustum(ctx, source, lo, hi, geom.Vec2{X: m - 1}, 1, o)
		if e == nil || bad.Topology().Faces != 0 || bh.StockFaces != nil {
			t.Fatal("tangent clearance", e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		bad, _, e = FilletAndBoreWrappedFrustum(cancelled, source, lo, hi, geom.Vec2{}, 1, o)
		if e != context.Canceled || bad.Topology().Faces != 0 {
			t.Fatal("cancel", e)
		}
	}
}
