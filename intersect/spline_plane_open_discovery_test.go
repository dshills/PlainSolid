package intersect

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestDiscoverOpenCurvedPlaneSection(t *testing.T) {
	for _, oblique := range []bool{false, true} {
		var poles []geom.Vec3
		for _, u := range []float64{0, 1} {
			for i, z := range []float64{.25, .375, .25} {
				height := u - z
				if oblique {
					height += u
				}
				poles = append(poles, geom.Vec3{X: u, Y: float64(i) / 2, Z: height})
			}
		}
		s, e := geom.NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		var plane []geom.Vec3
		for _, u := range []float64{-.25, 1.25} {
			for _, v := range []float64{-.25, 1.25} {
				z := 0.
				if oblique {
					z = u
				}
				plane = append(plane, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		k := []float64{-.25, -.25, 1.25, 1.25}
		p, e := geom.NewBSplineSurface(1, 1, 2, 2, plane, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		a, b := Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: p, Min: geom.Vec2{X: -.25, Y: -.25}, Max: geom.Vec2{X: 1.25, Y: 1.25}}
		for swapped, pair := range [][2]Patch{{a, b}, {b, a}} {
			r, e := DiscoverOpenSplinePlanePairTubes(context.Background(), pair[0], pair[1], geom.SurfacePatchOptions{}, SplinePlanePairDiscoveryOptions{})
			if e != nil || r.OpenContours != 1 || len(r.Terminals) != 2 {
				t.Fatal(oblique, swapped, r, e)
			}
			for _, terminal := range r.Terminals {
				if !terminal.Certificate.Proven || terminal.Parameters[2*swapped].Cmp(big.NewRat(1, 4)) != 0 {
					t.Fatal(terminal)
				}
			}
			proof, e := CertifySplinePairBranches(context.Background(), pair[0], pair[1], r.Seeds, SplinePairTraceOptions{})
			if e != nil || !proof.Complete || len(proof.Curves) != 1 {
				t.Fatal(proof, e)
			}
		}
		for _, upper := range []float64{.75, 1.25} {
			clipped := b
			clipped.Min.Y = .25
			clipped.Max.Y = upper
			for swapped, pair := range [][2]Patch{{a, clipped}, {clipped, a}} {
				r, e := DiscoverOpenSplinePlanePairTubes(context.Background(), pair[0], pair[1], geom.SurfacePatchOptions{}, SplinePlanePairDiscoveryOptions{})
				if e != nil || r.OpenContours != 1 || len(r.Terminals) != 2 {
					t.Fatal("plane side", oblique, upper, swapped, r, e)
				}
				planeEnd, curvedEnd := false, false
				for _, terminal := range r.Terminals {
					if !terminal.Certificate.Proven {
						t.Fatal(terminal)
					}
					v := terminal.Parameters[1+2*swapped]
					if v.Cmp(big.NewRat(1, 4)) == 0 || v.Cmp(big.NewRat(3, 4)) == 0 {
						planeEnd = true
					}
					if v.Cmp(big.NewRat(1, 1)) == 0 {
						curvedEnd = true
					}
				}
				if !planeEnd || (upper > 1 && !curvedEnd) {
					t.Fatal(r)
				}
				proof, e := CertifySplinePairBranches(context.Background(), pair[0], pair[1], r.Seeds, SplinePairTraceOptions{})
				if e != nil || !proof.Complete || len(proof.Curves) != 1 {
					t.Fatal("plane side coverage", upper, swapped, proof, e)
				}
			}
		}
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		_, e = DiscoverOpenSplinePlanePairTubes(canceled, a, b, geom.SurfacePatchOptions{}, SplinePlanePairDiscoveryOptions{})
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		_, e = DiscoverOpenSplinePlanePairTubes(context.Background(), a, b, geom.SurfacePatchOptions{}, SplinePlanePairDiscoveryOptions{Tubes: geom.SurfaceArcTubeDiscoveryOptions{MaxOperations: 1}})
		if e == nil {
			t.Fatal("budget accepted")
		}
	}
}

func TestDiscoverMixedClosedOpenPlaneSections(t *testing.T) {
	// z=3*(u-7/8)*((u-3/8)^2+(v-1/2)^2-(5/32)^2).
	// Scaling by three makes every cubic Bernstein coefficient dyadic; model
	// x=3u likewise has exactly represented cubic affine controls.
	cu := []float64{9.0 / 64, -15.0 / 64, 25.0 / 64}
	cv := []float64{.25, -.25, .25}
	var poles []geom.Vec3
	for i := 0; i < 4; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i > 0 {
				z += float64(i) * .125 * (cu[i-1] + cv[j] - 25.0/1024)
			}
			if i < 3 {
				z -= float64(3-i) * .875 * (cu[i] + cv[j] - 25.0/1024)
			}
			poles = append(poles, geom.Vec3{X: float64(i), Y: float64(j) / 2, Z: z})
		}
	}
	s, e := geom.NewBSplineSurface(3, 2, 4, 3, poles, []float64{0, 0, 0, 0, 1, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	k := []float64{-.25, -.25, 1.25, 1.25}
	p, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: -.75, Y: -.25}, {X: -.75, Y: 1.25}, {X: 3.75, Y: -.25}, {X: 3.75, Y: 1.25}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, b := Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: p, Min: geom.Vec2{X: -.25, Y: -.25}, Max: geom.Vec2{X: 1.25, Y: 1.25}}
	o := SplinePlanePairDiscoveryOptions{UCuts: []float64{.25, .28125, .46875, .5}, VCuts: []float64{.375, .40625, .59375, .625}}
	o.Plane.Arcs.ResolveSubdivisionCorners = true
	r, e := DiscoverSplinePlanePairSections(context.Background(), a, b, geom.SurfacePatchOptions{}, o)
	if e != nil || r.OpenContours != 1 || r.ClosedContours != 1 || len(r.Terminals) != 2 || len(r.Seeds.Attempts) == 0 {
		t.Fatal(r.OpenContours, r.ClosedContours, len(r.Terminals), len(r.Seeds.Attempts), e)
	}
	_, e = DiscoverOpenSplinePlanePairTubes(context.Background(), a, b, geom.SurfacePatchOptions{}, o)
	if !errors.Is(e, ErrUnresolved) {
		t.Fatal("strict open accepted mixed contours", e)
	}
}
