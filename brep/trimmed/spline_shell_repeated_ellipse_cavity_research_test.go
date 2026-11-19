//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestTangentEllipseCavityCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	outer, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, -2}, [3]float64{1.125, 1.125, 0}, 0)
	inner := quarticRoofSolid(t, 1.0/16, 1.0/16)
	o := SplineShellRepeatedEllipseOptions{Faces: [2]int{5, 1}}
	expect := func(s Solid, h SplineShellCombineHistory, p SplineShellRepeatedEllipse, e error) {
		t.Helper()
		topo := s.Topology()
		if e != nil || !p.Contact.Proven || p.Enclosing != 0 || p.SeparatedPairs != 35 || len(topo.Shells) != 2 ||
			topo.EulerCharacteristic != 4 || len(h.ReversedFaces) != 6 || h.Method != "contained-repeated-ellipse-cavity" {
			t.Fatal("oriented original cavity Cut", topo, h, p, e)
		}
		for i := range 6 {
			if s.faces[i].Reversed != outer.faces[i].Reversed || s.faces[6+i].Reversed == inner.faces[i].Reversed ||
				h.ReversedFaces[i] != 6+i || h.FacesA[i][0] != i || h.FacesB[i][0] != 6+i {
				t.Fatal("original faces and reversed cavity material ancestry", i, h)
			}
		}
		properties, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(properties.Volume-43773.0/20480) > 1e-6 {
			t.Fatal("original oriented signed volume", properties, e)
		}
		for _, q := range []struct {
			z       float64
			winding int
		}{
			{-.5, 0},
			{-1.5, 1},
		} {
			r, e := s.SplineRay(ctx, geom.Vec3{X: .5, Y: .5, Z: q.z}, geom.Vec3{Z: 1},
				SplineRayOptions{Line: geom.LineSplineSurfaceOptions{MaxParameter: 4}})
			if e != nil || !r.Complete {
				t.Fatal("complete exact material ray", q, r, e)
			}
			winding := 0
			for _, h := range r.Hits {
				if h.Direction == 0 {
					t.Fatal("unsigned material crossing", h)
				}
				winding += h.Direction
			}
			if winding != q.winding {
				t.Fatal("occupancy from exact spline crossing orientation", q, winding)
			}
		}
		r, e := s.SplineRay(ctx, geom.Vec3{X: .75, Y: .5, Z: -.125}, geom.Vec3{Z: 1},
			SplineRayOptions{Line: geom.LineSplineSurfaceOptions{MaxParameter: 1}})
		if e != nil || !r.Complete || len(r.Hits) != 2 || r.Hits[0].Direction+r.Hits[1].Direction != 0 {
			t.Fatal("two original opposed contacts at the cavity pinch", r, e)
		}
		for _, h := range r.Hits {
			if h.ParameterMin > .125 || h.ParameterMax < .125 || h.Face != 5 && h.Face != 7 {
				t.Fatal("contact on original outward and inward supports", h)
			}
		}
	}
	s, h, p, e := CutInternallyTangentSplineShellEllipseCavity(ctx, outer, inner, o)
	expect(s, h, p, e)
	pose := geom.Affine{{0, 0, 1, 4}, {1, 0, 0, -2}, {0, 1, 0, 3}}
	placed, e := s.TransformedContext(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	for _, q := range []struct {
		point geom.Vec3
		want  parametric.Location
	}{
		{geom.Vec3{X: .5, Y: .5, Z: -.5}, parametric.Outside},
		{geom.Vec3{X: .5, Y: .5, Z: -1.5}, parametric.Inside},
		{geom.Vec3{X: .75, Y: .5}, parametric.Boundary},
		{geom.Vec3{X: .5, Y: .5, Z: -9.0 / 4096}, parametric.Boundary},
	} {
		if result, e := s.LocateSplineMaterialPoint(ctx, q.point, SplineShellBooleanOptions{}); e != nil || result != q.want {
			t.Fatal("exact explicit spline material classification", q, result, e)
		}
		if result, e := placed.LocateSplineMaterialPoint(ctx, pose.Apply(q.point), SplineShellBooleanOptions{}); e != nil || result != q.want {
			t.Fatal("placed original spline classification in an independent projection", q, result, e)
		}
	}
	// The second placement puts the nonaffine height in every world
	// coordinate. Both cases must recover exact original UV from model XYZ.
	oblique := geom.Affine{{1, .25, .5, 2}, {.5, 1, .25, -3}, {.25, .5, 1, 4}}
	for _, m := range []geom.Affine{pose, oblique} {
		face := s.faces[7]
		face.Surface = parametric.MapSurface(face.Surface, m)
		posedFace := Solid{faces: []Face{face}}
		p0 := m.Apply(geom.Vec3{X: .5, Y: .5, Z: -9.0 / 4096})
		query := [3]*big.Rat{ratInset(p0.X), ratInset(p0.Y), ratInset(p0.Z)}
		hit, e := posedFace.LocateRationalOnSplineFace(ctx, 0, query, SplineFacePointOptions{})
		if e != nil || hit.Location != SplineFacePointInterior || hit.UV[0].Cmp(big.NewRat(1, 2)) != 0 || hit.UV[1].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal("exact inverse on the placed original curved cavity face", hit, e)
		}
		query[0].Add(query[0], big.NewRat(1, 1<<60))
		hit, e = posedFace.LocateRationalOnSplineFace(ctx, 0, query, SplineFacePointOptions{})
		if e != nil || hit.Location != SplineFacePointOutside {
			t.Fatal("original face must exclude a sub-binary model offset", hit, e)
		}
	}
	bounded := SplineShellBooleanOptions{Ray: SplineRayOptions{MaxFaces: 11}}
	if _, e := s.LocateSplineMaterialPoint(ctx, geom.Vec3{X: .5, Y: .5, Z: -1.5}, bounded); !errors.Is(e, parametric.ErrBudget) {
		t.Fatal("material classification face quota", e)
	}
	if s.faces[5].Surface != outer.faces[5].Surface || s.faces[7].Surface != inner.faces[1].Surface {
		t.Fatal("both original contact supports must be retained")
	}
	s, h, e = CombineSplineShells(ctx, outer, inner, parametric.Difference, SplineShellCombineOptions{
		ClosedEllipseTangency: &geom.GraphPointContactOptions{},
	})
	if e != nil || h.ClosedEllipse == nil {
		t.Fatal("ordinary opt-in cavity Cut dispatch", e)
	}
	expect(s, h, *h.ClosedEllipse, nil)
	var raw bytes.Buffer
	if e := WriteTangentEllipseCavityCutJSON(ctx, &raw, outer, inner, o); e != nil {
		t.Fatal(e)
	}
	s, h, p, e = ReadTangentEllipseCavityCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, SplineShellBooleanOptions{}, geom.GraphPointContactOptions{})
	expect(s, h, p, e)
	if _, _, _, e := ReadTangentEllipseComponentFuseJSON(ctx, &raw, JSONLimits{}, SplineShellBooleanOptions{}, geom.GraphPointContactOptions{}); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("incompatible exterior recipe must fail", e)
	}
	o.Faces = [2]int{1, 5}
	if s, h, p, e := CutInternallyTangentSplineShellEllipseCavity(ctx, inner, outer, o); !errors.Is(e, geom.ErrConvergence) || len(s.faces) != 0 || h.Method != "" || p.Contact.Proven {
		t.Fatal("contained stock cannot acquire an enclosing cavity", e)
	}
}
