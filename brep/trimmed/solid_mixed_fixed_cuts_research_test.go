//go:build research

package trimmed

import (
	"context"
	"math/big"
	"testing"
)

func TestMixedExactHolePrescribedDirection(t *testing.T) {
	outer, hole := mixedAffineLoop(t, big.NewRat(3, 8), false), mixedAffineLoop(t, big.NewRat(1, 8), true)
	cuts := [][][]*big.Rat{make([][]*big.Rat, 4), make([][]*big.Rat, 4)}
	cuts[1][0] = []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 1)}
	o := MixedExactAffineMeshOptions{}
	o.Display.Deflection = .01
	r, e := TessellateMixedExactAffineRegionWithCutsCertified(context.Background(), nativeObliquePatch(t), outer, [][]PCurve{hole}, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, s := range r.Samples {
		if s.Loop != 1 || s.Trim != 0 {
			continue
		}
		for j, q := range s.Parameters {
			if q.Cmp(big.NewRat(1, 3)) == 0 {
				found = true
				uv, e := mixedExactEvaluate(hole[0], q)
				if e != nil {
					t.Fatal(e)
				}
				for k := range uv {
					if uv[k].Cmp(r.ExactUV[s.Vertices[j]][k]) != 0 {
						t.Fatal("original hole direction")
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("nonbinary hole cut omitted")
	}
}
