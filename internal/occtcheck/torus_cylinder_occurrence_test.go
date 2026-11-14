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
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestOCCTTorusCylinderOccurrence(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_TORUS_OCCURRENCE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_TORUS_OCCURRENCE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "world.jsonl")
	if b, e := exec.CommandContext(ctx, program, path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	seen := map[int]bool{}
	r := big.NewRat
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	for {
		var ref struct {
			Operation     int
			Normal, Point [3]float64
			Parameters    []float64
		}
		e := dec.Decode(&ref)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if ref.Operation < 0 || ref.Operation > 2 || seen[ref.Operation] {
			t.Fatal("record")
		}
		seen[ref.Operation] = true
		op := []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion}[ref.Operation]
		source, _ := geom.NewTorusCylinderMaterial(5, 2, 6, -3, 4, op)
		occ, e := placement.PlaceTorusCylinder(ctx, source, pose)
		if e != nil {
			t.Fatal(e)
		}
		x := int64(7)
		if ref.Operation == 0 {
			x = 3
		}
		p, _ := pose.TransformPoint(ctx, [3]*big.Rat{r(x, 1), r(0, 1), r(0, 1)}, false, 4096)
		loc, mask, e := occ.ClassifyRational(ctx, p, geom.SurfacePatchOptions{})
		if e != nil || loc != geom.BoreBoundary || mask != 1 {
			t.Fatal(loc, mask, e)
		}
		n, e := occ.BoundaryNormalsRational(ctx, p, geom.SurfacePatchOptions{})
		if e != nil || n.Owners != 1 {
			t.Fatal(n, e)
		}
		var nf [3]float64
		norm := 0.0
		for i := range nf {
			nf[i], _ = n.Directions[0][i].Float64()
			norm += nf[i] * nf[i]
		}
		norm = math.Sqrt(norm)
		for i := range nf {
			pf, _ := p[i].Float64()
			if math.Abs(pf-ref.Point[i]) > 1e-7 || math.Abs(nf[i]/norm-ref.Normal[i]) > 1e-7 {
				t.Fatal("actual retained face point/normal", ref, nf)
			}
		}
		origin, _ := pose.TransformPoint(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, false, 4096)
		direction, _ := pose.TransformVector(ctx, [3]*big.Rat{r(1, 1), r(0, 1), r(1, 10)}, false, 4096)
		line, e := occ.ClipRationalLine(ctx, origin, direction, r(-8, 1), r(8, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
		if e != nil || len(line.Intervals)*2 != len(ref.Parameters) {
			t.Fatal(line, e)
		}
		for i, v := range line.Intervals {
			for j, root := range []*geom.ExactAlgebraicRoot{v.Start, v.End} {
				b := root.Interval()
				lo, _ := b.Lower.Float64()
				hi, _ := b.Upper.Float64()
				if ref.Parameters[2*i+j] < lo-1e-7 || ref.Parameters[2*i+j] > hi+1e-7 {
					t.Fatal("world line t", ref, lo, hi)
				}
			}
		}
	}
	if len(seen) != 3 {
		t.Fatal("records", len(seen))
	}
}
