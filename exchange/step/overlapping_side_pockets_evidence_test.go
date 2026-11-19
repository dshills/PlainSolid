package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"strings"
	"testing"
	"time"
)

func TestOverlappingPocketSourceGraph(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"side", "interior"} {
		b, e := os.ReadFile("testdata/overlapping-" + mode + "-pockets.step")
		if e != nil {
			t.Fatal(e)
		}
		f, e := Parse(bytes.NewReader(b), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		counts := map[string]int{}
		for _, ent := range f.Entities {
			for _, r := range ent.Records {
				counts[r.Type]++
			}
		}
		edges, vertices, bounds := 21, 14, 9
		if mode == "interior" {
			edges, vertices, bounds = 18, 12, 10
		}
		for kind, want := range map[string]int{"ADVANCED_FACE": 9, "CYLINDRICAL_SURFACE": 2, "EDGE_CURVE": edges, "VERTEX_POINT": vertices, "FACE_BOUND": bounds, "MANIFOLD_SOLID_BREP": 1} {
			if counts[kind] != want {
				t.Fatal(mode, kind, counts[kind], want)
			}
		}
		if _, e = ReadAnalyticCircularPocketExtrusions(bytes.NewReader(b), Limits{}); e == nil {
			t.Fatal("overlapping footprint silently admitted", mode)
		}
		if _, e = ReadCircularHoleExtrusions(bytes.NewReader(b), Limits{}); e == nil {
			t.Fatal("exact canonical source route broadened", mode)
		}
	}
}
func TestOverlappingSidePocketPCurveDiscrepancy(t *testing.T) {
	t.Parallel()
	b, e := os.ReadFile("testdata/overlapping-side-pockets.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, want := range map[int64]geom.Vec3{359: {X: 3.5, Y: 4, Z: 3}, 388: {X: 6.5, Y: 4, Z: 3}, 429: {X: 5, Y: 6, Z: 3}} {
		p, e := splitTriple(f, id, "CARTESIAN_POINT")
		if e != nil || p != want {
			t.Fatal("source binding", id, p, e)
		}
	}
	wrapper, e := simple(f, 427, "SURFACE_CURVE", 4)
	if e != nil || wrapper.Args[1].Ref != 428 || wrapper.Args[3].Text != "PCURVE_S1" {
		t.Fatal("source wrapper binding", e)
	}
	for _, id := range []int64{357, 386} {
		r, e := simple(f, id, "CYLINDRICAL_SURFACE", 3)
		if e != nil {
			t.Fatal(e)
		}
		radius, e := cylinderSupportNumber(r.Args[2])
		if e != nil || radius != 2.5 {
			t.Fatal("source radius", e)
		}
	}
	for i, id := range []int64{435, 441} {
		r, e := simple(f, id, "CARTESIAN_POINT", 2)
		if e != nil {
			t.Fatal(e)
		}
		u, e := liftSourceRat(r.Args[1].Items[0])
		if e != nil {
			t.Fatal(e)
		}
		next, e := simple(f, id+1, "CARTESIAN_POINT", 2)
		if e != nil {
			t.Fatal(e)
		}
		other, e := liftSourceRat(next.Args[1].Items[0])
		if e != nil || other.Cmp(u) != 0 {
			t.Fatal("nonconstant source U", e)
		}
		_, cosine, errBound, e := liftTrigBernstein([]*big.Rat{u, u})
		if e != nil {
			t.Fatal(e)
		}
		target := big.NewRat(3, 5)
		if i == 1 {
			target.Neg(target)
		}
		delta := new(big.Rat).Abs(new(big.Rat).Sub(cosine[0], target))
		if delta.Cmp(errBound) <= 0 {
			t.Fatal("did not prove nonzero support discrepancy")
		}
		upper := new(big.Rat).Add(delta, errBound)
		if upper.Cmp(big.NewRat(1, 10000000000)) >= 0 {
			t.Fatal("unexpected source discrepancy size")
		}
	}
}

// An independent exact two-arc model profile, without angular parameters.
func TestOverlappingSidePocketExactRationalArcs(t *testing.T) {
	t.Parallel()
	for side := 0; side < 2; side++ {
		poles := [][2]int64{{1, 4}, {1, 9}, {5, 6}}
		weights := []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 2), big.NewRat(5, 4)}
		center := big.NewRat(7, 2)
		if side == 1 {
			poles = [][2]int64{{5, 6}, {9, 9}, {9, 4}}
			weights = []*big.Rat{big.NewRat(5, 4), big.NewRat(1, 2), big.NewRat(1, 1)}
			center = big.NewRat(13, 2)
		}
		x, y := make([]*big.Rat, 3), make([]*big.Rat, 3)
		for i, p := range poles {
			x[i] = new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(p[0], 1), center), weights[i])
			y[i] = new(big.Rat).Mul(big.NewRat(p[1]-4, 1), weights[i])
			if weights[i].Sign() <= 0 {
				t.Fatal("weight")
			}
		}
		xx, yy, ww := liftProduct(x, x), liftProduct(y, y), liftProduct(weights, weights)
		for i := range xx {
			res := new(big.Rat).Sub(new(big.Rat).Add(xx[i], yy[i]), new(big.Rat).Mul(big.NewRat(25, 4), ww[i]))
			if res.Sign() != 0 {
				t.Fatal("circle identity", side, i, res)
			}
		}
		dx, dy := make([]*big.Rat, 2), make([]*big.Rat, 2)
		for i := 0; i < 2; i++ {
			dx[i] = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(x[i+1], x[i]))
			dy[i] = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(y[i+1], y[i]))
		}
		a, b := liftProduct(x, dy), liftProduct(y, dx)
		for i := range a {
			if new(big.Rat).Sub(a[i], b[i]).Sign() >= 0 {
				t.Fatal("not strictly clockwise", side, i)
			}
		}
	}
}
func TestOverlappingSidePocketOCCTMeasures(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_OVERLAPPING_SIDE_POCKETS")
	if bin == "" {
		t.Skip("optional independent Boolean oracle")
	}
	for _, mode := range []string{"side", "interior"} {
		path := filepath.Join(t.TempDir(), mode+".step")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		raw, e := exec.CommandContext(ctx, bin, mode, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(raw))
		}
		start := strings.LastIndex(string(raw), "\n{")
		if start < 0 {
			t.Fatal(string(raw))
		}
		var v struct {
			Faces, Planes, Cylinders int
			Volume, Area             float64
			Center                   [3]float64
		}
		if e = json.Unmarshal(raw[start+1:], &v); e != nil {
			t.Fatal(e)
		}
		theta := math.Acos(3.0 / 5)
		a := 12.5*(math.Pi-theta) + 6
		p := 10 * (math.Pi - theta)
		volume := 480 - 3*a
		area := 376 + 3*p
		center := [3]float64{5, 4, (1440 - 13.5*a) / volume}
		if mode == "side" {
			b := a / 2
			volume = 360 - 3*b
			area = 288 + 1.5*p
			center = [3]float64{5, (2464 - 12*b) / volume, (1080 - 13.5*b) / volume}
		}
		if v.Faces != 9 || v.Planes != 7 || v.Cylinders != 2 || math.Abs(v.Volume-volume) > 1e-9 || math.Abs(v.Area-area) > 1e-9 {
			t.Fatal(mode, v, volume, area)
		}
		for i, x := range center {
			if math.Abs(v.Center[i]-x) > 1e-10 {
				t.Fatal(mode, v.Center, center)
			}
		}
	}
}
