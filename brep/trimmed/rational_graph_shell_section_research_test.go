//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRationalGraphSplineShellAffineExactTerminals(t *testing.T) {
	ctx := context.Background()
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	// z=(1+x)/3 crosses four stock edges at points that cannot be
	// represented in binary64. A section does not need to alter the solid.
	q := [3]geom.Vec3{{X: -1}, {X: 2, Z: 1}, {X: -1, Y: 1}}
	r, e := s.SectionRationalGraphSplineShell(ctx, q, RationalGraphShellSectionOptions{VerifyModelEdges: true, Regions: &RationalGraphShellRegionOptions{}})
	if e != nil || !r.Complete || len(r.Wires) != 1 || len(r.Wires[0]) != 4 {
		t.Fatal("exact rational box section", r, e)
	}
	if len(r.Regions.Material) != 1 || len(r.Regions.Material[0].Holes) != 0 || r.Regions.Rings[0].Orientation != 1 {
		t.Fatal("exact nonbinary section does not prove the projected material", r.Regions)
	}
	seen := map[[2]int64]bool{}
	for _, b := range r.Branches {
		p, ok := b.Pieces[0].(ExactQuadraticUV)
		if !ok {
			t.Fatal("missing exact chart line", b)
		}
		exact, display := p.Endpoints()
		for i, end := range b.Ends {
			x, y, z := end.Point[0], end.Point[1], end.Point[2]
			if new(big.Rat).Mul(z, big.NewRat(3, 1)).Cmp(new(big.Rat).Add(x, big.NewRat(1, 1))) != 0 {
				t.Fatal("rounded exact endpoint", end)
			}
			seen[[2]int64{x.Num().Int64(), y.Num().Int64()}] = true
			u, v := ratInset(display[i].X), ratInset(display[i].Y)
			if exact[i][0].Cmp(u) == 0 && exact[i][1].Cmp(v) == 0 {
				t.Fatal("expected one chart coordinate to retain nonbinary data", exact[i], display[i])
			}
		}
	}
	if len(seen) != 4 {
		t.Fatal("missing corner columns", seen)
	}
	capDomain := geom.ParameterRectangle{UMax: 1, VMax: 1}
	knots := []float64{0, 0, 1, 1}
	support, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{
		{X: -1}, {X: -1, Y: 1}, {X: 2, Z: 1}, {X: 2, Y: 1, Z: 1},
	}, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	opt := RationalGraphShellSplitOptions{Caps: RationalGraphSectionCapOptions{RationalEndpoints: true}}
	if rejected, e := s.SplitRationalGraphSplineShell(ctx, q, support, capDomain, RationalGraphShellSplitOptions{}); e == nil || len(rejected.Halves[0].Solid.faces) != 0 {
		t.Fatal("strict binary endpoint path silently rounded", e)
	}
	split, e := s.SplitRationalGraphSplineShell(ctx, q, support, capDomain, opt)
	if e != nil {
		t.Fatal("nonbinary direct split", e)
	}
	for i, h := range split.Halves {
		var native bytes.Buffer
		if e = WriteJSON(&native, h.Solid); e != nil {
			t.Fatal(e)
		}
		reload, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal("nonbinary split native reload", e)
		}
		vertices, e := reload.RationalVertices(ctx)
		if e != nil {
			t.Fatal(e)
		}
		saw := 0
		for _, v := range vertices {
			if v.Point[2].Denom().Cmp(big.NewInt(3)) == 0 {
				saw++
			}
		}
		if saw != 4 {
			t.Fatal("nonbinary vertices lost", i, saw)
		}
		mp, e := reload.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(mp.Volume-.5) > 1e-8 {
			t.Fatal("nonbinary oblique half volume", i, mp.Volume, e)
		}
		display, e := reload.TessellateEmbeddedDisplay(ctx, EmbeddedDisplayOptions{Tessellation: parametric.TessellationOptions{Deflection: .05}})
		if e != nil || display.SignedVolume.Sign() <= 0 {
			t.Fatal("nonbinary split displays do not close", i, e)
		}
	}
	r2 := q
	for i := range r2 {
		r2[i].Z += .125
	}
	support2, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{
		{X: -1, Z: .125}, {X: -1, Y: 1, Z: .125}, {X: 2, Z: 1.125}, {X: 2, Y: 1, Z: 1.125},
	}, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	band, e := s.ClipParallelRationalGraphSplineShell(ctx, [2][3]geom.Vec3{q, r2},
		[2]geom.Surface{support, support2}, [2]geom.ParameterRectangle{capDomain, capDomain},
		RationalGraphParallelBandOptions{Caps: RationalGraphSectionCapOptions{RationalEndpoints: true}})
	if e != nil || len(band.CapEdges[0]) != 4 || len(band.CapEdges[1]) != 4 {
		t.Fatal("oblique rational band", e)
	}
	v, e := band.Solid.RationalVertices(ctx)
	if e != nil || len(v) != 8 {
		t.Fatal("rational band vertices", len(v), e)
	}
	for _, p := range v {
		h := new(big.Rat).Sub(p.Point[2], new(big.Rat).Quo(new(big.Rat).Add(p.Point[0], big.NewRat(1, 1)), big.NewRat(3, 1)))
		if h.Sign() != 0 && h.Cmp(big.NewRat(1, 8)) != 0 {
			t.Fatal("rounded rational band vertex", p.Point)
		}
	}
	mass, e := band.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(mass.Volume-.125) > 1e-8 {
		t.Fatal("parallel rational band volume", mass.Volume, e)
	}
	limited := RationalGraphShellSectionOptions{VerifyModelEdges: true, EdgeContacts: geom.ComposedPlaneContactsOptions{EventOptions: geom.ExactCurvePlaneEventOptions{MaxOperations: 1}}}
	if bad, e := s.SectionRationalGraphSplineShell(ctx, q, limited); e == nil || bad.Complete || len(bad.Branches) != 0 {
		t.Fatal("exhausted independent model-edge verification returned a wire", bad, e)
	}
	for _, c := range []struct {
		h    float64
		pass bool
	}{{2, true}, {1, false}} {
		p := [3]geom.Vec3{{Z: c.h}, {X: 1, Z: c.h}, {Y: 1, Z: c.h}}
		r, e := s.SectionRationalGraphSplineShell(ctx, p, RationalGraphShellSectionOptions{})
		if len(r.Branches) != 0 || len(r.Wires) != 0 || r.Complete != c.pass || (e == nil) != c.pass {
			t.Fatal("disjoint or unsupported coincident plane", c, r, e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := s.SectionRationalGraphSplineShell(canceled, q, RationalGraphShellSectionOptions{}); !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal("canceled section published a result", r, e)
	}
}
