//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestDiscoverMixedRationalWallPair(t *testing.T) {
	slowtest.Skip(t)
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
	first, err := chain.Piece(0)
	if err != nil {
		t.Fatal(err)
	}
	// Independently, at x=y=1 the stock roof is 1/64 and the
	// tool wall runs from -1/2 to 13/64. The crossing segment is
	// therefore z in [-1/2,1/64]. On the two original height
	// parameterizations its endpoints are (0,32/65) and (11/15,1).
	for i, ep := range [2]geom.TubeAlgebraicGraphEndpointCertificate{first.Endpoints[0], last.Endpoints[1]} {
		want := [2][4]*big.Rat{{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(32, 65)}, {big.NewRat(1, 1), big.NewRat(11, 15), big.NewRat(1, 1), big.NewRat(1, 1)}}[i]
		for j, root := range ep.Parameters {
			v, err := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if err != nil || v.Kind != geom.ExactRootKindRational || v.Rational.Cmp(want[j]) != 0 {
				t.Fatal("automatic section changed original endpoint ancestry", i, j, v, err)
			}
		}
	}
	bad := geom.RationalIsoTubeDiscoveryOptions{Proof: geom.RationalIsoTubeChainOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}}
	if i, f, c, p, chain, err := geom.DiscoverRationalIsoTubeChain(ctx, p, q, bad); i != 0 || f != 0 || c != nil || p != nil || chain != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("exhausted discovery cannot publish a partial recipe", i, f, c, p, chain, err)
	}
	t.Log("exact cuts and automatically discovered boxes", cuts, pieces)
	section, coverage, err := DiscoverSplineShellsMixedAlgebraicWithCoverage(ctx, a, b, o, MixedRationalIsoDiscoveryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(section.Wires()) != 2 || len(section.Branches()) != 12 || len(section.proposals) != 1 || section.proposals[0].Faces != [2]int{5, 3} || section.proposals[0].SourceOperand != 1 {
		t.Fatal("the complete original wire ledger must retain exactly one discovered wall branch", section.Wires(), section.proposals)
	}
	if len(coverage) != 36 || len(section.PairCoverage()) != len(coverage) {
		t.Fatal("missing original face-pair accounting", len(coverage))
	}
	seen := make([]bool, len(section.source.branches))
	for i, row := range coverage {
		if row.Faces != [2]int{i / 6, i % 6} || row.Disposition == ShellPairUnproved || (row.Disposition == ShellPairRestrictedIsoCoverage) != (row.Faces == [2]int{5, 3}) {
			t.Fatal("incorrect coverage attribution", i, row)
		}
		for k := row.FirstBranch; k < row.FirstBranch+row.Branches; k++ {
			if k < 0 || k >= len(seen) || seen[k] || section.source.branches[k].Faces != row.Faces {
				t.Fatal("branch is missing or has more than one owner", row, k)
			}
			seen[k] = true
		}
	}
	for k, ok := range seen {
		if !ok {
			t.Fatal("branch not in face-pair ledger", k)
		}
	}
	coverage[0].Faces = [2]int{-1, -1}
	if section.PairCoverage()[0].Faces != [2]int{0, 0} {
		t.Fatal("caller mutation changed the immutable section ledger")
	}
	for i, wire := range section.Wires() {
		bounds, err := section.EncloseWire(ctx, i, MixedSectionWireBoundsOptions{})
		if err != nil || len(bounds.Uses) != len(wire) || bounds.Pieces < len(wire) || bounds.ExactOperations <= 0 {
			t.Fatal("incomplete original wire enclosure", i, bounds, err)
		}
		for j, u := range bounds.Uses {
			b := section.source.branches[u.Use.Branch]
			if u.Use != wire[j] || u.Faces != b.Faces || !bounds.Box.Contains(u.Box.Min) || !bounds.Box.Contains(u.Box.Max) {
				t.Fatal("wire enclosure lost original face order or branch bounds", i, j, u)
			}
			for _, end := range b.Ends {
				if !b.Closed {
					p := end.Certificate.PointBounds
					if _, ok := u.Box.Intersect(p); !ok {
						t.Fatal("wire bounds omit an exact terminal enclosure", i, j, u, p)
					}
				}
			}
			if b.rational != nil {
				for _, z := range []float64{-.5, -.25, 1.0 / 64} {
					if !u.Box.Contains(geom.Vec3{X: 1, Y: 1, Z: z}) {
						t.Fatal("missed an independently known point on the wall intersection", z, u)
					}
				}
			}
		}
		if partial, err := section.EncloseWire(ctx, i, MixedSectionWireBoundsOptions{MaxPieces: 1}); !errors.Is(err, geom.ErrConvergence) || partial.Uses != nil || partial.Pieces != 0 {
			t.Fatal("piece exhaustion must publish no wire enclosure", partial, err)
		}
	}
	var saved bytes.Buffer
	if err := WriteMixedAlgebraicShellSectionJSON(&saved, section); err != nil {
		t.Fatal(err)
	}
	reload, err := ReadMixedAlgebraicShellSectionJSON(ctx, &saved, JSONLimits{}, o)
	if err != nil || len(reload.Wires()) != 2 || len(reload.proposals) != 1 || len(reload.proposals[0].Cuts) != len(cuts) || reload.proposals[0].Cuts[len(cuts)-1].Cmp(big.NewRat(11, 15)) != 0 {
		t.Fatal("generated recipes must retain the exact source domain through reload", err)
	}
	if got := reload.PairCoverage(); len(got) != 36 || got[5*6+3].Disposition != ShellPairRestrictedIsoCoverage {
		t.Fatal("reload failed to reconstruct source coverage", got)
	}
	if partial, diagnostic, err := DiscoverSplineShellsMixedAlgebraicWithCoverage(ctx, a, b, o, MixedRationalIsoDiscoveryOptions{MaxAttempts: 1}); partial != nil || !errors.Is(err, geom.ErrConvergence) || len(diagnostic) != 36 || diagnostic[5*6+3].Disposition != ShellPairUnproved {
		t.Fatal("exhausted attempt must retain an unproved pair but no section", partial, diagnostic, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if out, err := DiscoverSplineShellsMixedAlgebraic(cancelled, a, b, o, MixedRationalIsoDiscoveryOptions{}); out != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation must publish no section", out, err)
	}
}
