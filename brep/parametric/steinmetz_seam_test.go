package parametric

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestSteinmetzExplicitSeams(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 4, Y: -2, Z: 1}
	original, e := PerpendicularCylinderCommon(ctx, center, 2, 3, 4, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	result, e := PerpendicularCylinderCommonExplicit(ctx, center, 2, 3, 4, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteJSON(&native, result.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadJSON(&native, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	result.Solid = loaded
	old, edges := original.Solid.Edges(), result.Solid.Edges()
	if len(old) != len(edges) {
		t.Fatal("edge identity")
	}
	count := 0
	for i, edge := range edges {
		if edge.Start != old[i].Start || edge.End != old[i].End {
			t.Fatal("changed incidence")
		}
		if edge.Curve == nil {
			continue
		}
		seam, ok := edge.Curve.(SteinmetzSeam)
		if !ok {
			t.Fatal("not explicit")
		}
		count++
		for j := 0; j <= 32; j++ {
			t0 := float64(j) / 32
			p, e := seam.Eval(t0)
			if e != nil {
				t.Fatal(e)
			}
			q, e := old[i].Curve.Eval(t0)
			if e != nil || p.Sub(q).Length() > 1e-12 {
				t.Fatal("changed geometry", p, q, e)
			}
			local := p.Sub(center)
			if math.Abs(local.X*local.X+local.Y*local.Y-4) > 1e-12 || math.Abs(local.Z*local.Z+local.Y*local.Y-4) > 1e-12 {
				t.Fatal("not on both cylinders", local)
			}
		}
		bound, e := seam.IdealChordSquaredBound(.25, .5)
		if e != nil {
			t.Fatal(e)
		}
		limit, _ := bound.Float64()
		a, _ := seam.Eval(.25)
		b, _ := seam.Eval(.5)
		for j := 0; j <= 100; j++ {
			v := float64(j) / 100
			p, _ := seam.Eval(.25 + .25*v)
			chord := a.Scale(1 - v).Add(b.Scale(v))
			d := p.Sub(chord)
			if d.Dot(d) > limit {
				t.Fatal("bound", d.Dot(d), limit)
			}
		}
		smaller, e := seam.IdealChordSquaredBound(.25, .375)
		if e != nil {
			t.Fatal(e)
		}
		ratio := new(big.Rat).Quo(bound, smaller)
		if ratio.Cmp(big.NewRat(16, 1)) != 0 {
			t.Fatal("quartic refinement", ratio)
		}
	}
	if count != 4 {
		t.Fatal("ellipse half arcs", count)
	}
	m, e := result.Solid.Tessellate(ctx, TessellationOptions{Deflection: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	report, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal(e, len(report.Contacts))
	}
}

func TestSteinmetzSeamInvalidCodec(t *testing.T) {
	raw, e := EncodeCurve(SteinmetzSeam{radius: 2, sx: 1, sz: -1})
	if e != nil {
		t.Fatal(e)
	}
	bad := bytes.Replace(raw, []byte(`"Steinmetz":[1,-1,0]`), []byte(`"Steinmetz":[1,-1,2]`), 1)
	if bytes.Equal(raw, bad) {
		t.Fatal("test payload")
	}
	if _, e := DecodeCurve(bad, 100); e == nil {
		t.Fatal("bad reversal")
	}
	s := SteinmetzSeam{radius: 2, sx: 1, sz: 1}
	if _, e := s.IdealChordSquaredBound(.5, .25); e == nil {
		t.Fatal("inverted range")
	}
}
