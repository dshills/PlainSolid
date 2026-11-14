package geom

import (
	"context"
	"math"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestObliqueCavityBoundaryDistance(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	s, e := NewObliqueCylinderTorusCavityMaterial(ctx, 3, 1, 8, -6, 6, pose, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		p     [3]*big.Rat
		want  float64
		owner BoreBoundaryMask
	}{{[3]*big.Rat{r(4, 1), r(0, 1), r(-4, 1)}, 1, TorusCylinderTorus}, {[3]*big.Rat{r(22, 5), r(0, 1), r(-6, 5)}, 1, TorusCylinderTorus}, {[3]*big.Rat{r(8, 5), r(1, 1), r(-4, 5)}, math.Pow(2-math.Sqrt(2), 2), TorusCylinderTorus}, {[3]*big.Rat{r(9, 1), r(0, 1), r(0, 1)}, 1, TorusCylinderWall}, {[3]*big.Rat{r(0, 1), r(0, 1), r(7, 1)}, 1, TorusCylinderUpperCap}, {[3]*big.Rat{r(9, 1), r(0, 1), r(7, 1)}, 2, TorusCylinderWall | TorusCylinderUpperCap}} {
		out, e := s.BoundBoundaryDistance(ctx, tc.p, TorusCylinderDistanceOptions{SquaredTolerance: r(1, 1000000)})
		if e != nil {
			t.Fatal(tc, e)
		}
		lo, _ := out.Lower.Float64()
		hi, _ := out.Upper.Float64()
		if tc.want < lo-1e-12 || tc.want > hi+1e-12 || out.PossibleOwners != tc.owner {
			t.Fatal(tc, lo, hi, out.PossibleOwners)
		}
		// Every returned torus witness lies on the original meridian circle,
		// including algebraic-height witnesses which must not be rounded.
		w := out.Supports[0].Witness
		rho, rhoHi, err := rationalSqrtBounds(w.RadialSquared, 64, &polynomialBudget{maxOperations: 10000, maxBits: 4096})
		if err != nil || rho.Cmp(rhoHi) != 0 || w.HeightBase.Sign() != 0 {
			t.Fatal("torus witness radius", err)
		}
		dr := new(big.Rat).Sub(rho, r(3, 1))
		residual := new(big.Rat).Add(new(big.Rat).Mul(dr, dr), w.HeightRadicand)
		if residual.Cmp(r(1, 1)) != 0 {
			t.Fatal("original torus witness support")
		}
		// Independently reconstruct the exact inverse relation for the witness chart.
		for i := 0; i < 3; i++ {
			q := new(big.Rat).Set(out.TorusFrame[i][3])
			for j := 0; j < 3; j++ {
				q.Add(q, new(big.Rat).Mul(out.TorusFrame[i][j], out.TorusQuery[j]))
			}
			if q.Cmp(tc.p[i]) != 0 {
				t.Fatal("world inverse")
			}
		}
		if tc.owner == (TorusCylinderWall|TorusCylinderUpperCap) && out.ExactOwners != tc.owner {
			t.Fatal("exact tied seam")
		}
		if path := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY_DISTANCE"); path != "" {
			args := []string{}
			for _, x := range tc.p {
				f, _ := x.Float64()
				args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
			}
			data, e := exec.Command(path, args...).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			d, e := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
			if e != nil || d < lo-1e-7 || d > hi+1e-7 {
				t.Fatal("OCCT", d, lo, hi, e)
			}
		}
	}
	limited, e := s.BoundBoundaryDistance(ctx, [3]*big.Rat{r(1, 1), r(1, 1), r(1, 1)}, TorusCylinderDistanceOptions{SquaredTolerance: r(1, 100), Arithmetic: SurfacePatchOptions{MaxOperations: 1}})
	if e == nil || limited.Lower != nil {
		t.Fatal("atomic budget")
	}
}
