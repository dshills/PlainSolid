package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeClippedPolynomialFuse(t *testing.T) {
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

func checkClippedFuseMass(t *testing.T, r *ClippedPolynomialFuse, layout, axis, profile int) {
	t.Helper()
	// In the original unit physical parameters, B=3+uv/64 and
	// A-B=(105/128)(v-f)(v-g). The physical Jacobian is six.
	// Integrate B-base on the full footprint, then add A-B on the
	// retained A strips and the two positive overlap sectors. At fixed
	// u their dependent antiderivative is v³/3-(f+g)v²/2+fgv.
	volumes := [2][2]string{
		{"677452138343329261127/41216943789694779392", "677376236977816168007/41216943789694779392"},
		{"680001999132163925575/41216943789694779392", "679897637611624031815/41216943789694779392"},
	}
	want, _ := new(big.Rat).SetString(volumes[layout][profile])
	if r.ExactVolume().Cmp(want) != 0 {
		t.Fatal("independent original material antiderivatives", layout, axis, profile, r.ExactVolume(), want)
	}
	// Independent 32-point Gauss-Legendre per slab integrates the
	// original roof derivatives on the selected strips and sectors.
	// Add the planar base, outer wall fibers (roof-base), and interior
	// riser fibers (selected-roof minus continuing-roof).
	areas := [2][2][2]float64{
		{{40.144998232704417, 40.443802639426487}, {40.130544012507457, 40.432947134870815}},
		{{40.273027597410483, 40.637944013171207}, {40.253281989598470, 40.623031457871193}},
	}
	value, _ := want.Float64()
	mass, e := r.Boundary().MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	area := areas[layout][profile][axis]
	if e != nil || math.Abs(mass.Volume-value) > 1e-8 || math.Abs(mass.Area-area) > 1e-8 {
		t.Fatal("native boundary matches independently integrated physical volume and area", layout, axis, profile, mass.Volume, value, mass.Area, area, e)
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

func TestNativeClippedPolynomialFuseDependentFootprint(t *testing.T) {
	ctx := context.Background()
	for layout := range 2 {
		base, a, b := rectangularPolynomialStock(t, 0, 0)
		d := geom.ParameterRectangle{UMin: 2, UMax: 5, VMin: -.75, VMax: 1}
		if layout == 0 {
			a = restrictOriginalClipStock(t, a, geom.ParameterRectangle{UMin: -2, UMax: 2, VMin: 8.25, VMax: 10})
		} else {
			d.VMax = .75
		}
		b = restrictOriginalClipStock(t, b, d)
		r, e := FuseClippedPolynomialEnvelope(ctx, base, a, b, 0, 1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
		if e != nil {
			t.Fatal("original footprint can also differ on the dependent axis", layout, e)
		}
		// Integrate B-base over the full unit footprint. Add the two
		// positive A sectors over dependent range [0,7/8] for partial
		// overlap and [0,1] for containment, with physical Jacobian six.
		volumes := [2]string{"10238301406447463/628920651087872", "10376007315048295/628920651087872"}
		if r.ExactVolume().RatString() != volumes[layout] {
			t.Fatal("independent dependent-strip union volume", r.ExactVolume())
		}
		risers := 0
		for _, w := range r.Walls() {
			if w.Low == geom.ClippedPolynomialRoofBHeight && w.High == geom.ClippedPolynomialRoofAHeight {
				risers++
				if w.Points[0].Y != 1.625 || w.Points[1].Y != 1.625 || len(w.Stations[0]) != 1 || len(w.Stations[1]) != 1 {
					t.Fatal("one whole interior step on the dependent side", w)
				}
			}
		}
		if risers != 1-layout {
			t.Fatal("only an ending higher roof creates a step", layout, risers)
		}
	}
}

func TestNativeClippedPolynomialFuseRefusals(t *testing.T) {
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
