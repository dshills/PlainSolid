//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeClippedPolynomialFuse(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for layout := range 2 {
		for axis := range 2 {
			for profile := range 2 {
				src := clippedPolynomialStock(t, axis, profile, layout)
				for order := range 2 {
					r, e := FuseClippedPolynomialEnvelope(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, o)
					if e != nil {
						t.Fatal(layout, axis, profile, order, e)
					}
					checkClippedFuseIncidence(t, r, layout, profile)
					checkClippedFuseMass(t, r, layout, axis, profile)
					var raw bytes.Buffer
					if e := r.WriteJSON(ctx, &raw); e != nil {
						t.Fatal(e)
					}
					reloaded, e := ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
					if e != nil || !reflect.DeepEqual(r.Walls(), reloaded.Walls()) ||
						!reflect.DeepEqual(r.ledger, reloaded.ledger) || r.volume.Cmp(reloaded.volume) != 0 {
						t.Fatal("full stock wall and contact source reproof", layout, axis, profile, order, e)
					}
					for i, s := range r.sources {
						if !geom.SameSplineSurface(s, reloaded.sources[i]) {
							t.Fatal("source reload changed an original spline", i)
						}
					}
					var echo bytes.Buffer
					if e := reloaded.WriteJSON(ctx, &echo); e != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
						t.Fatal("canonical native Fuse source recipe", e)
					}
				}
			}
		}
	}
}

