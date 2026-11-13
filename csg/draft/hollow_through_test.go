package draft

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestHollowThroughIndependentDuct(t *testing.T) {
	ctx := context.Background()
	base, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
	if e != nil {
		t.Fatal(e)
	}
	for _, shear := range []bool{false, true} {
		s := base
		if shear {
			s, e = s.Transformed(geom.Affine{{1, 0, .5, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
			if e != nil {
				t.Fatal(e)
			}
		}
		thickness := []float64{0, 0, .25, .5, .75, 1}
		before := append([]float64(nil), thickness...)
		o := ThroughHollowOptions{OpenFaces: [2]int{0, 1}, CoordinateThickness: shear}
		r, e := HollowThrough(ctx, s, thickness, o)
		if e != nil {
			t.Fatal(shear, e)
		}
		if !reflect.DeepEqual(thickness, before) {
			t.Fatal("mutated input")
		}
		checkVolume(t, r.Interior, 60, 1)
		if r.ExactVolume.Cmp(big.NewRat(60, 1)) != 0 {
			t.Fatal("exact material volume", r.ExactVolume)
		}
		for i, id := range r.InteriorOpeningFaces {
			if id != r.Interior.OldToNew[r.OpenFaces[i]] || id < 0 {
				t.Fatal("opening ownership")
			}
		}

		m, e := r.Solid.MassProperties()
		center := geom.Vec3{X: 1.75, Y: 2.75, Z: 3}
		if shear {
			center.X += 1.5
		}
		if e != nil || math.Abs(m.Volume-60) > 1e-10 || m.Centroid.Sub(center).Length() > 1e-10 {
			t.Fatal("independent mass", shear, m, e)
		}
		if !shear && math.Abs(r.Solid.Tessellate().Area()-206) > 1e-10 {
			t.Fatal("independent area", r.Solid.Tessellate().Area())
		}
		top := r.Solid.Topology()
		if top.Shells != 1 || top.EulerCharacteristic != 0 {
			t.Fatal("through topology", top)
		}
		inherited, inner := false, false
		for _, f := range r.History.Faces {
			if f.Operand == poly.OperandB {
				inner = true
				if !f.Reversed {
					t.Fatal("cavity orientation")
				}
			} else {
				inherited = true
			}
		}
		if !inner || !inherited {
			t.Fatal("missing ancestry")
		}
		mesh := r.Solid.Tessellate()
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("contact", e)
		}
		o.OpenFaces = [2]int{1, 0}
		reverse, e := HollowThrough(ctx, s, thickness, o)
		if e != nil || !reflect.DeepEqual(reverse.Solid.Vertices(), r.Solid.Vertices()) || !reflect.DeepEqual(reverse.Solid.Faces(), r.Solid.Faces()) {
			t.Fatal("opening order", e)
		}
	}
}
func TestHollowThroughRejectsUnprovedOpenings(t *testing.T) {
	s := cube(t)
	ctx := context.Background()
	for _, ends := range [][2]int{{0, 0}, {-1, 1}, {0, 6}, {0, 2}} {
		th := []float64{.125, .125, .125, .125, .125, .125}
		for _, id := range ends {
			if id >= 0 && id < len(th) {
				th[id] = 0
			}
		}
		if r, e := HollowThrough(ctx, s, th, ThroughHollowOptions{OpenFaces: ends}); e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("bad openings", ends, e)
		}
	}
	for _, th := range [][]float64{{0, 0, 1, 1, 1, 1}, {0, 0, 0, .125, .125, .125}, {0, .125, .125, .125, .125, .125}, {0, 0, math.NaN(), .125, .125, .125}} {
		if _, e := HollowThrough(ctx, s, th, ThroughHollowOptions{OpenFaces: [2]int{0, 1}}); e == nil {
			t.Fatal("invalid cavity", th)
		}
	}
	th := []float64{0, 0, .125, .125, .125, .125}
	if _, e := HollowThrough(ctx, s, th, ThroughHollowOptions{OpenFaces: [2]int{0, 1}, Reconstruction: Options{MaxWork: 1}}); !errors.Is(e, ErrLimit) {
		t.Fatal("budget", e)
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := HollowThrough(c, s, th, ThroughHollowOptions{OpenFaces: [2]int{0, 1}}); !errors.Is(e, context.Canceled) {
		t.Fatal("cancel", e)
	}
}
func TestHollowThroughRejectsPartialSupportingFace(t *testing.T) {
	m := cube(t).Tessellate()
	var edges []brep.Edge
	var faces []brep.Face
	lookup := map[[2]int]int{}
	for _, tri := range m.Triangles {
		var uses []brep.EdgeUse
		for i, a := range tri {
			b := tri[(i+1)%3]
			key := [2]int{min(a, b), max(a, b)}
			id, ok := lookup[key]
			if !ok {
				id = len(edges)
				lookup[key] = id
				edges = append(edges, brep.Edge{Start: key[0], End: key[1]})
			}
			uses = append(uses, brep.EdgeUse{Edge: id, Reversed: a != key[0]})
		}
		faces = append(faces, brep.Face{Boundary: uses})
	}
	s, e := brep.NewSolid(m.Vertices, edges, faces)
	if e != nil {
		t.Fatal(e)
	}
	th := make([]float64, len(faces))
	for i := range th {
		th[i] = .125
	}
	th[0], th[2] = 0, 0
	if _, e = HollowThrough(context.Background(), s, th, ThroughHollowOptions{OpenFaces: [2]int{0, 2}}); !errors.Is(e, ErrUnsupported) {
		t.Fatal("partial coplanar support", e)
	}
}
func TestHollowThroughRejectsRoundedOpeningPlane(t *testing.T) {
	// Every original corner is exact. The offset interior's upper cap includes
	// (x,z)=(binary64(.1),1+binary64(.1)/2), whose z is not binary64.
	// The rounded chart is planar by itself but no longer on the ORIGINAL cap.
	s, e := cube(t).Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {.5, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	exactZ := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Quo(new(big.Rat).SetFloat64(.1), big.NewRat(2, 1)))
	rounded, exact := exactZ.Float64()
	if exact {
		t.Fatal("adversary unexpectedly representable")
	}
	error := new(big.Rat).Abs(new(big.Rat).Sub(exactZ, new(big.Rat).SetFloat64(rounded)))
	ulp := new(big.Rat).Sub(new(big.Rat).SetFloat64(math.Nextafter(rounded, math.Inf(1))), new(big.Rat).SetFloat64(rounded))
	if error.Sign() == 0 || error.Cmp(ulp) >= 0 {
		t.Fatal("expected less than one ULP", error, ulp)
	}
	r, e := HollowThrough(context.Background(), s, []float64{0, 0, .125, .1, .125, .1}, ThroughHollowOptions{OpenFaces: [2]int{0, 1}, CoordinateThickness: true})
	if !errors.Is(e, ErrPrecision) || len(r.Solid.Faces()) != 0 {
		t.Fatal("rounded cap left exact source plane", e)
	}
}
