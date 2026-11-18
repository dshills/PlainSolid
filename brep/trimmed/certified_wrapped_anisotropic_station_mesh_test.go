//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedAnisotropicStationMesh(t *testing.T) {
	ctx := context.Background()
	s := anisotropicMassFixture(t, false, geom.Vec3{})
	o := CertifiedWrappedStationMeshOptions{Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .12, MaxSubdivisions: 16, MaxTriangleChecks: 100000}}}
	m, e := s.TessellateWrappedAnisotropicStationLoftCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if !m.Source.Complete || !m.Collision.Complete || len(m.Collision.Contacts) != 0 || m.SignedDisplayVolume.Sign() <= 0 {
		t.Fatal("incomplete")
	}
	if e = m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	owners := map[[2]int]map[int]int{}
	for i, tri := range m.Mesh.Triangles {
		if m.SquaredErrors[i].Sign() < 0 || m.SquaredErrors[i].Cmp(new(big.Rat).Mul(ratInset(.12), ratInset(.12))) > 0 {
			t.Fatal("distance")
		}
		for j, a := range tri {
			b := tri[(j+1)%3]
			sign := 1
			if a > b {
				a, b = b, a
				sign = -1
			}
			key := [2]int{a, b}
			if owners[key] == nil {
				owners[key] = map[int]int{}
			}
			owners[key][m.OriginalFace[i]] += sign
		}
	}
	if len(m.Mesh.Vertices)-len(owners)+len(m.Mesh.Triangles) != 2 {
		t.Fatal("Euler")
	}
	for station, ring := range m.StationVertices {
		want := []int{}
		if station == 0 {
			want = []int{m.Source.History.CapFaces[0], m.Source.History.WallFaces[0]}
		} else if station == len(m.StationVertices)-1 {
			want = []int{m.Source.History.CapFaces[1], m.Source.History.WallFaces[station-1]}
		} else {
			want = []int{m.Source.History.WallFaces[station-1], m.Source.History.WallFaces[station]}
		}
		for i, a := range ring {
			b := ring[(i+1)%len(ring)]
			if a > b {
				a, b = b, a
			}
			got := owners[[2]int{a, b}]
			if len(got) != 2 || got[want[0]] == 0 || got[want[0]]+got[want[1]] != 0 {
				t.Fatal("nonshared or wrong oriented rim")
			}
			if ratInset(m.Mesh.Vertices[a].Z).Cmp(m.Source.Heights[station]) != 0 {
				t.Fatal("station drift")
			}
		}
	}
	// Independent exact original-curve/chord midpoint witnesses on every
	// station, including both sides of the genuine aspect-ratio crease.
	_, bands, e := s.validateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for station, ring := range m.StationVertices {
		band := min(station, len(bands)-1)
		w := bands[band]
		p, e := geom.CertifyWrappedAnisotropicEmbedding(ctx, w, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		domain := w.FundamentalPatch().Domain()
		vv := domain.VMin
		if station == len(bands) {
			vv = domain.VMax
		}
		if len(ring) != 4*m.Subdivisions {
			t.Fatal("unexpected original station schedule")
		}
		for i, id := range ring {
			quarter, j := i/m.Subdivisions, i%m.Subdivisions
			lo, hi := ratInset(p.QuarterParameters[quarter]), ratInset(p.QuarterParameters[quarter+1])
			u := new(big.Rat).Add(lo, new(big.Rat).Mul(new(big.Rat).Sub(hi, lo), big.NewRat(int64(2*j+1), int64(2*m.Subdivisions))))
			q, e := w.FundamentalPatch().EvaluateRational(ctx, u, ratInset(vv), geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			a, c := exactBoundPoint(m.Mesh.Vertices[id]), exactBoundPoint(m.Mesh.Vertices[ring[(i+1)%len(ring)]])
			distance := new(big.Rat)
			for axis := 0; axis < 3; axis++ {
				mid := new(big.Rat).Quo(new(big.Rat).Add(a[axis], c[axis]), big.NewRat(2, 1))
				delta := new(big.Rat).Sub(q.Point[axis], mid)
				distance.Add(distance, new(big.Rat).Mul(delta, delta))
			}
			if distance.Cmp(m.SourceImageSquaredDistance) > 0 {
				t.Fatal("original source witness exceeds bound")
			}
		}
	}
	// Independent Green quadrature of the original weighted profile times
	// integral sx(z)sy(z)dz = 43/6 for (3,1)->(1,2)->(2,1).
	area := rationalQuarterProfileArea(.75, 4096)
	want := area * 43 / 6
	got, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(got-want) > .04*want {
		t.Fatalf("independent volume %g want %g", got, want)
	}
	t.Logf("vertices=%d triangles=%d subdivisions=%d checks=%d volume=%g truth=%g", len(m.Mesh.Vertices), len(m.Mesh.Triangles), m.Subdivisions, m.TriangleChecks, got, want)
	for _, bad := range []CertifiedWrappedStationMeshOptions{{Band: o.Band, MaxStationSamples: 1}, {Band: CertifiedWrappedFrustumMeshOptions{Patch: o.Band.Patch, MaxTriangles: 16}}, {Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .12, MaxTriangleChecks: 1}}}} {
		q, e := s.TessellateWrappedAnisotropicStationLoftCertified(ctx, bad)
		if e == nil || len(q.Mesh.Vertices) != 0 || len(q.OriginalFace) != 0 {
			t.Fatal("partial budget mesh")
		}
	}
	badSource := s
	badSource.faces = append([]Face(nil), s.faces...)
	badSource.faces[m.Source.History.WallFaces[0]].Reversed = true
	if q, e := badSource.TessellateWrappedAnisotropicStationLoftCertified(ctx, o); e == nil || len(q.Mesh.Vertices) != 0 {
		t.Fatal("forged inward wall accepted")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.TessellateWrappedAnisotropicStationLoftCertified(c, o); e == nil {
		t.Fatal("cancel")
	}
}
