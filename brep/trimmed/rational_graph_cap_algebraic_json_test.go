package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestAlgebraicGraphCapStockReload(t *testing.T) {
	ctx := context.Background()
	stock := makeWarpedSquareRationalGraphTube(t, 1./8.0)
	section := [3]geom.Vec3{{Z: .5}, {X: 1, Z: .5}, {Y: 1, Z: .5}}
	// The original lower edge of the inner loop is Y=1+X²/8.
	// Cutting at Y=11/8 meets it at X=sqrt(3).
	cut := [3]geom.Vec3{{Y: 11. / 8}, {Y: 11. / 8, Z: 1}, {X: 1, Y: 11. / 8}}
	o := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	q, e := stock.ClipAlgebraicGraphCap(ctx, section, cut, true, o, RationalGraphShellRegionOptions{})
	if e != nil || len(q.Section.Contacts) != 4 || len(q.Section.Material) != 2 || len(q.Wires) != 1 {
		t.Fatal("cut the sheared native through-hole", len(q.Section.Contacts), len(q.Wires), e)
	}
	irrational := -1
	for i, c := range q.Section.Contacts {
		proof, e := c.CoordinateRoots[0].Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if proof.Kind == geom.ExactRootKindIrrational {
			if irrational != -1 {
				t.Fatal("expected one irrational intersection", i, irrational)
			}
			irrational = i
		}
	}
	if irrational < 0 {
		t.Fatal("missing X=sqrt(3) from the original inner curve")
	}
	checkAlgebraicGraphCapClosure(t, q)
	canonical := q.Section.Contacts[irrational]
	// Display and caller-facing topology are replaceable; the recipe is not.
	q.Section.Contacts[irrational] = AlgebraicGraphCapContact{}
	q.Wires[0][0] = AlgebraicGraphCapUse{}
	var buf bytes.Buffer
	if e := WriteAlgebraicGraphCapJSON(&buf, q); e != nil {
		t.Fatal(e)
	}
	saved := append([]byte(nil), buf.Bytes()...)
	back, e := ReadAlgebraicGraphCapJSON(ctx, bytes.NewReader(saved), JSONLimits{}, RationalGraphShellSectionOptions{}, RationalGraphShellRegionOptions{})
	if e != nil || len(back.Wires) != 1 || len(back.Section.Contacts) != 4 {
		t.Fatal("reload must independently discover the same cap", e)
	}
	restored := back.Section.Contacts[irrational]
	comparison, e := canonical.Parameter.Compare(ctx, restored.Parameter, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !comparison.Proven || comparison.Order != 0 || canonical.Source != restored.Source {
		t.Fatal("reload changed the original exact root/owner", comparison, e)
	}
	checkAlgebraicGraphCapClosure(t, back)
	testLocate := func(c AlgebraicGraphCapClip, x, y *big.Rat, loc trimvalidate.Location) {
		t.Helper()
		got, e := c.LocateRational(ctx, [3]*big.Rat{x, y, big.NewRat(1, 2)})
		if e != nil || loc != got {
			t.Fatal("clipped cap point", x, y, got, loc, e)
		}
	}
	for _, cap := range []AlgebraicGraphCapClip{q, back} {
		testLocate(cap, big.NewRat(1, 2), big.NewRat(11, 8), trimvalidate.Boundary)
		testLocate(cap, big.NewRat(3, 2), big.NewRat(11, 8), trimvalidate.Outside)
		testLocate(cap, big.NewRat(7, 4), big.NewRat(11, 8), trimvalidate.Boundary)
		testLocate(cap, big.NewRat(1, 2), big.NewRat(3, 2), trimvalidate.Inside)
		testLocate(cap, big.NewRat(1, 2), big.NewRat(5, 4), trimvalidate.Outside)
	}
	if r, e := ReadAlgebraicGraphCapJSON(ctx, bytes.NewReader(saved), JSONLimits{MaxBytes: int64(len(saved) - 1)}, o, RationalGraphShellRegionOptions{}); !errors.Is(e, geom.ErrConvergence) || r.recipe != nil {
		t.Fatal("oversize recipe must fail atomically", e)
	}
	var bad nativeAlgebraicGraphCapClip
	if e := json.Unmarshal(saved, &bad); e != nil {
		t.Fatal(e)
	}
	bad.Cut[2][0] = nil
	damaged, e := json.Marshal(bad)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := ReadAlgebraicGraphCapJSON(ctx, bytes.NewReader(damaged), JSONLimits{}, o, RationalGraphShellRegionOptions{}); !errors.Is(e, geom.ErrInvalid) || r.recipe != nil {
		t.Fatal("a missing plane coordinate is not zero", e)
	}
	bad.Cut[2] = bad.Cut[0]
	damaged, e = json.Marshal(bad)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := ReadAlgebraicGraphCapJSON(ctx, bytes.NewReader(damaged), JSONLimits{}, o, RationalGraphShellRegionOptions{}); !errors.Is(e, geom.ErrConvergence) || r.recipe != nil {
		t.Fatal("degenerate section recipe must not publish a cap", e)
	}
}
