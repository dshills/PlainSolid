package parametric

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestUnequalPerpendicularUnion(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: 3}
	r, e := UnequalPerpendicularCylinderUnion(ctx, center, 5, 3, 4, 6, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	common, e := PerpendicularCylinderCommonRadii(ctx, center, 5, 3, 4, 6, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	c, e := common.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	want := 308*math.Pi - c.Volume
	if math.Abs(m.Volume-want) > 1e-5 || m.Centroid.Sub(center).Length() > 1e-7 {
		t.Fatal(m, want)
	}
	t.Log(m.Area, m.Volume, len(r.Solid.Faces()))
	// Independent one-dimensional geometric integrals after y=3 sin(u).
	const n = 8192
	var overlap, buried float64
	h := math.Pi / n
	for i := 0; i <= n; i++ {
		u := -math.Pi/2 + float64(i)*h
		sn, cs := math.Sincos(u)
		root := math.Sqrt(25 - 9*sn*sn)
		weight := 2.
		if i == 0 || i == n {
			weight = 1
		} else if i%2 == 1 {
			weight = 4
		}
		overlap += weight * 36 * cs * cs * root
		buried += weight * (180*cs*cs/root + 12*root)
	}
	overlap *= h / 3
	buried *= h / 3
	wantVolume := 308*math.Pi - overlap
	wantArea := 4*math.Pi*(5*4+3*6) + 2*math.Pi*(25+9) - buried
	if math.Abs(m.Volume-wantVolume) > 1e-6 || math.Abs(m.Area-wantArea) > 1e-6 {
		t.Fatal("independent integrals", m, wantVolume, wantArea)
	}
	seen := map[int]bool{}
	for owner := 0; owner < 2; owner++ {
		for _, group := range [][]int{r.WallFaces[owner], r.LowerCapFaces[owner], r.UpperCapFaces[owner]} {
			for _, id := range group {
				if seen[id] {
					t.Fatal("duplicate ancestry")
				}
				seen[id] = true
			}
		}
	}
	if len(seen) != 32 {
		t.Fatal("lost ancestry")
	}
	bounds, e := r.Solid.Bounds()
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range r.Solid.Faces() {
		chart := f.Surface.(unequalUnionChart)
		for _, u := range []float64{f.U.at(.2), f.U.at(.7)} {
			p, e := chart.Eval(u, .3)
			if e != nil {
				t.Fatal(e)
			}
			d := p.Sub(center)
			var residual float64
			switch chart.kind {
			case 0, 1:
				residual = d.Y*d.Y + d.Z*d.Z - 25
			case 2, 3:
				residual = d.X - float64(chart.side)*4
			case 4:
				residual = d.X*d.X + d.Y*d.Y - 9
			case 5:
				residual = d.Z - float64(chart.side)*6
			}
			if math.Abs(residual) > 1e-12 || p.X < bounds.Min.X || p.X > bounds.Max.X || p.Y < bounds.Min.Y || p.Y > bounds.Max.Y || p.Z < bounds.Min.Z || p.Z > bounds.Max.Z {
				t.Fatal("support/bounds", p, residual)
			}
		}
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, r.Solid); e != nil {
		t.Fatal(e)
	}
	corrupt := bytes.Replace(data.Bytes(), []byte(`"Shoulder":4`), []byte(`"Shoulder":3`), 1)
	if bytes.Equal(corrupt, data.Bytes()) {
		t.Fatal("ineffective codec mutation")
	}
	if _, e := ReadJSON(bytes.NewReader(corrupt), JSONLimits{}); e == nil {
		t.Fatal("unproved shoulder accepted")
	}
	restored, e := ReadJSON(&data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := restored.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || mass != m {
		t.Fatal("native reload", mass, e)
	}
	mesh, e := restored.Tessellate(ctx, TessellationOptions{Deflection: 1, AnalyticBound: true})
	if e != nil || mesh.ValidateSolid() != nil || mesh.SignedVolume() <= 0 {
		t.Fatal("closed display", e)
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("contacts", e, len(contacts.Contacts))
	}
	_, e = UnequalPerpendicularCylinderUnion(ctx, center, 5, 2, 4, 6, 1e-8)
	if !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("irrational shoulder", e)
	}
	_, e = UnequalPerpendicularCylinderUnion(ctx, center, 5, 3, 3, 6, 1e-8)
	if !errors.Is(e, geom.ErrDomain) {
		t.Fatal("cap clearance", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = UnequalPerpendicularCylinderUnion(canceled, center, 5, 3, 4, 6, 1e-8)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}

}
