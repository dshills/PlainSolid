package occtcheck

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestOCCTSpherePairOccurrence(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_SPHERE_OCCURRENCE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERE_OCCURRENCE to pinned independent oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "placed.jsonl")
	if b, e := exec.CommandContext(ctx, program, path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	r := big.NewRat
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	seen := map[[3]int]bool{}
	dec := json.NewDecoder(f)
	for {
		var ref struct {
			Name                string
			Sign, Bound, Solids int
			Parameters          [2]float64
			SquaredDistance     float64
			Point               [3]float64
		}
		e := dec.Decode(&ref)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		index := map[string]int{"union": 0, "common": 1, "cut": 2}
		i, ok := index[ref.Name]
		if !ok || ref.Solids != 1 || (ref.Sign != 1 && ref.Sign != -1) || (ref.Bound != 20 && ref.Bound != 9) {
			t.Fatal("record", ref)
		}
		key := [3]int{i, ref.Sign, ref.Bound}
		if seen[key] {
			t.Fatal("duplicate")
		}
		seen[key] = true
		ops := [3]parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference}
		s, e := parametric.NewSpherePairSet(ctx, parametric.Ball{Center: geom.Vec3{X: -3}, Radius: 5}, parametric.Ball{Center: geom.Vec3{X: 3}, Radius: 5}, ops[i], 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		occ, e := s.PlaceRigid(ctx, pose)
		if e != nil {
			t.Fatal(e)
		}
		queries := [3][3]*big.Rat{{r(0, 1), r(2, 1), r(1, 1)}, {r(0, 1), r(7, 1), r(1, 1)}, {r(3, 1), r(4, 1), r(1, 1)}}
		wp, _ := pose.TransformPoint(ctx, queries[i], false, 4096)
		near, e := occ.NearestBoundaryRational(ctx, wp, geom.SphereFullBoundaryDistanceOptions{})
		if e != nil || near.World.Isolated == nil || near.World.Owners != 3 || len(near.Base.TiedFaces) != 2 {
			t.Fatal("nearest proof", near, e)
		}
		lo, _ := near.World.Lower.Float64()
		hi, _ := near.World.Upper.Float64()
		tol := 3e-7 * math.Max(1, ref.SquaredDistance)
		if ref.SquaredDistance < lo-tol || ref.SquaredDistance > hi+tol {
			t.Fatal("distance", ref, lo, hi)
		}
		foot := near.World.Isolated.Candidates[2]
		if !foot.Nearest {
			t.Fatal("shared rim not nearest")
		}
		rad, _ := foot.Radicand.Float64()
		for k := 0; k < 3; k++ {
			c, _ := foot.Center[k].Float64()
			n, _ := foot.Numerator[k].Float64()
			if math.Abs(ref.Point[k]-(c+n/math.Sqrt(rad))) > 1e-6 {
				t.Fatal("world isolated foot", ref)
			}
		}
		origin, _ := pose.TransformPoint(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, false, 4096)
		direction, _ := pose.TransformVector(ctx, [3]*big.Rat{r(int64(2*ref.Sign), 3), r(0, 1), r(0, 1)}, false, 4096)
		line, e := occ.ClipRationalLine(ctx, origin, direction, r(int64(-ref.Bound), 1), r(int64(ref.Bound), 1), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
		if e != nil || len(line.Intervals) != 1 || len(line.Points) != 0 {
			t.Fatal("line", line, e)
		}
		expected := [3][2]float64{{-12, 12}, {-3, 3}, {-12, -3}}
		ends := expected[i]
		masks := [3][2]geom.BoreBoundaryMask{{1, 2}, {2, 1}, {1, 2}}
		owners := masks[i]
		if ref.Sign < 0 {
			ends = [2]float64{-ends[1], -ends[0]}
			owners = [2]geom.BoreBoundaryMask{owners[1], owners[0]}
		}
		if ends[0] < float64(-ref.Bound) {
			ends[0] = float64(-ref.Bound)
			owners[0] = 0
		}
		if ends[1] > float64(ref.Bound) {
			ends[1] = float64(ref.Bound)
			owners[1] = 0
		}
		interval := line.Intervals[0]
		if interval.StartBoundary != owners[0] || interval.EndBoundary != owners[1] {
			t.Fatal("native owner masks", interval, owners)
		}
		for j, root := range []*geom.ExactAlgebraicRoot{interval.Start, interval.End} {
			b := root.Interval()
			a, _ := b.Lower.Float64()
			z, _ := b.Upper.Float64()
			if math.Abs(ref.Parameters[j]-ends[j]) > 1e-7 || ref.Parameters[j] < a-1e-7 || ref.Parameters[j] > z+1e-7 {
				t.Fatal("raw finite parameter", ref, j, a, z)
			}
		}
	}
	if len(seen) != 12 {
		t.Fatal("incomplete", len(seen))
	}
}
