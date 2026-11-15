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

func TestNativeClippedPolynomialCut(t *testing.T) {
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	for layout := range 4 {
		for axis := range 2 {
			for profile := range 2 {
				src := clippedPolynomialStock(t, axis, profile, layout%2)
				if layout >= 2 {
					src = twoAxisClippedFuseStock(t, axis, profile, layout-2)
				}
				r, e := CutClippedPolynomialComponents(ctx, src[0], src[2], src[1], axis, 1e-9, o)
				if e != nil {
					t.Fatal(layout, axis, profile, e)
				}
				checkNativeClippedCutVolumes(t, r, layout, axis, profile)
				checkNativeClippedCutShells(t, r, layout, profile)
				var raw bytes.Buffer
				if e := r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadClippedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if e != nil || !reflect.DeepEqual(r.shells, loaded.shells) || !reflect.DeepEqual(r.Contacts(), loaded.Contacts()) {
					t.Fatal("original chamber, wall and contact reproof on reload", layout, axis, profile, e)
				}
				for i, source := range r.native.sources {
					if !geom.SameSplineSurface(source, loaded.native.sources[i]) {
						t.Fatal("reload preserves original unmodified support", i)
					}
				}
				var canonical bytes.Buffer
				if e := loaded.WriteJSON(ctx, &canonical); e != nil || !bytes.Equal(raw.Bytes(), canonical.Bytes()) {
					t.Fatal("canonical unequal-footprint source recipe", e)
				}
				if q, e := CutClippedPolynomialComponents(ctx, src[0], src[1], src[2], axis, 1e-9, o); q != nil ||
					!errors.Is(e, ErrClippedPolynomialCutSelfContact) || !errors.Is(e, geom.ErrConvergence) {
					t.Fatal("disconnected local links must refuse within the merged material component", layout, axis, profile, e)
				}
			}
		}
	}
}

func checkNativeClippedCutVolumes(t *testing.T, r *ClippedPolynomialCut, layout, axis, profile int) {
	t.Helper()
	q := func(s string) *big.Rat {
		n, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatal(s)
		}
		return n
	}
	// The original B-minus-A between-branch integral is
	// (105/128)*|f-g|³/6 across a free station slab, times the physical
	// Jacobian six. Independently add the exclusive stock integral of
	// B-base on the original B footprint minus the overlap rectangle.
	pureEnds := [2]string{"337636263/82433887579389558784", "54292832487/82433887579389558784"}
	outside := [4][2]string{
		{"15693/8192", "15861/8192"},
		{"0", "0"},
		{"1636419/524288", "1636419/524288"},
		{"1690221/524288", "1709037/524288"},
	}[layout][axis]
	expected := [3]*big.Rat{
		q(pureEnds[profile]), q("3/19193135104"), new(big.Rat).Add(q(outside), q(pureEnds[1-profile])),
	}
	got, total := r.ExactComponentVolumes()
	if len(got) != 3 {
		t.Fatal("three proved positive components", got)
	}
	sum := new(big.Rat)
	for i, volume := range got {
		if volume.Sign() <= 0 || volume.Cmp(expected[i]) != 0 {
			t.Fatal("independent original region and exclusive stock integrals", layout, axis, profile, i, volume, expected[i])
		}
		sum.Add(sum, expected[i])
	}
	if total.Cmp(sum) != 0 {
		t.Fatal("exact total of all retained material", total, sum)
	}
	if layout != 1 {
		want, _ := expected[2].Float64()
		m, e := r.Components()[2].MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || math.Abs(m.Volume-want) > 1e-8 {
			t.Fatal("native strip-bearing boundary agrees with independent volume", m.Volume, want, e)
		}
	}
}

