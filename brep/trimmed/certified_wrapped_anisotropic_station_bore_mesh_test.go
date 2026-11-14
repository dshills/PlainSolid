package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedAnisotropicBoreDisplay(t *testing.T) {
	ctx := context.Background()
	s := anisotropicMeasuredBore(t) // asymmetric profile, independent cuts, reindex/reload
	o := CertifiedWrappedStationMeshOptions{Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .12}}}
	m, e := s.TessellateWrappedAnisotropicStationBoreCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !m.Source.Complete || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal("incomplete")
	}
	if m.SourceImageSquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.12), ratInset(.12))) > 0 {
		t.Fatal("distance")
	}
	type incidence struct {
		faces          map[int]bool
		balance, count int
	}
	edges := map[[2]int]*incidence{}
	for i, tri := range m.Mesh.Triangles {
		face := m.OriginalFace[i]
		a, b, c := m.Mesh.Vertices[tri[0]], m.Mesh.Vertices[tri[1]], m.Mesh.Vertices[tri[2]]
		n := b.Sub(a).Cross(c.Sub(a))
		if _, ok := s.faces[face].Surface.(*geom.WrappedSurfacePatch); ok {
			radial := geom.Vec3{X: (a.X+b.X+c.X)/3 - .25, Y: (a.Y+b.Y+c.Y)/3 + .5}
			dot := n.Dot(radial)
			if s.faces[face].Reversed {
				dot = -dot
			}
			if dot <= 0 {
				t.Fatal("wrong original wall orientation", face)
			}
		} else {
			want := 1.0
			if face == m.Source.Caps[0] {
				want = -1
			}
			if n.Z*want <= 0 {
				t.Fatal("cap normal")
			}
		}
		for j, x := range tri {
			y := tri[(j+1)%3]
			sign := 1
			if x > y {
				x, y = y, x
				sign = -1
			}
			k := [2]int{x, y}
			if edges[k] == nil {
				edges[k] = &incidence{faces: map[int]bool{}}
			}
			edges[k].faces[face] = true
			edges[k].balance += sign
			edges[k].count++
		}
	}
	if len(m.Mesh.Vertices)-len(edges)+len(m.Mesh.Triangles) != 0 {
		t.Fatal("genus one")
	}
	for _, v := range edges {
		if v.balance != 0 || v.count != 2 {
			t.Fatal("oriented incidence")
		}
	}
	walls := [2][]int{m.Source.OuterWalls, m.Source.InnerWalls}
	heights := [2][]*big.Rat{m.Source.Outer.Heights, m.Source.Inner.Heights}
	for side, stations := range m.StationVertices {
		if len(stations) != 3 {
			t.Fatal("source station count")
		}
		for station, ring := range stations {
			want := map[int]bool{}
			if station == 0 {
				want[walls[side][0]] = true
				want[m.Source.Caps[0]] = true
			} else if station == len(stations)-1 {
				want[walls[side][station-1]] = true
				want[m.Source.Caps[1]] = true
			} else {
				want[walls[side][station-1]] = true
				want[walls[side][station]] = true
			}
			for j, x := range ring {
				y := ring[(j+1)%len(ring)]
				if x > y {
					x, y = y, x
				}
				v := edges[[2]int{x, y}]
				if len(v.faces) != 2 {
					t.Fatal("shared indexed rim")
				}
				for id := range want {
					if !v.faces[id] {
						t.Fatal("original rim owners")
					}
				}
				if ratInset(m.Mesh.Vertices[x].Z).Cmp(heights[side][station]) != 0 {
					t.Fatal("station moved")
				}
			}
			// Each original quarter edge owns the exact station display subdivision.
			for quarter, id := range m.StationRimEdges[side][station] {
				edge := s.edges[id]
				curve, _, e := exactEdgeSplineWithOptions(edge, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for k := 0; k < m.Subdivisions; k++ {
					u := edge.Range.Min + (edge.Range.Max-edge.Range.Min)*float64(k)/float64(m.Subdivisions)
					p, e := curve.Eval(u)
					if e != nil {
						t.Fatal(e)
					}
					got := m.Mesh.Vertices[ring[quarter*m.Subdivisions+k]]
					if p.Distance(got) > 1e-12 {
						t.Fatal("original source rim sample")
					}
				}
			}
		}
	}
	// Independently sample the exact annular chart at each cap triangle's
	// parameter barycenter, rather than accepting a polygon area comparison.
	for cap, id := range m.Source.Caps {
		station := 0
		if cap == 1 {
			station = 2
		}
		section, e := s.SectionWrappedAnisotropicStationBoreRational(ctx, heights[0][station], geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		parameters := map[int][2]float64{}
		for side := 0; side < 2; side++ {
			for j, vertex := range m.StationVertices[side][station] {
				edge := s.edges[m.StationRimEdges[side][station][j/m.Subdivisions]]
				u := edge.Range.Min + (edge.Range.Max-edge.Range.Min)*float64(j%m.Subdivisions)/float64(m.Subdivisions)
				parameters[vertex] = [2]float64{u, float64(1 - side)}
			}
		}
		for i, tri := range m.Mesh.Triangles {
			if m.OriginalFace[i] != id {
				continue
			}
			var uv [3][2]float64
			lo, hi := math.Inf(1), math.Inf(-1)
			for j, v := range tri {
				uv[j] = parameters[v]
				lo = math.Min(lo, uv[j][0])
				hi = math.Max(hi, uv[j][0])
			}
			u, v := 0.0, 0.0
			display := geom.Vec3{}
			for j, vertex := range tri {
				if hi-lo > 5 && uv[j][0] == -2 {
					uv[j][0] = 8
				}
				u += uv[j][0] / 3
				v += uv[j][1] / 3
				display = display.Add(m.Mesh.Vertices[vertex].Scale(1.0 / 3))
			}
			a, e := section.Outer.Profile.Curve.Eval(u)
			if e != nil {
				t.Fatal(e)
			}
			b, e := section.Inner.Profile.Curve.Eval(u)
			if e != nil {
				t.Fatal(e)
			}
			model := a.Scale(v).Add(b.Scale(1 - v))
			if model.Distance(display) > .12+1e-12 {
				t.Fatal("independent cap residual")
			}
		}
	}
	truth := independentAnisotropicBoreMoments(2048)[0]
	got, _ := m.SignedDisplayVolume.Float64()
	if math.Abs(got-truth) > .04*truth {
		t.Fatal("independent volume diagnostic", got, truth)
	}
	t.Log("vertices triangles subdivisions checks samples volume", len(m.Mesh.Vertices), len(m.Mesh.Triangles), m.Subdivisions, m.TriangleChecks, m.StationSamples, got, truth)
	for _, bad := range []CertifiedWrappedStationMeshOptions{
		{Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .12, MaxTriangleChecks: 1}}},
		{Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .12}, MaxTriangles: 8}},
		{Band: o.Band, MaxStationSamples: 1},
	} {
		q, e := s.TessellateWrappedAnisotropicStationBoreCertified(ctx, bad)
		if e == nil || len(q.Mesh.Vertices) != 0 || q.Source.Complete {
			t.Fatal("atomic budget", e)
		}
	}
}

