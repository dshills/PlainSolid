package trimmed

import (
	"context"
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

func TestNativeObliqueCavityLine(t *testing.T) {
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
	o := NativeObliqueCavityLineOptions{Line: geom.ToroidLineOptions{ParameterTolerance: 1e-9}}
	worldFrame := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}
	pose, e := placement.NewRationalRigid(worldFrame, 4096)
	if e != nil {
		t.Fatal(e)
	}
	for _, y := range []*big.Rat{r(0, 1), r(1, 1)} {
		origin := [3]*big.Rat{r(1, 1), y, r(0, 1)}
		direction := [3]*big.Rat{r(3, 5), r(0, 1), r(-4, 5)}
		if y.Sign() != 0 {
			direction = [3]*big.Rat{r(6, 5), r(0, 1), r(-8, 5)}
		}
		line, e := s.IntersectRationalLineBoundary(ctx, origin, direction, r(-8, 1), r(8, 1), o)
		if e != nil {
			t.Fatal(e)
		}
		if len(line.Boundary.Hits) != 6 || len(line.Boundary.Intervals) != 0 {
			t.Fatal("complete section", line)
		}
		for i := 1; i < 5; i++ {
			want := 4
			if y.Sign() != 0 {
				want = 2
			}
			if len(line.HitFaces[i]) != want {
				t.Fatal("algebraic seam owners", i, line.HitFaces)
			}
		}
		a, _ := pose.TransformPoint(ctx, origin, false, 4096)
		d, _ := pose.TransformVector(ctx, direction, false, 4096)
		world, e := s.IntersectPlacedRationalLineBoundary(ctx, pose, a, d, r(-8, 1), r(8, 1), o)
		if e != nil {
			t.Fatal(e)
		}
		for i, hit := range line.Boundary.Hits {
			order, e := hit.Parameter.Compare(ctx, world.Boundary.Hits[i].Parameter, geom.ExactCurvePlaneEventOptions{})
			if e != nil || !order.Proven || order.Order != 0 {
				t.Fatal("world t", e)
			}
			if len(world.HitFaces[i]) != len(line.HitFaces[i]) {
				t.Fatal("world owners")
			}
			for j, id := range line.HitFaces[i] {
				if world.HitFaces[i][j] != id {
					t.Fatal("changed original owner")
				}
			}
		}
		partition, e := s.PartitionPlacedRationalLine(ctx, pose, a, d, r(-8, 1), r(8, 1), o)
		if e != nil || len(partition.Boundary.Hits) != len(world.Boundary.Hits) {
			t.Fatal("partition boundary", e)
		}
		for i, hit := range partition.Boundary.Hits {
			cmp, e := hit.Parameter.Compare(ctx, world.Boundary.Hits[i].Parameter, geom.ExactCurvePlaneEventOptions{})
			if e != nil || !cmp.Proven || cmp.Order != 0 || hit.Boundary != world.Boundary.Hits[i].Boundary {
				t.Fatal("partition original event", e)
			}
		}
		// Feed this one-pass partition's boundary into the live reference below.
		world.Boundary = partition.Boundary
		if oracle := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY_LINE"); oracle != "" {
			args := []string{}
			for _, vector := range [][3]*big.Rat{a, d} {
				for _, v := range vector {
					f, _ := v.Float64()
					args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
				}
			}
			args = append(args, "-8", "8", "oblique")
			data, e := exec.Command(oracle, args...).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			fields := strings.Fields(string(data))
			if len(fields) != 6 {
				t.Fatal("OCCT count", string(data))
			}
			for i, f := range fields {
				v, e := strconv.ParseFloat(f, 64)
				lo, _ := world.Boundary.Hits[i].Bounds.Lower.Float64()
				hi, _ := world.Boundary.Hits[i].Bounds.Upper.Float64()
				if e != nil || v < lo-1e-7 || v > hi+1e-7 {
					t.Fatal("OCCT parameter", i, v, lo, hi, e)
				}
			}
		}
	}
	tangent, e := s.IntersectRationalLineBoundary(ctx, [3]*big.Rat{r(17, 5), r(0, 1), r(-16, 5)}, [3]*big.Rat{r(0, 1), r(1, 1), r(0, 1)}, r(-1, 1), r(1, 1), o)
	if e != nil || len(tangent.Boundary.Hits) != 1 || tangent.Boundary.Hits[0].Multiplicity != 2 || len(tangent.HitFaces[0]) != 4 {
		t.Fatal("non-toggling tangent", tangent, e)
	}
	contact, e := s.IntersectRationalLineBoundary(ctx, [3]*big.Rat{r(8, 1), r(0, 1), r(6, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(1, 1)}, r(-1, 1), r(1, 1), o)
	if e != nil || len(contact.HitFaces) != 1 || len(contact.HitFaces[0]) != 2 {
		t.Fatal("simultaneous owners", contact, e)
	}
	generator, e := s.IntersectRationalLineBoundary(ctx, [3]*big.Rat{r(8, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}, r(-7, 1), r(7, 1), o)
	if e != nil || len(generator.IntervalFaces) != 1 || len(generator.StartFaces[0]) != 2 || len(generator.EndFaces[0]) != 2 {
		t.Fatal("boundary interval", generator, e)
	}
	clipped, e := s.IntersectRationalLineBoundary(ctx, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(3, 5), r(0, 1), r(-4, 5)}, r(2, 1), r(3, 1), o)
	if e != nil || len(clipped.Boundary.Hits) != 1 {
		t.Fatal("finite endpoint clipping", clipped, e)
	}
	at, e := clipped.Boundary.Hits[0].Parameter.SignPolynomial(ctx, []*big.Rat{r(-2, 1), r(1, 1)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || at.Sign != 0 {
		t.Fatal("exact finite endpoint", e)
	}
	o.Ownership.MaxOperations = 129
	bad, e := s.IntersectRationalLineBoundary(ctx, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(3, 5), r(0, 1), r(-4, 5)}, r(-8, 1), r(8, 1), o)
	if e == nil || len(bad.Boundary.Hits) != 0 {
		t.Fatal("ownership quota")
	}
}