func checkNativeClippedCutShells(t *testing.T, r *ClippedPolynomialCut, layout, profile int) {
	t.Helper()
	components := r.Components()
	for i, solid := range components {
		top := solid.Topology()
		if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 {
			t.Fatal("each component is one embedded positive shell", i, top)
		}
		chambers := r.Chambers(i)
		want := 1
		if i == 2 {
			want = [4]int{2, 1, 3, 3}[layout]
		}
		if len(chambers) != want || chambers[0] != (ClippedPolynomialCutRegion{Tile: 0, Region: i + 1}) {
			t.Fatal("original overlap region and exclusive strip membership", i, chambers)
		}
		for j, ch := range chambers {
			floor := r.native.sources[2]
			if ch.Region == -1 {
				floor = r.native.sources[0]
			}
			if solid.faces[2*j].Surface != r.native.sources[1] || solid.faces[2*j+1].Surface != floor {
				t.Fatal("unmodified original roof and lower support in each chamber", i, j, ch)
			}
		}
		for _, w := range r.Walls(i) {
			for layer, owner := range [2]geom.ClippedPolynomialHeightOwner{w.Low, w.High} {
				use := solid.faces[w.Face].Outer[0]
				if layer == 1 {
					use = solid.faces[w.Face].Outer[1+len(w.Stations[1])]
				}
				p, ok := graphWallSourceTrim(use.Curve, layer == 1, true)
				if !ok || use.Edge != w.TraceEdges[layer] || p.surface != r.native.sources[owner-1] ||
					p.a != w.OwnerUV[layer][0] || p.b != w.OwnerUV[layer][1] {
					t.Fatal("exact original upper and lower wall coedges", i, layer, w)
				}
			}
		}
	}
	for i, c := range r.Contacts() {
		order := [2][2]int{{3, 2}, {2, 3}}[profile][i]
		if c.Order != order || c.Components != ([2]int{i, i + 1}) {
			t.Fatal("two typed contacts between separately embedded components", i, c)
		}
		for j, id := range c.Components {
			p := components[id].vertices[c.Vertices[j]]
			if c.Point[0].Cmp(ratInset(p.X)) != 0 || c.Point[1].Cmp(ratInset(p.Y)) != 0 || c.Point[2].Cmp(ratInset(p.Z)) != 0 {
				t.Fatal("one exact contact vertex in each independent native namespace", i, j, c)
			}
		}
	}
}

func TestNativeClippedPolynomialCutRecipeRefusals(t *testing.T) {
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	src := clippedPolynomialStock(t, 0, 0, 0)
	r, e := CutClippedPolynomialComponents(ctx, src[0], src[2], src[1], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*clippedPolynomialCutRecipe){
		func(r *clippedPolynomialCutRecipe) { r.Contacts[0].Components[1] = r.Contacts[0].Components[0] },
		func(r *clippedPolynomialCutRecipe) { r.Contacts[1].Vertices[1]++ },
		func(r *clippedPolynomialCutRecipe) { r.Orders[0] = 1 },
		func(r *clippedPolynomialCutRecipe) { r.Shells[2].Chambers[1].Tile = 0 },
		func(r *clippedPolynomialCutRecipe) { r.Shells[1].Volume = "0" },
		func(r *clippedPolynomialCutRecipe) { r.Shells[2].Walls[0].OwnerUV[0][0].Y += .125 },
		func(r *clippedPolynomialCutRecipe) { r.Total = "0" },
		func(r *clippedPolynomialCutRecipe) { r.Footprints[0].UMax -= .125 },
	} {
		var recipe clippedPolynomialCutRecipe
		if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
			t.Fatal(e)
		}
		edit(&recipe)
		var forged bytes.Buffer
		if e := writePolynomialGraphRecipe(ctx, &forged, recipe); e != nil {
			t.Fatal(e)
		}
		if result, e := ReadClippedPolynomialCutJSON(ctx, &forged, JSONLimits{}, o); result != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("complete source proof refuses a stale Cut recipe", e)
		}
	}
	var changed clippedPolynomialCutRecipe
	if e := json.Unmarshal(raw.Bytes(), &changed); e != nil {
		t.Fatal(e)
	}
	d := src[2].Data()
	for i := range d.Poles {
		d.Poles[i].X += .125
	}
	translated, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	changed.Sources[1], e = parametric.EncodeSurface(translated)
	if e != nil {
		t.Fatal(e)
	}
	var forged bytes.Buffer
	if e := writePolynomialGraphRecipe(ctx, &forged, changed); e != nil {
		t.Fatal(e)
	}
	if out, e := ReadClippedPolynomialCutJSON(ctx, &forged, JSONLimits{}, o); out != nil || e == nil {
		t.Fatal("shifted original stock requires a new positive component proof", e)
	}
	// A changed base can still support a valid Cut with identical
	// contacts. Its exclusive strip volume must be re-proved.
	if e := json.Unmarshal(raw.Bytes(), &changed); e != nil {
		t.Fatal(e)
	}
	d = src[0].Data()
	for i := range d.Poles {
		d.Poles[i].Z += .125
	}
	raised, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	fresh, e := CutClippedPolynomialComponents(ctx, raised, src[2], src[1], 0, 1e-9, o)
	if e != nil {
		t.Fatal("valid altered original stock", e)
	}
	_, freshTotal := fresh.ExactComponentVolumes()
	_, oldTotal := r.ExactComponentVolumes()
	if new(big.Rat).Sub(oldTotal, freshTotal).Cmp(big.NewRat(3, 32)) != 0 {
		t.Fatal("raising the base by 1/8 changes the area-3/4 exclusive strip")
	}
	changed.Sources[0], e = parametric.EncodeSurface(raised)
	if e != nil {
		t.Fatal(e)
	}
	forged.Reset()
	if e := writePolynomialGraphRecipe(ctx, &forged, changed); e != nil {
		t.Fatal(e)
	}
	if out, e := ReadClippedPolynomialCutJSON(ctx, &forged, JSONLimits{}, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("a valid changed base invalidates old exact volume and wall claims", e)
	}
	limit := polynomialGraphEntityCount(r.Components()) - 1
	if out, e := ReadClippedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: limit}, o); out != nil ||
		!errors.Is(e, geom.ErrConvergence) {
		t.Fatal("all native component entities count against the recipe quota", e)
	}
}

