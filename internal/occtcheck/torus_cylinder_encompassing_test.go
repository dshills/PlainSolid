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
	"plainsolid/geom"
	"testing"
	"time"
)

func TestOCCTEncompassingTorusCylinder(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_TORUS_CYLINDER_ENCOMPASSING")
	if oracle == "" {
		t.Skip("set pinned encompassing torus-cylinder oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "reference.jsonl")
	if log, e := exec.CommandContext(ctx, oracle, path).CombinedOutput(); e != nil {
		t.Fatalf("%v %s", e, log)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	seen := map[[2]int]bool{}
	r := big.NewRat
	for {
		var ref struct {
			Operation, Solids, Shells             int
			Lower, Upper, Volume, Area, CentroidZ float64
			Parameters                            []float64
		}
		e := decoder.Decode(&ref)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		key := [2]int{ref.Operation, int(ref.Lower)}
		if seen[key] || ref.Operation < 1 || ref.Operation > 3 {
			t.Fatal("records")
		}
		seen[key] = true
		s, e := parametric.NewEncompassingTorusCylinderSet(ctx, 5, 2, 8, ref.Lower, ref.Upper, geom.TorusCylinderOperation(ref.Operation), 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		if len(s.Solids()) != ref.Solids || ref.Solids != map[int]int{1: 1, 2: 0, 3: 1}[ref.Operation] || ref.Shells != ref.Solids {
			t.Fatal("components", ref)
		}
		measures, e := s.Measures()
		if e != nil {
			t.Fatal(e)
		}
		asFloat := func(q *big.Rat) float64 { v, _ := q.Float64(); return v }
		v := asFloat(measures.VolumePi2)*math.Pi*math.Pi + asFloat(measures.VolumePi)*math.Pi
		a := asFloat(measures.AreaPi2)*math.Pi*math.Pi + asFloat(measures.AreaPi)*math.Pi
		fz := asFloat(measures.FirstZPi) * math.Pi
		if math.Abs(ref.Volume-v) > 1e-6 || math.Abs(ref.Area-a) > 1e-6 || math.Abs(ref.CentroidZ*ref.Volume-fz) > 1e-6 {
			t.Fatal("symbolic original measures", ref, v, a, fz)
		}
		if ref.Solids > 0 {
			body, e := s.Solid()
			if e != nil {
				t.Fatal(e)
			}
			native, e := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil || math.Abs(native.Volume-ref.Volume) > 1e-6 || math.Abs(native.Area-ref.Area) > 1e-6 || math.Abs(native.Centroid.Z-ref.CentroidZ) > 1e-7 {
				t.Fatal("native", native, ref, e)
			}
		}
		line, e := s.Query().ClipRationalLine(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-9, 1), r(9, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-10})
		if e != nil || len(line.Points) != 0 || 2*len(line.Intervals) != len(ref.Parameters) {
			t.Fatal("section", e, line)
		}
		for i, piece := range line.Intervals {
			owner := geom.TorusCylinderTorus
			if ref.Operation == 3 {
				owner = geom.TorusCylinderWall
			}
			if piece.StartBoundary != owner || piece.EndBoundary != owner {
				t.Fatal("owner")
			}
			for j, root := range []*geom.ExactAlgebraicRoot{piece.Start, piece.End} {
				b := root.Interval()
				lo, _ := b.Lower.Float64()
				hi, _ := b.Upper.Float64()
				x := ref.Parameters[2*i+j]
				if x < lo-1e-7 || x > hi+1e-7 {
					t.Fatal("independent roots", x, lo, hi)
				}
			}
		}
	}
	if len(seen) != 6 {
		t.Fatal("coverage", len(seen))
	}
}
