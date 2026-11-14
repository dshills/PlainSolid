package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExactRestrictedWallChain(t *testing.T) {
	ctx := context.Background()
	a, _ := crossingFootprintRoofShell(t, false)
	b, _ := crossingFootprintRoofShell(t, true)
	source, e := exactFaceSupportPatch(b.faces[3], geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	target, e := exactFaceSupportPatch(a.faces[5], geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pieces := []geom.RationalIsoTubePiece{
		{Box: geom.SurfacePairParameterBox{A: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: 0, VMax: .5}, B: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: .375, VMax: .9375}}, FreeAxis: 1},
		{Box: geom.SurfacePairParameterBox{A: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: .25, VMax: .875}, B: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: .75, VMax: 1}}, FreeAxis: 3},
	}
	cuts := []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(11, 15)}
	chain, e := geom.CertifyRationalIsoTubeChain(ctx, source, target, 0, 1, cuts, pieces, geom.RationalIsoTubeChainOptions{})
	if e != nil || chain.PieceCount() != 2 {
		t.Fatal("chain", e)
	}
	ownerA, ownerB, axis, fixed, err := chain.OriginalSupports()
	if err != nil || ownerA != source || ownerB != target || axis != 0 || fixed.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("lost original support binding", err)
	}
	fixed.SetInt64(0)
	_, _, _, fixedAgain, _ := chain.OriginalSupports()
	if fixedAgain.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("mutable source binding")
	}
	first, _ := chain.Piece(0)
	second, _ := chain.Piece(1)
	if first.FreeAxis != 1 || second.FreeAxis != 3 || second.Free.Lower.Cmp(big.NewRat(109, 130)) != 0 || second.Free.Upper.Cmp(big.NewRat(1, 1)) != 0 || first.Source.Upper.Cmp(second.Source.Lower) != 0 {
		t.Fatal("exact restriction lost", first, second)
	}
	expected := [][4]*big.Rat{
		{big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1), big.NewRat(32, 65)},
		{big.NewRat(1, 1), big.NewRat(1, 2), big.NewRat(1, 1), big.NewRat(109, 130)},
		{big.NewRat(1, 1), big.NewRat(11, 15), big.NewRat(1, 1), big.NewRat(1, 1)},
	}
	for i, event := range []geom.TubeAlgebraicGraphPointCertificate{first.Endpoints[0], first.Endpoints[1], second.Endpoints[1]} {
		for j, root := range event.Parameters {
			q, err := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if err != nil || q.Rational == nil || q.Rational.Cmp(expected[i][j]) != 0 {
				t.Fatal("original endpoint", i, j, err)
			}
		}
		aa, err := source.EvaluateRational(ctx, expected[i][0], expected[i][1], geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		bb, err := target.EvaluateRational(ctx, expected[i][2], expected[i][3], geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		wantZ := []*big.Rat{big.NewRat(-1, 2), big.NewRat(-19, 128), big.NewRat(1, 64)}[i]
		for j := 0; j < 3; j++ {
			if aa.Point[j].Cmp(bb.Point[j]) != 0 {
				t.Fatal("original supports differ")
			}
		}
		if aa.Point[2].Cmp(wantZ) != 0 {
			t.Fatal("independent height", aa.Point)
		}
	}
	same, _, err := first.Endpoints[1].ModelEvent.CompareModelPoint(ctx, second.Endpoints[0].ModelEvent, geom.ExactCurvePlaneEventOptions{})
	if err != nil || !same {
		t.Fatal("unmatched chart junction", err)
	}
	point, err := chain.EvaluatePiece(ctx, 1, big.NewRat(9, 10), geom.TubeAlgebraicGraphEndpointOptions{})
	if err != nil || !point.Proven {
		t.Fatal(err)
	}
	sourceV, err := point.Parameters[1].Classify(ctx, geom.ExactCurvePlaneEventOptions{})
	if err != nil || sourceV.Rational.Cmp(big.NewRat(53, 90)) != 0 {
		t.Fatal("retained evaluation", err)
	}
	if _, err := chain.EvaluatePiece(ctx, 1, big.NewRat(4, 5), geom.TubeAlgebraicGraphEndpointOptions{}); !errors.Is(err, geom.ErrDomain) {
		t.Fatal("raw overlap admitted as retained material", err)
	}
	bounds, err := chain.EnclosePiece(ctx, 1, second.Free.Lower, second.Free.Upper, geom.SurfaceTubeIntervalOptions{MaxIterations: 2})
	if err != nil && !errors.Is(err, geom.ErrConvergence) || !bounds.HasBounds || bounds.Bounds.Min.Z > float64(-19)/128 || bounds.Bounds.Max.Z < float64(1)/64 {
		t.Fatal("restricted enclosure", bounds, err)
	}
	large := new(big.Int).Lsh(big.NewInt(1), 100)
	expensive := new(big.Rat).SetFrac(large, new(big.Int).Add(new(big.Int).Set(large), big.NewInt(1)))
	badPoint, err := chain.EvaluatePiece(ctx, 1, expensive, geom.TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxCoefficientBits: 64}}})
	if !errors.Is(err, geom.ErrConvergence) || badPoint.Proven {
		t.Fatal("point bit limit", err)
	}
	badBounds, err := chain.EnclosePiece(ctx, 1, expensive, big.NewRat(1, 1), geom.SurfaceTubeIntervalOptions{MaxCoefficientBits: 64})
	if !errors.Is(err, geom.ErrConvergence) || badBounds.HasBounds {
		t.Fatal("interval bit limit", err)
	}
	// Owned domains and proposals cannot mutate the accepted source occurrence.
	cuts[1].SetInt64(0)
	second.Free.Lower.SetInt64(0)
	pieces[1].Box.B.VMin = 0
	again, _ := chain.Piece(1)
	if again.Source.Lower.Cmp(big.NewRat(1, 2)) != 0 || again.Free.Lower.Cmp(big.NewRat(109, 130)) != 0 {
		t.Fatal("mutable restriction")
	}
	if _, err := json.Marshal(chain); !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("restriction erased by legacy persistence", err)
	}
	cuts = []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(11, 15)}
	pieces[1].Box.B.VMin = .75
	for _, which := range []int{0, 1, 2} {
		badCuts := append([]*big.Rat(nil), cuts...)
		badPieces := append([]geom.RationalIsoTubePiece(nil), pieces...)
		switch which {
		case 0:
			badCuts = badCuts[1:]
			badPieces = badPieces[1:]
		case 1:
			badPieces[1].Box.B.VMin = math.Nextafter(float64(109)/130, 1)
		case 2:
			badPieces[1].Box.A.UMin = math.Nextafter(1, 2)
		}
		bad, err := geom.CertifyRationalIsoTubeChain(ctx, source, target, 0, 1, badCuts, badPieces, geom.RationalIsoTubeChainOptions{})
		if err == nil || bad != nil {
			t.Fatal("invalid retained cover", which, err)
		}
	}
	bad, err := geom.CertifyRationalIsoTubeChain(ctx, source, target, 0, 1, cuts, pieces, geom.RationalIsoTubeChainOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}})
	if err == nil || bad != nil {
		t.Fatal("atomic budget", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, err := geom.CertifyRationalIsoTubeChain(cancelled, source, target, 0, 1, cuts, pieces, geom.RationalIsoTubeChainOptions{}); !errors.Is(err, context.Canceled) || bad != nil {
		t.Fatal("cancel", err)
	}
	// Reversing the target chart retains source order and exact model geometry.
	surface, ok := a.faces[5].Surface.(*geom.BSplineSurface)
	if !ok {
		t.Fatal("fixture support")
	}
	reversed, err := surface.ReversedV()
	if err != nil {
		t.Fatal(err)
	}
	reversePatch, err := geom.NewSurfacePatch(reversed, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	reversePieces := append([]geom.RationalIsoTubePiece(nil), pieces...)
	for i := range reversePieces {
		d := &reversePieces[i].Box.B
		d.VMin, d.VMax = 1-d.VMax, 1-d.VMin
	}
	reverseChain, err := geom.CertifyRationalIsoTubeChain(ctx, source, reversePatch, 0, 1, cuts, reversePieces, geom.RationalIsoTubeChainOptions{})
	if err != nil {
		t.Fatal("reversed chart", err)
	}
	reverseDomain, _ := reverseChain.Piece(1)
	if reverseDomain.FreeIncreasing || reverseDomain.Free.Lower.Sign() != 0 || reverseDomain.Free.Upper.Cmp(big.NewRat(21, 130)) != 0 {
		t.Fatal("reversed exact domain", reverseDomain)
	}
	reversedPoint, err := reverseChain.EvaluatePiece(ctx, 1, big.NewRat(1, 10), geom.TubeAlgebraicGraphEndpointOptions{})
	if err != nil {
		t.Fatal(err)
	}
	same, _, err = point.ModelEvent.CompareModelPoint(ctx, reversedPoint.ModelEvent, geom.ExactCurvePlaneEventOptions{})
	if err != nil || !same {
		t.Fatal("reversal changed geometry", err)
	}
	t.Logf("complete original wall pair:2 pieces, retained junction B_V=1/2 A_V=109/130, operations=%d", chain.ExactOperations())
}