func TestNativeClippedPolynomialCutDependentFootprint(t *testing.T) {
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	for axis := range 2 {
		base, a, b := rectangularPolynomialStock(t, axis, 0)
		aDomain := [2]geom.ParameterRectangle{
			{UMin: -2, UMax: 2, VMin: 8.25, VMax: 10},
			{UMin: -2, UMax: 1.5, VMin: 8, VMax: 10},
		}[axis]
		a = restrictOriginalClipStock(t, a, aDomain)
		b = restrictOriginalClipStock(t, b, geom.ParameterRectangle{UMin: 2, UMax: 5, VMin: -.75, VMax: 1})
		volumes := [2][]string{
			{[2]string{"2669354272804551/1257841302175744", "2695149846384327/1257841302175744"}[axis], "314625206304775/1257841302175744"},
			{"14729223/1257841302175744", "3/19193135104", "482528967/1257841302175744", [2]string{"15861/8192", "15693/8192"}[axis]},
		}
		for order, roofs := range [2][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			out, e := CutClippedPolynomialComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, o)
			if e != nil {
				t.Fatal("each difference can retain separate links on dependent-only offsets", axis, order, e)
			}
			v, _ := out.ExactComponentVolumes()
			if len(v) != len(volumes[order]) {
				t.Fatal("one attached or isolated strip plus the exact overlap geometry", axis, order, v)
			}
			// Independent B-base or A-base rectangular integrals and the
			// factor antiderivative on the full free interval [0,1].
			for i, q := range v {
				if q.RatString() != volumes[order][i] {
					t.Fatal("independent physical chamber sum", axis, order, i, q, volumes[order][i])
				}
			}
			for i, c := range out.Contacts() {
				pair := [2]int{0, 1}
				if order == 1 {
					pair = [2]int{i, i + 1}
				}
				if c.Components != pair {
					t.Fatal("contact remains on original overlap shells, away from the free stock strip", axis, order, i, c)
				}
			}
			if order == 0 {
				if len(out.Chambers(0)) != 2 || len(out.Chambers(1)) != 1 {
					t.Fatal("the under-branch sector alone attaches to the lower dependent strip")
				}
			} else if group := out.Chambers(3); !reflect.DeepEqual(group, []ClippedPolynomialCutRegion{{Tile: 1, Region: -1}}) {
				t.Fatal("exclusive upper strip has no adjacent positive overlap region", group)
			}
			var raw bytes.Buffer
			if e := out.WriteJSON(ctx, &raw); e != nil {
				t.Fatal(e)
			}
			reloaded, e := ReadClippedPolynomialCutJSON(ctx, &raw, JSONLimits{}, o)
			if e != nil || !reflect.DeepEqual(out.shells, reloaded.shells) {
				t.Fatal("dependent-only source and component replay", axis, order, e)
			}
		}
	}
}