func TestCertifiedAnisotropicBoreCloseClearance(t *testing.T) {
	ctx := context.Background()
	makeLoft := func(lower, upper [2]float64, knots bool) Solid {
		w := anisotropicStationBand(t, -1, 1, lower, upper, .75)
		if knots {
			recipe, e := w.ExportRecipe()
			if e != nil {
				t.Fatal(e)
			}
			d := recipe.Surface.Data()
			for i := range d.KnotsU {
				d.KnotsU[i] = 2*d.KnotsU[i] + 1
			}
			raw, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
			if e != nil {
				t.Fatal(e)
			}
			patch, e := geom.NewSurfacePatch(raw, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			w, _, e = geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: true})
			if e != nil {
				t.Fatal(e)
			}
		}
		s, _, e := NewWrappedAnisotropicStationLoft(ctx, []*geom.WrappedSurfacePatch{w}, 1e-8, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	outer := makeLoft([2]float64{3, 1}, [2]float64{2, 3}, false)
	inner := makeLoft([2]float64{3 - 1.0/64, 1 - 1.0/64}, [2]float64{2 - 1.0/64, 3 - 1.0/64}, false)
	s, _, e := CutWrappedAnisotropicStationLofts(ctx, outer, inner, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := CertifiedWrappedStationMeshOptions{Band: CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .2}}}
	m, e := s.TessellateWrappedAnisotropicStationBoreCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if m.Source.MinimumScaleGap[0].Cmp(big.NewRat(1, 64)) != 0 || m.Source.MinimumScaleGap[1].Cmp(big.NewRat(1, 64)) != 0 || !m.Collision.Complete || len(m.Collision.Contacts) != 0 || m.SignedDisplayVolume.Sign() <= 0 {
		t.Fatal("close gap proof")
	}
	t.Log("close gap triangles", len(m.Mesh.Triangles))
	// Different circumferential bases describe valid original nested material,
	// but this shared-parameter mesher must reject rather than weld by position.
	inner = makeLoft([2]float64{.5, .25}, [2]float64{.25, .5}, true)
	s, _, e = CutWrappedAnisotropicStationLofts(ctx, outer, inner, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bad, e := s.TessellateWrappedAnisotropicStationBoreCertified(ctx, o)
	if e == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("mismatched U gate", e)
	}
}
