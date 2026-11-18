package offset

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestRoundedCylinderBothSharpErosion(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	for _, radii := range [][2]float64{{.5, 1}, {1, .5}, {1, 1}} {
		w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 8, o)
		if e != nil {
			t.Fatal(e)
		}
		stock, _, e := trimmed.NewWrappedCylinder(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		source, _, e := trimmed.FilletWrappedCylinderRims(ctx, stock, radii[0], radii[1], o)
		if e != nil {
			t.Fatal(e)
		}
		faces := source.Faces()
		faces = append(faces[2:], faces[:2]...)
		source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), faces, source.Tolerance())
		if e != nil {
			t.Fatal(e)
		}
		p, e := source.ValidateWrappedCylinderRimFillet(ctx, o)
		if e != nil {
			t.Fatal(e)
		}
		for _, depth := range []float64{1, 1.5, 3.5} {
			out, h, e := ErodeWrappedRoundedCylinderToSharp(ctx, source, depth, o)
			if e != nil {
				t.Fatal(radii, depth, e)
			}
			radius, height := 4-depth, 8-2*depth
			if out.Topology().Faces != 3 || out.Topology().EulerCharacteristic != 2 || h.Faces.ResultBlends != [2]int{-1, -1} {
				t.Fatal(out.Topology(), h)
			}
			if len(h.Faces.SourceFaces[h.Faces.ResultLateral]) != 1 || h.Faces.SourceFaces[h.Faces.ResultLateral][0] != p.Lateral {
				t.Fatal(h)
			}
			for end, rim := range h.Rims {
				z := depth
				if end == 1 {
					z = 8 - depth
				}
				if rim.End != end || len(rim.Edges) != 4 || rim.SourceCap != p.Caps[end] || rim.SourceBlend != p.Blends[end] || rim.SourceLateral != p.Lateral || rim.Radius.Cmp(new(big.Rat).SetFloat64(radius)) != 0 || rim.Z.Cmp(new(big.Rat).SetFloat64(z)) != 0 {
					t.Fatal(rim)
				}
				if h.Faces.SourceFaces[h.Faces.ResultCaps[end]][0] != p.Caps[end] {
					t.Fatal(h)
				}
				for _, owners := range h.Faces.SourceFaces {
					for _, id := range owners {
						if id == p.Blends[end] {
							t.Fatal("extinct face retained", id)
						}
					}
				}
			}
			mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(mass.Volume-math.Pi*radius*radius*height) > 1e-7 || math.Abs(mass.Area-2*math.Pi*radius*(radius+height)) > 1e-7 {
				t.Fatal(mass)
			}
			agreement, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !agreement.Complete {
				t.Fatal(agreement, e)
			}
			var b bytes.Buffer
			if e = trimmed.WriteJSON(&b, out); e != nil {
				t.Fatal(e)
			}
			back, e := trimmed.ReadJSONContext(ctx, &b, trimmed.JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			_, bp, e := back.ValidateCanonicalWrappedCylinder(ctx, o)
			if e != nil || bp.Radius.Cmp(new(big.Rat).SetFloat64(radius)) != 0 {
				t.Fatal(bp, e)
			}
		}
		for _, depth := range []float64{.75, 4, 5, math.Nextafter(1, 2), math.NaN()} {
			out, h, e := ErodeWrappedRoundedCylinderToSharp(ctx, source, depth, o)
			if e == nil || out.Topology().Faces != 0 || h.Rims[0].Radius != nil {
				t.Fatal("unsupported/nonrepresentable must fail atomically", depth, e)
			}
		}
	}
}

func TestRoundedCylinderSharpCollapseRejected(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	for _, height := range []float64{6, 12} {
		w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, height, o)
		if e != nil {
			t.Fatal(e)
		}
		s, _, e := trimmed.NewWrappedCylinder(ctx, w, 1e-9, o)
		if e != nil {
			t.Fatal(e)
		}
		s, _, e = trimmed.FilletWrappedCylinderRims(ctx, s, .5, 1, o)
		if e != nil {
			t.Fatal(e)
		}
		depth := 4.0
		if height == 6 {
			depth = 3
		}
		out, h, e := ErodeWrappedRoundedCylinderToSharp(ctx, s, depth, o)
		if e == nil || out.Topology().Faces != 0 || h.Rims[0].Radius != nil {
			t.Fatal("separate radius/height collapse", height, e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		out, h, e = ErodeWrappedRoundedCylinderToSharp(cancelled, s, 1, o)
		if e != context.Canceled || out.Topology().Faces != 0 || h.Rims[0].Radius != nil {
			t.Fatal("cancelled", e)
		}
	}
}
