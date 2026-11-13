package plaincadbridge

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCoaxialIntervalUnion(t *testing.T) {
	for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		for _, reverse := range []bool{false, true} {
			b := New()
			origin := geom.Vec3{X: 10.25, Y: -3.5, Z: 2.125}
			cylinder := func(z, r, h float64) Shape {
				s, e := b.circle(origin.Add(axis.Scale(z)), axis, r, h)
				if e != nil {
					t.Fatal(e)
				}
				return s
			}
			a, c := cylinder(0, 2, 5), cylinder(3, 3, 4)
			lhs, rhs := a, c
			if reverse {
				lhs, rhs = rhs, lhs
			}
			joined, handled, e := b.fuseCoaxial(b.shapes[lhs.Handle], b.shapes[rhs.Handle])
			if !handled || e != nil {
				t.Fatal(axis, reverse, handled, e)
			}
			if math.Abs(joined.GeometryAssertions.Volume-48*math.Pi) > 1e-10 || math.Abs(joined.GeometryAssertions.SurfaceArea-54*math.Pi) > 1e-10 {
				t.Fatal(joined)
			}
			tail := cylinder(6, 1, 4)
			joined, handled, e = b.fuseCoaxial(b.shapes[tail.Handle], b.shapes[joined.Handle])
			if !handled || e != nil {
				t.Fatal(e)
			}
			body := b.shapes[joined.Handle].analytic
			mp, e := body.solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			want := origin.Add(axis.Scale(223.5 / 51))
			if math.Abs(mp.Volume-51*math.Pi) > 1e-8 || math.Abs(mp.Area-60*math.Pi) > 1e-8 || mp.Centroid.Distance(want) > 1e-8 {
				t.Fatal(mp, want)
			}
			m, e := body.tessellate(parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			mm, e := m.MassProperties()
			if e != nil || math.Abs(mm.Volume-mp.Volume) > mp.Volume*.01 {
				t.Fatal(mm, e)
			}
			if len(m.Triangles) > 10000 {
				t.Fatal("unbounded display mesh", len(m.Triangles))
			}
			if b.shapes[a.Handle].result.GeometryAssertions.Volume != 20*math.Pi {
				t.Fatal("source mutated")
			}
		}
	}
}

func TestCoaxialIntervalUnionContainmentTouchAndAtomicReject(t *testing.T) {
	b := New()
	makeCylinder := func(z, r, h float64) Shape {
		s, e := b.circle(geom.Vec3{Z: z}, geom.Vec3{Z: 1}, r, h)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a := makeCylinder(0, 2, 5)
	for index, c := range []Shape{makeCylinder(1, 1, 2), makeCylinder(0, 2, 5), makeCylinder(5, 1, 2)} {
		out, ok, e := b.fuseCoaxial(b.shapes[a.Handle], b.shapes[c.Handle])
		if !ok || e != nil {
			t.Fatal(ok, e)
		}
		want := 20 * math.Pi
		if index == 2 {
			want = 22 * math.Pi
		}
		if math.Abs(out.GeometryAssertions.Volume-want) > 1e-10 {
			t.Fatal(out)
		}
	}
	gap := makeCylinder(math.Nextafter(5, math.Inf(1)), 1, 2)
	before, next := len(b.shapes), b.next
	_, ok, e := b.fuseCoaxial(b.shapes[a.Handle], b.shapes[gap.Handle])
	if !ok || e == nil || len(b.shapes) != before || b.next != next {
		t.Fatal("gap must reject atomically", ok, e)
	}
	opposed, e := b.circle(geom.Vec3{Z: 7}, geom.Vec3{Z: -1}, 3, 4)
	if e != nil {
		t.Fatal(e)
	}
	out, ok, e := b.fuseCoaxial(b.shapes[a.Handle], b.shapes[opposed.Handle])
	if !ok || e != nil || math.Abs(out.GeometryAssertions.Volume-48*math.Pi) > 1e-10 {
		t.Fatal(out, ok, e)
	}
}

func TestCoaxialDecimalTranslatedJoin(t *testing.T) {
	b := New()
	a, e := b.circle(geom.Vec3{X: 12.3, Y: -4.7, Z: 2.25}, geom.Vec3{Z: 1}, 1.3, 5)
	if e != nil {
		t.Fatal(e)
	}
	c, e := b.circle(geom.Vec3{X: 12.3, Y: -4.7, Z: 5.25}, geom.Vec3{Z: 1}, 2.7, 4)
	if e != nil {
		t.Fatal(e)
	}
	out, ok, e := b.fuseCoaxial(b.shapes[a.Handle], b.shapes[c.Handle])
	if !ok || e != nil {
		t.Fatal(ok, e)
	}
	want := math.Pi * (1.3*1.3*3 + 2.7*2.7*4)
	if math.Abs(out.GeometryAssertions.Volume-want) > 1e-10 {
		t.Fatal(out)
	}
}

func TestCoaxialDispatchPlacementAndMeshBudget(t *testing.T) {
	b := New()
	a := circle(t, b, vector{}, vector{0, 0, 1}, 2, 5)
	c := circle(t, b, vector{0, 0, 3}, vector{0, 0, 1}, 3, 4)
	joined := shape(t, b, map[string]any{"op": "fuse", "a": a.Handle, "b": c.Handle})
	clone := shape(t, b, map[string]any{"op": "clone", "handle": joined.Handle})
	moved := shape(t, b, map[string]any{"op": "transform", "handle": clone.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	tail := circle(t, b, vector{13, 8, 9}, vector{1, 0, 0}, 1, 4)
	result := shape(t, b, map[string]any{"op": "fuse", "a": moved.Handle, "b": tail.Handle})
	if math.Abs(result.GeometryAssertions.Volume-51*math.Pi) > 1e-10 || result.Bounds.Min != (vector{7, 5, 6}) || result.Bounds.Max != (vector{17, 11, 12}) {
		t.Fatal(result)
	}
	if b.shapes[joined.Handle].result.Bounds.Min != (vector{-3, -3, 0}) {
		t.Fatal("placement mutated source")
	}
	body := b.shapes[result.Handle].analytic.coaxial
	options := parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 384}
	m, e := body.tessellate(options)
	if e != nil || len(m.Triangles) != 384 {
		t.Fatal(len(m.Triangles), e)
	}
	options.MaxTriangles = 383
	if _, e = body.tessellate(options); e != parametric.ErrBudget {
		t.Fatal(e)
	}
	options.MaxTriangles = 384
	options.MaxSubdivisions = 16
	if _, e = body.tessellate(options); e != parametric.ErrBudget {
		t.Fatal(e)
	}
	options.MaxSubdivisions = 128
	options.AnalyticBound = true
	if _, e = body.tessellate(options); e != parametric.ErrDeflectionUnsupported {
		t.Fatal(e)
	}
}
