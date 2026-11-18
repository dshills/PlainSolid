package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func scaledWrappedCylinderFixture(t *testing.T, w *geom.WrappedSurfacePatch, scale float64) *geom.WrappedSurfacePatch {
	t.Helper()
	s, e := w.FundamentalPatch().ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	d := s.Data()
	for i := range d.Poles {
		d.Poles[i].X *= scale
		d.Poles[i].Y *= scale
	}
	s, e = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	out, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func TestCertifiedWrappedBoreClosed(t *testing.T) {
	slowtest.Skip(t)
	for _, circular := range []bool{true, false} {
		ctx := context.Background()
		outer := certifiedWrappedFrustumFixture(t, circular, 2)
		inner := scaledWrappedCylinderFixture(t, outer, .5)
		o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}}
		m, e := TessellateWrappedBoreClosedCertified(ctx, outer, inner, o)
		if e != nil {
			t.Fatal(e)
		}
		if !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
			t.Fatal("contacts")
		}
		if e := m.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		edges := map[[2]int]int{}
		owners := map[[2]int]map[int]bool{}
		for i, tri := range m.Mesh.Triangles {
			for j, a := range tri {
				b := tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				key := [2]int{a, b}
				edges[key]++
				if owners[key] == nil {
					owners[key] = map[int]bool{}
				}
				owners[key][m.OriginalFace[i]] = true
			}
		}
		if len(m.Mesh.Vertices)-len(edges)+len(m.Mesh.Triangles) != 0 {
			t.Fatal("genus")
		}
		for wall := 0; wall < 2; wall++ {
			for cap := 0; cap < 2; cap++ {
				ring := m.RimVertices[wall][cap]
				for i, a := range ring {
					b := ring[(i+1)%len(ring)]
					if a > b {
						a, b = b, a
					}
					key := [2]int{a, b}
					if edges[key] != 2 || !owners[key][wall] || !owners[key][cap+2] {
						t.Fatal("shared annular rim")
					}
				}
			}
		}

		for cap := 0; cap < 2; cap++ {
			ring := m.RimVertices[1][cap]
			for i := range ring {
				u := big.NewRat(int64(2*i+1), int64(2*m.Subdivisions))
				v := big.NewRat(int64(2*cap-1), 1)
				pi, e := inner.FundamentalPatch().EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				po, e := outer.FundamentalPatch().EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				j := (i + 1) % len(ring)
				a, b, c := exactBoundPoint(m.Mesh.Vertices[ring[i]]), exactBoundPoint(m.Mesh.Vertices[ring[j]]), exactBoundPoint(m.Mesh.Vertices[m.RimVertices[0][cap][j]])
				squared := new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					source := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(5, 8), pi.Point[axis]), new(big.Rat).Mul(big.NewRat(3, 8), po.Point[axis]))
					display := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(1, 2), a[axis]), new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(1, 8), b[axis]), new(big.Rat).Mul(big.NewRat(3, 8), c[axis])))
					d := new(big.Rat).Sub(source, display)
					squared.Add(squared, new(big.Rat).Mul(d, d))
				}
				if squared.Cmp(m.SourceImageSquaredDistance) > 0 {
					t.Fatal("radial cap interpolation certificate")
				}
			}
		}
		area := math.Pi
		if !circular {
			area = 3.1748736330733568
		} // Independent dense Green integral of rational quadrant.
		want := 15 * area
		got, _ := m.SignedDisplayVolume.Float64()
		if math.Abs(got-want) > .02*want {
			t.Fatal(got, want)
		}
		if m.SourceImageSquaredDistance.Cmp(ratInset(.0025)) > 0 {
			t.Fatal("error bound")
		}
		t.Log(circular, len(m.Mesh.Vertices), len(m.Mesh.Triangles), m.Subdivisions, m.TriangleChecks, got, want)
		o.Patch.MaxTriangleChecks = 1
		bad, e := TessellateWrappedBoreClosedCertified(ctx, outer, inner, o)
		if e == nil || len(bad.Mesh.Vertices) != 0 {
			t.Fatal("partial budget", e)
		}
	}
}
func TestCertifiedWrappedBoreRejectsMismatch(t *testing.T) {
	outer := certifiedWrappedFrustumFixture(t, true, 2)
	different := scaledWrappedCylinderFixture(t, certifiedWrappedFrustumFixture(t, false, 2), .5)
	o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}}
	for _, inner := range []*geom.WrappedSurfacePatch{different, outer, scaledWrappedCylinderFixture(t, outer, 2)} {
		m, e := TessellateWrappedBoreClosedCertified(context.Background(), outer, inner, o)
		if e == nil || len(m.Mesh.Vertices) != 0 {
			t.Fatal("invalid nesting/profile", e)
		}
	}
}

func TestCertifiedWrappedBoreIndependentTapers(t *testing.T) {
	slowtest.Skip(t)
	for _, circular := range []bool{true, false} {
		outer := certifiedWrappedFrustumFixture(t, circular, 1.5)
		inner := scaledWrappedCylinderFixture(t, certifiedWrappedFrustumFixture(t, circular, .5), .5)
		o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}}
		m, e := TessellateWrappedBoreClosedCertified(context.Background(), outer, inner, o)
		if e != nil {
			t.Fatal(e)
		}
		area := math.Pi
		if !circular {
			area = 3.1748736330733568
		}
		// Independent Simpson integral of the quadratic area difference over height.
		bottom := 4.0 - 1
		middle := 1.75*1.75 - .625*.625
		top := 1.5*1.5 - .25*.25
		want := area * 5 * (bottom + 4*middle + top) / 6
		got, _ := m.SignedDisplayVolume.Float64()
		if math.Abs(got-want) > .02*want || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
			t.Fatal(got, want, m.Collision)
		}
		if m.Walls[0].LowerRadius.Cmp(big.NewRat(2, 1)) != 0 || m.Walls[0].UpperRadius.Cmp(big.NewRat(3, 2)) != 0 || m.Walls[1].UpperRadius.Cmp(big.NewRat(1, 4)) != 0 {
			t.Fatal("independent taper ancestry")
		}
		t.Log(circular, len(m.Mesh.Triangles), m.TriangleChecks, got, want)
	}
	// These walls are correctly ordered below, but cross before the upper cap.
	outer := certifiedWrappedFrustumFixture(t, true, .5)
	inner := scaledWrappedCylinderFixture(t, certifiedWrappedFrustumFixture(t, true, 1.5), .5)
	m, e := TessellateWrappedBoreClosedCertified(context.Background(), outer, inner, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}})
	if e == nil || len(m.Mesh.Vertices) != 0 {
		t.Fatal("crossing walls accepted", e)
	}
}
