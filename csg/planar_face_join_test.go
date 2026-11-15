package csg

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

func joinPrism(t *testing.T, x, z float64, shift int) brep.Solid {
	t.Helper()
	p := []geom.Vec3{{X: x, Z: z}, {X: x + 3, Z: z}, {X: x + 3, Y: 1, Z: z}, {X: x + 1, Y: 1, Z: z}, {X: x + 1, Y: 3, Z: z}, {X: x, Y: 3, Z: z}}
	loop := append(append([]geom.Vec3{}, p[shift:]...), p[:shift]...)
	s, err := brep.Prism(loop, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestJoinPlanarFacePairPreservesConcaveSourceTopology(t *testing.T) {
	ctx := context.Background()
	a, b := joinPrism(t, 0, 0, 0), joinPrism(t, 0, 1, 3)
	// The stored edge orientation is arbitrary and can differ even for
	// geometrically identical original edges.
	be, bf := b.Edges(), b.Faces()
	for i := range be {
		be[i].Start, be[i].End = be[i].End, be[i].Start
	}
	for i := range bf {
		for j := range bf[i].Boundary {
			bf[i].Boundary[j].Reversed = !bf[i].Boundary[j].Reversed
		}
	}
	var err error
	b, err = brep.NewSolid(b.Vertices(), be, bf)
	if err != nil {
		t.Fatal(err)
	}
	s, h, err := JoinPlanarFacePair(ctx, a, b, 1, 0, collision.PlanarFaceContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	v, e, f := s.Vertices(), s.Edges(), s.Faces()
	if len(v) != 18 || len(e) != 30 || len(f) != 14 || h.Contact.SignedVolume.RatString() != "10" {
		t.Fatalf("unexpected original topology: %d vertices, %d edges, %d faces", len(v), len(e), len(f))
	}
	m, err := s.MassProperties()
	if err != nil || math.Abs(m.Volume-10) > 1e-14 || math.Abs(m.SurfaceArea-34) > 1e-14 {
		t.Fatalf("incorrect union measures: %+v %v", m, err)
	}
	fOwners := make(map[int]int)
	for owner, source := range [2]brep.Solid{a, b} {
		m := h.Owners[owner]
		for i, p := range source.Vertices() {
			if v[m.Vertices[i]] != p {
				t.Fatal("moved original vertex", owner, i)
			}
		}
		for i, old := range source.Edges() {
			use := m.Edges[i]
			mapped := e[use.Edge]
			if use.Reversed {
				mapped.Start, mapped.End = mapped.End, mapped.Start
			}
			if mapped != (brep.Edge{Start: m.Vertices[old.Start], End: m.Vertices[old.End]}) {
				t.Fatal("lost original edge direction", owner, i)
			}
		}
		for i, old := range source.Faces() {
			id := m.Faces[i]
			if i == 1-owner {
				if id != -1 {
					t.Fatal("interface face survived", owner, i)
				}
				continue
			}
			fOwners[id]++
			for j, u := range old.Boundary {
				want := m.Edges[u.Edge]
				want.Reversed = want.Reversed != u.Reversed
				if f[id].Boundary[j] != want {
					t.Fatal("lost original oriented use", owner, i, j)
				}
			}
		}
	}
	if len(fOwners) != len(f) {
		t.Fatal("unexpected new or missing face", fOwners)
	}
	for id, n := range fOwners {
		if n != 1 {
			t.Fatal("unexpected face owner multiplicity", id, n)
		}
	}
	loc, err := collision.NewPointLocator(ctx, s, collision.LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		p    geom.Vec3
		want collision.Location
	}{
		{geom.Vec3{X: .5, Y: .5, Z: 1}, collision.Inside},
		{geom.Vec3{X: 2, Y: 2, Z: 1}, collision.Outside},
		{geom.Vec3{X: 1, Y: 2, Z: 1}, collision.OnBoundary},
	} {
		r, err := loc.Locate(ctx, test.p)
		if err != nil || r.Location != test.want {
			t.Fatalf("point %v: got %+v %v, want %v", test.p, r, err, test.want)
		}
	}
}

func TestJoinPlanarFacePairRejectsPartialOverlap(t *testing.T) {
	a, b := joinPrism(t, 0, 0, 0), joinPrism(t, .25, 1, 3)
	s, h, err := JoinPlanarFacePair(context.Background(), a, b, 1, 0, collision.PlanarFaceContactOptions{})
	if !errors.Is(err, collision.ErrInvalid) || len(s.Faces()) != 0 || h.Contact.SignedVolume != nil {
		t.Fatalf("partial face published a union or history: %v, %+v, %v", s, h, err)
	}
}

// pocketJoinStock independently models a square prism with a blind recess.
// The recess opens toward the join; the opposite cap closes the outer stock.
func pocketJoinStock(t *testing.T, lower, shiftHole bool) brep.Solid {
	t.Helper()
	z := 0.0
	if !lower {
		z = 1
	}
	loop := func(min, max, offset float64) []geom.Vec3 {
		return []geom.Vec3{{X: min + offset, Y: min, Z: z}, {X: max + offset, Y: min, Z: z}, {X: max + offset, Y: max, Z: z}, {X: min + offset, Y: max, Z: z}}
	}
	shift := 0.0
	if shiftHole {
		shift = .25
	}
	s, err := brep.PrismWithHoles(loop(0, 4, 0), [][]geom.Vec3{loop(1, 3, shift)}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	v, e, f := s.Vertices(), s.Edges(), s.Faces()
	closeEnd, depth := 0, .25
	if !lower {
		closeEnd, depth = 1, 1.75
	}
	bottom := f[closeEnd].Holes[0]
	for _, u := range bottom {
		v[e[u.Edge].Start].Z = depth
		v[e[u.Edge].End].Z = depth
	}
	f[closeEnd].Holes = nil
	f = append(f, brep.Face{Boundary: bottom})
	s, err = brep.NewSolid(v, e, f)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestJoinPlanarFacePairClosesAnOriginalCavity(t *testing.T) {
	ctx := context.Background()
	a, b := pocketJoinStock(t, true, false), pocketJoinStock(t, false, false)
	s, h, err := JoinPlanarFacePair(ctx, a, b, 1, 0, collision.PlanarFaceContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p := h.Contact
	if len(p.WireOffsets) != 3 || p.WireOffsets[0] != 0 || p.WireOffsets[1] != 4 || p.WireOffsets[2] != 8 ||
		p.TwiceProjectedArea.RatString() != "24" || p.SignedVolume.RatString() != "26" || len(s.Faces()) != 20 {
		t.Fatalf("annular join lost matched inner and outer source wires: %+v; %d faces", p, len(s.Faces()))
	}
	m, err := s.MassProperties()
	if err != nil || math.Abs(m.Volume-26) > 1e-13 || math.Abs(m.SurfaceArea-84) > 1e-13 {
		t.Fatalf("incorrect joined cavity measures: %+v %v", m, err)
	}
	loc, err := collision.NewPointLocator(ctx, s, collision.LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	shells := loc.Shells()
	if len(shells) != 2 || shells[0].Depth != 0 || shells[1].Parent != 0 || shells[1].Depth != 1 || shells[1].SignedVolume.RatString() != "-6" {
		t.Fatalf("missing independently validated inward cavity shell: %+v", shells)
	}
	for _, test := range []struct {
		p    geom.Vec3
		want collision.Location
	}{
		{geom.Vec3{X: 2, Y: 2, Z: 1}, collision.Outside},
		{geom.Vec3{X: .5, Y: 2, Z: 1}, collision.Inside},
		{geom.Vec3{X: 1, Y: 2, Z: 1}, collision.OnBoundary},
		{geom.Vec3{X: 2, Y: 2, Z: .1}, collision.Inside},
	} {
		r, err := loc.Locate(ctx, test.p)
		if err != nil || r.Location != test.want {
			t.Fatalf("%v: %+v %v; want %v", test.p, r, err, test.want)
		}
	}
	s, h, err = JoinPlanarFacePair(ctx, a, pocketJoinStock(t, false, true), 1, 0, collision.PlanarFaceContactOptions{})
	if !errors.Is(err, collision.ErrInvalid) || len(s.Faces()) != 0 || h.Contact.SignedVolume != nil {
		t.Fatalf("mismatched recess exported a joined body: %+v %v", h, err)
	}
}
