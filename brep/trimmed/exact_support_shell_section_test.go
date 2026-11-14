package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestAlgebraicShellSectionOnNonbinaryRoof(t *testing.T) {
	ctx := context.Background()
	a, b, hints := transverseSplineCaps(t)
	parts := make([]SewFace, 6)
	for i, face := range b.Faces() {
		s := face.Surface.(*geom.BSplineSurface)
		var e error
		if i == 1 {
			p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			data, e := p.ExportExactBezierRecipe()
			if e != nil {
				t.Fatal(e)
			}
			// Elevate the original quadratic U basis exactly. The new global
			// Bernstein control coefficients include thirds; the circle
			// intersection and original side curves remain exactly the same.
			controls := make([][4]string, 12)
			for u := 0; u < 4; u++ {
				for v := 0; v < 3; v++ {
					for k := 0; k < 4; k++ {
						q := new(big.Rat)
						if u > 0 {
							z, _ := new(big.Rat).SetString(data.Controls[(u-1)*3+v][k])
							q.Mul(z, big.NewRat(int64(u), 3))
						}
						if u < 3 {
							z, _ := new(big.Rat).SetString(data.Controls[u*3+v][k])
							q.Add(q, z.Mul(z, big.NewRat(int64(3-u), 3)))
						}
						controls[3*u+v][k] = q.RatString()
					}
				}
			}
			data.DegreeU, data.Controls = 3, controls
			p, e = geom.ImportExactBezierSurfaceRecipe(ctx, data, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			q, e := geom.NewExactBezierSurface(p)
			if e != nil {
				t.Fatal(e)
			}
			parts[i], e = ExactSplineFaceRectangle(ctx, q, geom.ParameterRectangle{}, face.Reversed, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			continue
		}
		parts[i], e = RectangularSplineFace(s, face.Reversed)
		if e != nil {
			t.Fatal(e)
		}
	}
	b, _, e := Sew(ctx, parts, SewOptions{Tolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	o := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{0, 1}, Tubes: hints}}}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	check := func(r AlgebraicShellSectionResult, e error) {
		t.Helper()
		if e != nil || !r.Complete || len(r.Wires) != 1 || len(r.Branches) != 1 || !r.Branches[0].Closed || r.Branches[0].Faces != [2]int{0, 1} {
			t.Fatalf("exact roof section: complete=%v wires=%v pairs=%v error=%v", r.Complete, r.Wires, r.Pairs, e)
		}
		if _, ok := r.Branches[0].Segments[0].TrimB().SourceSurface().(*geom.ExactBezierSurface); !ok {
			t.Fatal("section used a rounded replacement for the original roof chart")
		}
	}
	r, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	check(r, e)
	fragments, e := r.ImprintOriginalShells(ctx, AlgebraicShellImprintOptions{})
	if e != nil || fragments[0].Topology.Faces != 7 || fragments[1].Topology.Faces != 7 || fragments[1].Topology.InnerWires != 1 {
		t.Fatal("embedded exact support was lost from imprinted face fragments", fragments[0].Topology, fragments[1].Topology, e)
	}
	common, e := r.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil || common.Topology.Faces != 2 || common.Topology.EulerCharacteristic != 2 {
		t.Fatal("exact roof and plane did not assemble a closed Boolean lens", common.Topology, e)
	}
	kept := 0
	for i := range common.Faces {
		f, e := common.OriginalFace(i)
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := f.Surface.(*geom.ExactBezierSurface); ok {
			kept++
		}
	}
	if kept != 1 {
		t.Fatal("Boolean selected a surrogate for its original exact roof", kept)
	}
	coarse, e := common.BoundVolume(ctx, AlgebraicShellVolumeOptions{MaxDepth: 2, TubeSubdivisions: 4})
	if e != nil {
		t.Fatal(e)
	}
	volume, e := common.BoundVolume(ctx, AlgebraicShellVolumeOptions{MaxDepth: 6, TubeSubdivisions: 4})
	if e != nil {
		t.Fatal(e)
	}
	// Integrating r²−x²−y² over the disk gives πr⁴/2, with r=5/16.
	// Keep a rational bracket of that value separate from the flux proof.
	factor := big.NewRat(625, 131072)
	wantLo := new(big.Rat).Mul(big.NewRat(3141592653589793, 1e15), factor)
	wantHi := new(big.Rat).Mul(big.NewRat(3141592653589794, 1e15), factor)
	width := func(x geom.ExactParameterInterval) *big.Rat { return new(big.Rat).Sub(x.Upper, x.Lower) }
	if volume.Volume.Lower.Cmp(wantLo) >= 0 || volume.Volume.Upper.Cmp(wantHi) <= 0 || width(volume.Volume).Cmp(width(coarse.Volume)) >= 0 {
		t.Fatal("refinement did not tighten the enclosure of the analytic common volume", coarse.Volume, volume.Volume)
	}
	var raw bytes.Buffer
	if e = WriteAlgebraicShellSectionJSON(&raw, r); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(raw.String(), `"exact-bezier-surface"`) {
		t.Fatal("the native section lost its original exact chart data")
	}
	check(ReadAlgebraicShellSectionJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o))
	var envelope nativeAlgebraicShellSection
	if e = json.Unmarshal(raw.Bytes(), &envelope); e != nil || len(envelope.Pairs) != 1 {
		t.Fatal(e)
	}
	// Change only the saved proposal's exact chart, leaving the real shell
	// intact. Even a value with the same represented float must not bind.
	for i := range envelope.Pairs[0].Tubes {
		b := envelope.Pairs[0].Tubes[i].B.Surface
		var q map[string]json.RawMessage
		if e = json.Unmarshal(b, &q); e != nil {
			t.Fatal(e)
		}
		var recipe geom.ExactBezierSurfaceRecipe
		if e = json.Unmarshal(q["ExactBezier"], &recipe); e != nil {
			t.Fatal(e)
		}
		z, _ := new(big.Rat).SetString(recipe.Controls[4][2])
		z.Add(z, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(3), 80)))
		recipe.Controls[4][2] = z.RatString()
		q["ExactBezier"], _ = json.Marshal(recipe)
		b, _ = json.Marshal(q)
		envelope.Pairs[0].Tubes[i].B.Surface = b
	}
	bad, _ := json.Marshal(envelope)
	if _, e = ReadAlgebraicShellSectionJSON(ctx, bytes.NewReader(bad), JSONLimits{}, o); e == nil {
		t.Fatal("a modified recipe support bound to the original shell")
	}
	raw.Reset()
	if e = WriteAlgebraicShellBooleanJSON(&raw, common); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadAlgebraicShellBooleanJSON(ctx, &raw, JSONLimits{}, o, AlgebraicShellBooleanOptions{})
	if e != nil || reloaded.Topology != common.Topology {
		t.Fatal("exact roof Boolean did not reconstruct", reloaded.Topology, e)
	}
}
