package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestAlgebraicGraphSeparatedStockBand(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	q := big.NewRat
	stock := reparameterizedGraphSleeveForFlux(t)
	planes := [2][3]geom.Vec3{
		{{Z: .25}, {Y: 1, Z: .25}, {X: 1, Z: .25}},
		{{Z: .75}, {X: 1, Z: .75}, {Y: 1, Z: .75}},
	}
	options := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	regions := RationalGraphShellRegionOptions{}
	if p, e := stock.ClipAlgebraicGraphCapPair(ctx, planes, options, regions); !errors.Is(e, geom.ErrConvergence) || p.caps[0].recipe != nil {
		t.Fatal("legacy crossing contract must require a common seam", e)
	}
	r, e := stock.ClipAlgebraicGraphBand(ctx, planes, options, regions)
	if e != nil || len(r.Seams) != 0 || len(r.Caps[0].Wires) != 2 || len(r.Caps[1].Wires) != 2 {
		t.Fatal("two original holed caps, no manufactured shared seam", len(r.Seams), e)
	}
	roots := geom.ExactCurvePlaneEventOptions{MaxOperations: 20000000, MaxCoefficientBits: 32768}
	b, e := r.ExactIndexedBoundary(ctx, options, roots)
	if e != nil || b.EulerCharacteristic != 0 || b.BoundaryComponents != 1 {
		t.Fatal("the exact separated band retains the through bore", b, e)
	}
	// The original square sleeve area is eight; the quadratic shear and
	// projective chart parameters preserve area. Constant cap separation
	// 1/2 therefore gives volume 4 over the unchanged through bore.
	v, e := r.BoundVerticalVolume(ctx, q(1, 1<<14), options, roots)
	if e != nil || v.Lower.Cmp(q(4, 1)) > 0 || v.Upper.Cmp(q(4, 1)) < 0 {
		t.Fatal("exact weighted parallel band must contain volume four", v, e)
	}
	var capBytes bytes.Buffer
	if e := WriteAlgebraicGraphCapJSON(&capBytes, r.Caps[0]); e != nil {
		t.Fatal(e)
	}
	single, e := ReadAlgebraicGraphCapJSON(ctx, &capBytes, JSONLimits{}, options, regions)
	if e != nil || len(single.Wires) != 2 || !single.separated {
		t.Fatal("native single separated cap", e)
	}
	var save bytes.Buffer
	if e := WriteAlgebraicGraphStockJSON(&save, r); e != nil {
		t.Fatal(e)
	}
	p, topology, e := ReadAlgebraicGraphStockJSON(ctx, bytes.NewReader(save.Bytes()), JSONLimits{}, options, regions, roots)
	if e != nil || topology.EulerCharacteristic != 0 || len(p.Seams) != 0 {
		t.Fatal("original band history must survive reproof", topology, e)
	}
	var doc nativeAlgebraicGraphStock
	if e := json.Unmarshal(save.Bytes(), &doc); e != nil {
		t.Fatal(e)
	}
	doc.Separated = false
	raw, e := json.Marshal(doc)
	if e != nil {
		t.Fatal(e)
	}
	if p, b, e := ReadAlgebraicGraphStockJSON(ctx, bytes.NewReader(raw), JSONLimits{}, options, regions, roots); e == nil || p.caps[0].recipe != nil || len(b.Faces) != 0 {
		t.Fatal("a false crossing-mode claim cannot publish a band", e)
	}
	f, e := p.PrepareVerticalMaterial(ctx, options, roots)
	if e != nil {
		t.Fatal(e)
	}
	line, e := f.IntersectRationalLine(ctx, [3]*big.Rat{q(1, 2), q(7, 4), q(0, 1)}, [3]*big.Rat{q(0, 1), q(0, 1), q(1, 1)}, [2]*big.Rat{q(0, 1), q(1, 1)}, roots)
	if e != nil || len(line.Contacts) != 2 || len(line.Material) != 1 || line.Contacts[0].Cap != 0 || line.Contacts[1].Cap != 1 {
		t.Fatal("parallel caps remain separate exact material events", line, e)
	}
	for i, hit := range line.Contacts {
		a, e := hit.Parameter.Classify(ctx, roots)
		if e != nil || a.Kind != geom.ExactRootKindRational || a.Rational.Cmp(q(int64(2*i+1), 4)) != 0 {
			t.Fatal("independent parallel-plane hit", i, a, e)
		}
	}
	display := AlgebraicGraphStockFillOptions{Deflection: .15}
	display.Boundary.Section = options
	display.Boundary.Exact = roots
	display.Boundary.Display.Deflection = .004
	mesh, e := p.TessellateAffineProjectedStockCertified(ctx, display)
	if e != nil || mesh.SignedVolume == nil || mesh.Boundary.Indexed.EulerCharacteristic != 0 ||
		mesh.MaximumFillError > display.Deflection {
		t.Fatal("the separated weighted band must retain a closed certified holed mesh", mesh.MaximumFillError, e)
	}
	// Nonparallel planes may also be rigorously separated over the
	// finite source. The upper plane z=5/8+x/16 meets the lower at
	// x=-6, outside the stock; integrating the affine gap yields 15/4.
	planes[1] = [3]geom.Vec3{{Z: 5. / 8}, {X: 1, Z: 11. / 16}, {Y: 1, Z: 5. / 8}}
	oblique, e := stock.ClipAlgebraicGraphBand(ctx, planes, options, regions)
	if e != nil || len(oblique.Seams) != 0 {
		t.Fatal("separated nonparallel cap pair", e)
	}
	v, e = oblique.BoundVerticalVolume(ctx, q(1, 1<<14), options, roots)
	if e != nil || v.Lower.Cmp(q(15, 4)) > 0 || v.Upper.Cmp(q(15, 4)) < 0 {
		t.Fatal("oblique affine stock gap", v, e)
	}

}
