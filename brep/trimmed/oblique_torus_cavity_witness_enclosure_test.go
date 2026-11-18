package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"strconv"
	"strings"
	"testing"
)

func TestNativeObliqueWitnessEnclosure(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	frame := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	body, _, e := ObliqueTorusCavity(ctx, 3, 1, 8, -6, 6, frame, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s, e := PrepareObliqueTorusCavity(ctx, body, 3, 1, 8, -6, 6, frame, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	d := geom.TorusCylinderDistanceOptions{SquaredTolerance: r(1, 1000000)}
	options := ObliqueCavityWitnessEnclosureOptions{CoordinateTolerance: r(1, 1000000000000)}
	for i, p := range [][3]*big.Rat{{r(4, 1), r(0, 1), r(-4, 1)}, {r(1, 1), r(0, 1), r(0, 1)}, {r(8, 5), r(1, 1), r(-4, 5)}} {
		world, e := pose.TransformPoint(ctx, p, false, 4096)
		if e != nil {
			t.Fatal(e)
		}
		out, e := s.BoundPlacedBoundaryDistanceEnclosed(ctx, pose, world, d, options)
		if e != nil {
			t.Fatal(e)
		}
		w := out.Witnesses[0]
		if out.EnclosureOperations <= 0 || w.GloballyNearest != (i != 2) || w.WholeCircle != (i == 1) || len(w.OriginalFaces) == 0 {
			t.Fatal("proof status", i, w)
		}
		if i == 0 {
			limited := options
			limited.MaxOperations = out.EnclosureOperations - 1
			bad, e := s.BoundPlacedBoundaryDistanceEnclosed(ctx, pose, world, d, limited)
			if e == nil || bad.Distance.Base.Lower != nil {
				t.Fatal("cumulative four-support quota")
			}
		}
		if i == 2 {
			// Independently isolate sqrt(2) by the original algebraic root engine.
			root, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{"-2", "0", "1"}, Lower: "1", Upper: "2"}, geom.ExactRootCodecOptions{})
			if e != nil {
				t.Fatal(e)
			}
			enclosed, e := root.Enclose(ctx, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 160)), geom.ExactCurvePlaneEventOptions{})
			if e != nil || !enclosed.Converged {
				t.Fatal(e)
			}
			for axis, coeff := range [][2]*big.Rat{{r(7, 10), r(-7, 25)}, {r(-2, 5), r(1, 1)}, {r(-17, 40), r(-24, 25)}} {
				lo := new(big.Rat).Add(coeff[0], new(big.Rat).Mul(coeff[1], enclosed.Interval.Lower))
				hi := new(big.Rat).Add(coeff[0], new(big.Rat).Mul(coeff[1], enclosed.Interval.Upper))
				if lo.Cmp(hi) > 0 {
					lo, hi = hi, lo
				}
				if w.Minimum[axis][0].Cmp(lo) > 0 || w.Maximum[axis][1].Cmp(hi) < 0 {
					t.Fatal("independent radical witness", axis)
				}
			}
		}
		if path := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY_DISTANCE"); path != "" {
			args := []string{}
			for _, q := range world {
				x, _ := q.Float64()
				args = append(args, strconv.FormatFloat(x, 'g', 17, 64))
			}
			args = append(args, "oblique", "witness")
			data, e := exec.Command(path, args...).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			var squared float64
			var point [3]float64
			if _, e = fmt.Fscan(strings.NewReader(string(data)), &squared, &point[0], &point[1], &point[2]); e != nil {
				t.Fatal(e, string(data))
			}
			for axis := 0; axis < 3; axis++ {
				lo, _ := w.Minimum[axis][0].Float64()
				hi, _ := w.Maximum[axis][1].Float64()
				if point[axis] < lo-1e-7 || point[axis] > hi+1e-7 {
					t.Fatal("OCCT witness", i, axis, point, lo, hi)
				}
			}
		}
	}
	world, _ := pose.TransformPoint(ctx, [3]*big.Rat{r(8, 5), r(1, 1), r(-4, 5)}, false, 4096)
	for _, o := range []ObliqueCavityWitnessEnclosureOptions{{CoordinateTolerance: r(1, 100), MaxOperations: 1}, {CoordinateTolerance: new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 200)), PrecisionBits: 16}} {
		bad, e := s.BoundPlacedBoundaryDistanceEnclosed(ctx, pose, world, d, o)
		if e == nil || bad.Distance.Base.Lower != nil {
			t.Fatal("atomic enclosure quota/precision")
		}
	}
}