func checkClippedFuseIncidence(t *testing.T, r *ClippedPolynomialFuse, layout, profile int) {
	t.Helper()
	s := r.Boundary()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 ||
		len(s.vertices) != 28 || len(s.edges) != 46 || len(s.faces) != 20 {
		t.Fatal("one embedded union shell with no collapsed edges", top)
	}
	if s.faces[r.ledger.Base].Surface != r.sources[0] || len(r.ledger.RoofFaces) != 7 {
		t.Fatal("original base and all selected overlap and strip faces", r.ledger)
	}
	for i, id := range r.ledger.RoofFaces {
		if s.faces[id].Surface != r.sources[1+r.ledger.RoofOwners[i]] {
			t.Fatal("unaltered roof owner", i, id)
		}
	}
	for i, contact := range r.ledger.Contacts {
		want := [2][2]int{{3, 2}, {2, 3}}[profile][i]
		if contact.Order != want || len(contact.Rays) != 4 || len(contact.Sectors) != 4 {
			t.Fatal("both original typed contact links", i, contact)
		}
	}
	ctx := context.Background()
	risers := make(map[int]bool)
	var riserFaces, tapers, dividedStations int
	for _, wall := range r.Walls() {
		face := s.faces[wall.Face]
		if _, ok := face.Surface.(splineGraphAffineWall); !ok || face.Reversed || wall.EndingRoofs == 0 {
			t.Fatal("outward planar wall on an original stock boundary", wall)
		}
		isRiser := wall.Low != geom.ClippedPolynomialBaseHeight
		if isRiser {
			risers[wall.Atom] = true
			riserFaces++
			if wall.EndingRoofs != 1<<uint8(wall.High-geom.ClippedPolynomialRoofAHeight) {
				t.Fatal("riser belongs to the ending upper stock wall", wall)
			}
		}
		var z [2][2]float64
		for layer, owner := range [2]geom.ClippedPolynomialHeightOwner{wall.Low, wall.High} {
			k := 0
			if layer == 1 {
				k = len(wall.Stations[1]) + 1
			}
			u := face.Outer[k]
			p, ok := graphWallSourceTrim(u.Curve, layer == 1, true)
			if !ok || u.Edge != wall.TraceEdges[layer] || p.surface != r.sources[owner-1] || p.a != wall.OwnerUV[layer][0] || p.b != wall.OwnerUV[layer][1] {
				t.Fatal("exact lower and upper owner pcurves reach the realized wall", wall, layer)
			}
			patch, e := geom.NewSurfacePatch(p.surface, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for j, uv := range wall.OwnerUV[layer] {
				lift, e := patch.EvaluateRational(ctx, ratInset(uv.X), ratInset(uv.Y), geom.SurfacePatchEvaluationOptions{})
				if e != nil || lift.Point[0].Cmp(ratInset(wall.Points[j].X)) != 0 || lift.Point[1].Cmp(ratInset(wall.Points[j].Y)) != 0 {
					t.Fatal("wall UV endpoint independently lifts on its original surface", wall, layer, j, e)
				}
				z[layer][j], _ = lift.Point[2].Float64()
			}
		}
		for j, chain := range wall.Stations {
			last := z[0][j]
			for _, id := range chain {
				edge := s.edges[id]
				p, q := s.vertices[edge.Start], s.vertices[edge.End]
				if p.X != wall.Points[j].X || q.X != p.X || p.Y != wall.Points[j].Y || q.Y != p.Y || p.Z != last || q.Z <= p.Z {
					t.Fatal("shared exact station is ordered bottom-to-top", wall, j, p, q)
				}
				last = q.Z
			}
			if last != z[1][j] || len(chain) == 0 && !isRiser {
				t.Fatal("station chain exactly covers the wall interval", wall, j)
			}
			if len(chain) == 0 {
				tapers++
			}
			if len(chain) > 1 {
				dividedStations++
			}
		}
	}
	if len(risers) != 3-layout || riserFaces != 4 || tapers != 4 || dividedStations != 2*(1-layout) {
		t.Fatal("true interior steps, tapered faces and subdivided corner coedges", layout, len(risers), riserFaces, tapers, dividedStations)
	}
}

func TestNativeClippedPolynomialFuseRefusals(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	src := clippedPolynomialStock(t, 0, 0, 0)
	r, e := FuseClippedPolynomialEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*clippedPolynomialFuseRecipe){
		func(r *clippedPolynomialFuseRecipe) { r.Walls[0].OwnerUV[0][0].X += .125 },
		func(r *clippedPolynomialFuseRecipe) { r.Walls[0].EndingRoofs ^= 1 },
		func(r *clippedPolynomialFuseRecipe) { r.Walls[0].TraceEdges[1]++ },
		func(r *clippedPolynomialFuseRecipe) { r.Walls[0].Stations[0] = []int{0} },
		func(r *clippedPolynomialFuseRecipe) { r.Ledger.Contacts[0].Order = 1 },
		func(r *clippedPolynomialFuseRecipe) { r.Volume = "16" },
		func(r *clippedPolynomialFuseRecipe) { r.Footprints[0].UMin += .125 },
	} {
		var recipe clippedPolynomialFuseRecipe
		if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
			t.Fatal(e)
		}
		edit(&recipe)
		var changed bytes.Buffer
		if e := writePolynomialGraphRecipe(ctx, &changed, recipe); e != nil {
			t.Fatal(e)
		}
		if forged, e := ReadClippedPolynomialFuseJSON(ctx, &changed, JSONLimits{}, o); forged != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("rebuild refuses stale original wall and contact claims", e)
		}
	}
	var changedSource clippedPolynomialFuseRecipe
	if e := json.Unmarshal(raw.Bytes(), &changedSource); e != nil {
		t.Fatal(e)
	}
	d := src[2].Data()
	for i := range d.Poles {
		d.Poles[i].X += .25
	}
	translated, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	changedSource.Sources[2], e = parametric.EncodeSurface(translated)
	if e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e := writePolynomialGraphRecipe(ctx, &encoded, changedSource); e != nil {
		t.Fatal(e)
	}
	if stale, e := ReadClippedPolynomialFuseJSON(ctx, &encoded, JSONLimits{}, o); stale != nil || e == nil {
		t.Fatal("original changed support must independently reprove the boundary", e)
	}
	limit := polynomialGraphEntityCount([]Solid{r.Boundary()}) - 1
	if r, e := ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: limit}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("native topology quota", e)
	}
	var common bytes.Buffer
	c, e := CommonClippedPolynomialEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	if e := c.WriteJSON(ctx, &common); e != nil {
		t.Fatal(e)
	}
	if r, e := ReadClippedPolynomialFuseJSON(ctx, &common, JSONLimits{}, o); r != nil || e == nil {
		t.Fatal("Common retains its separate original recipe", e)
	}
	short := restrictOriginalClipStock(t, src[0], geom.ParameterRectangle{UMin: 8, UMax: 10, VMin: -1.5, VMax: 1.5})
	if r, e := FuseClippedPolynomialEnvelope(ctx, short, src[1], src[2], 0, 1e-9, o); r != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("complete original base coverage required on both exclusive strips", e)
	}
}
