package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedWrappedConeTruePole(t *testing.T) {
	for _, circular := range []bool{true, false} {
		ctx := context.Background()
		w := certifiedWrappedFrustumFixture(t, circular, 0)
		o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}}
		m, e := TessellateWrappedConeClosedCertified(ctx, w, o)
		if e != nil {
			t.Fatal(e)
		}
		if m.Mesh.Vertices[m.ApexVertex] != (geom.Vec3{Z: 3}) || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
			t.Fatal("pole/embedding")
		}
		n := len(m.RimVertices)
		if len(m.Mesh.Vertices) != n+2 || len(m.Mesh.Triangles) != 2*n || m.Mesh.SignedVolume() <= 0 {
			t.Fatal("topology")
		}
		link := map[int]bool{}
		for _, tri := range m.Mesh.Triangles {
			has := false
			for _, id := range tri {
				has = has || id == m.ApexVertex
			}
			if has {
				for _, id := range tri {
					if id != m.ApexVertex {
						link[id] = true
					}
				}
			}
		}
		if len(link) != n {
			t.Fatal("apex link")
		}
		if e := m.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		volume, _ := m.SignedDisplayVolume.Float64()
		if circular && math.Abs(volume-20*math.Pi/3) > .02*20*math.Pi/3 {
			t.Fatal(volume)
		}
		// Exact source-point/triangle correspondence at quarter-cell midpoints,
		// including nonbinary homogeneous evaluation and a point close to the apex.
		for i, a := range m.RimVertices {
			b := m.RimVertices[(i+1)%n]
			u := new(big.Rat).Quo(new(big.Rat).Add(ratInset(m.Profile.Lateral.UParameters[i]), ratInset(m.Profile.Lateral.UParameters[i+1])), big.NewRat(2, 1))
			for _, fraction := range []*big.Rat{big.NewRat(0, 1), big.NewRat(3, 8), big.NewRat(1023, 1024), big.NewRat(1, 1)} {
				v := new(big.Rat).Add(big.NewRat(-1, 1), new(big.Rat).Mul(big.NewRat(2, 1), fraction))
				p, e := w.FundamentalPatch().EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				pa, pb, apex := exactBoundPoint(m.Mesh.Vertices[a]), exactBoundPoint(m.Mesh.Vertices[b]), exactBoundPoint(m.Mesh.Vertices[m.ApexVertex])
				squared := new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					chord := new(big.Rat).Quo(new(big.Rat).Add(pa[axis], pb[axis]), big.NewRat(2, 1))
					display := new(big.Rat).Add(chord, new(big.Rat).Mul(fraction, new(big.Rat).Sub(apex[axis], chord)))
					d := new(big.Rat).Sub(display, p.Point[axis])
					squared.Add(squared, new(big.Rat).Mul(d, d))
				}
				if squared.Cmp(m.SourceImageSquaredDistance) > 0 {
					t.Fatal("ruled certificate")
				}
			}
		}
		t.Log(circular, n, len(m.Mesh.Triangles), volume)
	}
}

func TestCertifiedWrappedConeRejectsFalsePole(t *testing.T) {
	original := certifiedWrappedFrustumFixture(t, true, 0)
	for _, name := range []string{"almost-collapsed", "off-axis"} {
		t.Run(name, func(t *testing.T) {
			s, e := original.FundamentalPatch().ToBSplineSurface()
			if e != nil {
				t.Fatal(e)
			}
			d := s.Data()
			if name == "almost-collapsed" {
				d.Poles[7].X = math.SmallestNonzeroFloat64
			} else {
				for i := 0; i < 9; i++ {
					d.Poles[2*i+1].X = .125
				}
			}
			changed, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
			if e != nil {
				t.Fatal(e)
			}
			p, e := geom.NewSurfacePatch(changed, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			w, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true})
			if e != nil {
				t.Fatal(e)
			}
			m, e := TessellateWrappedConeClosedCertified(context.Background(), w, CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}})
			if e == nil || len(m.Mesh.Vertices) != 0 {
				t.Fatal("false pole accepted", e)
			}
		})
	}
	o := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}, MaxTriangles: 1}
	m, e := TessellateWrappedConeClosedCertified(context.Background(), original, o)
	if e == nil || len(m.Mesh.Vertices) != 0 {
		t.Fatal("partial budget", e)
	}
}
