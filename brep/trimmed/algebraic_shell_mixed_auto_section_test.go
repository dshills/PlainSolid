package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestDiscoverMixedRationalWallPair(t *testing.T) {
	ctx := context.Background()
	a, b, _, o := mixedWallFixture(t)
	patch := func(f Face) *geom.SurfacePatch {
		p, e := exactFaceSupportPatch(f, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		d, e := faceUVBounds(f)
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.Trim(d)
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	p, q := patch(b.faces[3]), patch(a.faces[5])
	axis, clamp, cuts, pieces, chain, err := geom.DiscoverRationalIsoTubeChain(ctx, p, q, geom.RationalIsoTubeDiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if axis != 0 || clamp != 1 || cuts[0].Sign() != 0 || cuts[len(cuts)-1].Cmp(big.NewRat(11, 15)) != 0 || len(pieces) != chain.PieceCount() {
		t.Fatal(axis, clamp, cuts, pieces)
	}
	last, err := chain.Piece(chain.PieceCount() - 1)
	if err != nil || last.FreeAxis != 3 || last.Free.Upper.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(last, err)
	}
	bad := geom.RationalIsoTubeDiscoveryOptions{Proof: geom.RationalIsoTubeChainOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}}
	if i, f, c, p, chain, err := geom.DiscoverRationalIsoTubeChain(ctx, p, q, bad); i != 0 || f != 0 || c != nil || p != nil || chain != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("exhausted discovery cannot publish a partial recipe", i, f, c, p, chain, err)
	}
	t.Log("exact cuts and automatically discovered boxes", cuts, pieces)
	section, err := DiscoverSplineShellsMixedAlgebraic(ctx, a, b, o, MixedRationalIsoDiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(section.Wires()) != 2 || len(section.Branches()) != 12 || len(section.proposals) != 1 || section.proposals[0].Faces != [2]int{5, 3} || section.proposals[0].SourceOperand != 1 {
		t.Fatal("the complete original wire ledger must retain exactly one discovered wall branch", section.Wires(), section.proposals)
	}
	var saved bytes.Buffer
	if err := WriteMixedAlgebraicShellSectionJSON(&saved, section); err != nil {
		t.Fatal(err)
	}
	reload, err := ReadMixedAlgebraicShellSectionJSON(ctx, &saved, JSONLimits{}, o)
	if err != nil || len(reload.Wires()) != 2 || len(reload.proposals) != 1 || len(reload.proposals[0].Cuts) != len(cuts) || reload.proposals[0].Cuts[len(cuts)-1].Cmp(big.NewRat(11, 15)) != 0 {
		t.Fatal("generated recipes must retain the exact source domain through reload", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if out, err := DiscoverSplineShellsMixedAlgebraic(cancelled, a, b, o, MixedRationalIsoDiscoveryOptions{}); out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation must publish no section", out, err)
	}
}
