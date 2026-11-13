package draft

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func inclinedDuctStock(t *testing.T, topWidth, height float64) (brep.Solid, [2]int) {
	t.Helper()
	profile := []geom.Vec3{{}, {X: 4}, {X: topWidth, Z: height}, {Z: height}}
	if topWidth == 0 {
		profile = profile[:3]
	}
	s, e := brep.Prism(profile, geom.Vec3{Y: 2})
	if e != nil {
		t.Fatal(e)
	}
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	ends := [2]int{-1, -1}
	for id, f := range faces {
		left, right := true, true
		for _, u := range f.Boundary {
			p := v[edges[u.Edge].Start]
			left = left && p.X == 0
			right = right && (p.X == 4 && p.Z == 0 || p.X == topWidth && p.Z == height)
		}
		if left {
			ends[0] = id
		}
		if right {
			ends[1] = id
		}
	}
	if ends[0] < 0 || ends[1] < 0 {
		t.Fatal("fixture opening IDs", ends)
	}
	return s, ends
}
func TestHollowThroughNonparallelEnds(t *testing.T) {
	ctx := context.Background()
	s, ends := inclinedDuctStock(t, 3, 1)
	th := make([]float64, len(s.Faces()))
	for i := range th {
		th[i] = .25
	}
	th[ends[0]], th[ends[1]] = 0, 0
	r, e := HollowThrough(ctx, s, th, ThroughHollowOptions{OpenFaces: ends})
	if e != nil {
		t.Fatal(e)
	}
	if r.ExactVolume.Cmp(big.NewRat(35, 8)) != 0 || r.Interior.ExactVolume.Cmp(big.NewRat(21, 8)) != 0 {
		t.Fatal("exact wedge subtraction", r.ExactVolume, r.Interior.ExactVolume)
	}
	m, e := r.Solid.MassProperties()
	center := geom.Vec3{X: 2969. / 1680, Y: 1, Z: 391. / 840}
	if e != nil || math.Abs(m.Volume-35./8) > 1e-11 || m.Centroid.Sub(center).Length() > 1e-11 {
		t.Fatal("independent integral", m, e)
	}
	mesh := r.Solid.Tessellate()
	if math.Abs(mesh.Area()-(145+5*math.Sqrt2)/4) > 1e-10 {
		t.Fatal("independent area", mesh.Area())
	}
	if r.Solid.Topology().Shells != 1 || r.Solid.Topology().EulerCharacteristic != 0 {
		t.Fatal("through topology")
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("display contacts", e)
	}
	for i, id := range r.InteriorOpeningFaces {
		if id < 0 || r.OpenFaces[i] != ends[i] {
			t.Fatal("opening ancestry")
		}
	}
}
func TestHollowThroughWholeOpeningSeparation(t *testing.T) {
	for _, gap := range []float64{math.Ldexp(1, -20), 0} {
		s, ends := inclinedDuctStock(t, gap, 1)
		th := make([]float64, len(s.Faces()))
		for i := range th {
			th[i] = .125
		}
		th[ends[0]], th[ends[1]] = 0, 0
		r, e := HollowThrough(context.Background(), s, th, ThroughHollowOptions{OpenFaces: ends, CoordinateThickness: true})
		if gap == 0 {
			if !errors.Is(e, ErrUnsupported) || len(r.Solid.Faces()) != 0 {
				t.Fatal("shared original edge", e)
			}
		} else if e != nil || r.Solid.Topology().EulerCharacteristic != 0 {
			t.Fatal("strict small source gap", e)
		}
	}
}
