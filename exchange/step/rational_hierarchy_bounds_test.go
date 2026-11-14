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
	"testing"
)

func TestRationalHierarchyBounds(t *testing.T) {
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy.step"
		if vendor {
			name = "occt-rational-spherical-hierarchy.step"
		}
		t.Run(name, func(t *testing.T) {
			raw, _ := os.ReadFile("testdata/" + name)
			ctx := context.Background()
			m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
			if e != nil {
				t.Fatal(e)
			}
			query := func(bits int) (RationalHierarchyBounds, error) {
				if vendor {
					return BoundOCCTRationalSphericalHierarchy(ctx, OCCTRationalSphericalHierarchy(m), RationalHierarchyBoundsOptions{FractionBits: bits})
				}
				return BoundRationalSphericalHierarchy(ctx, m, RationalHierarchyBoundsOptions{FractionBits: bits})
			}
			coarse, e := query(8)
			if e != nil {
				t.Fatal(e)
			}
			fine, e := query(64)
			if e != nil {
				t.Fatal(e)
			}
			if len(fine.Leaves) != 7 || fine.SquareRoots != 12 || fine.PointEvaluations != 14 || fine.LengthUnit != .001 {
				t.Fatal("work", fine.PointEvaluations, fine.SquareRoots)
			}
			min, max, pad := fine.Aggregate.Min(), fine.Aggregate.Max(), fine.Aggregate.MaximumAxisPadding()
			cm, cx := coarse.Aggregate.Min(), coarse.Aggregate.Max()
			for i := 0; i < 3; i++ {
				if min[i].Cmp(cm[i]) < 0 || max[i].Cmp(cx[i]) > 0 {
					t.Fatal("precision nesting")
				}
				if pad[i].Cmp(new(big.Rat).SetFrac(big.NewInt(3), new(big.Int).Lsh(big.NewInt(1), 64))) > 0 {
					t.Fatal("padding")
				}
			}
			if !vendor {
				wantMin := [3]*big.Rat{big.NewRat(-23, 1), big.NewRat(-20, 1), big.NewRat(-5, 1)}
				wantMax := [3]*big.Rat{big.NewRat(162, 5), big.NewRat(30, 1), big.NewRat(15, 1)}
				for i := 0; i < 3; i++ {
					if min[i].Cmp(wantMin[i]) != 0 || max[i].Cmp(wantMax[i]) != 0 || pad[i].Sign() != 0 {
						t.Fatal("exact bounds")
					}
				}
			}
			before := new(big.Rat).Set(min[0])
			min[0].SetInt64(999)
			if fine.Aggregate.Min()[0].Cmp(before) != 0 {
				t.Fatal("mutable bound")
			}
			min = fine.Aggregate.Min()
			// Independently verify the circular support enclosure by squaring, without
			// using the square-root routine or any floating point acceptance.
			for k, leaf := range fine.Leaves {
				u := m.Instances[k]
				p := m.Products[u.ProductID]
				if leaf.ProductID != u.ProductID || leaf.SourceBodyID != p.SourceBodyID || len(leaf.SourceFaceIDs) == 0 {
					t.Fatal("ancestry")
				}
				if leaf.Family == "planar" || leaf.Family == "sphere" {
					continue
				}
				local := geom.Vec3{X: 1, Y: 2, Z: 3}
				r := big.NewRat(3, 1)
				offset := big.NewRat(1, 1)
				if leaf.Family == "cylinder" {
					local = geom.Vec3{Z: 1.5}
					r = big.NewRat(2, 1)
				}
				center, e := u.ApplyExact(local)
				if e != nil {
					t.Fatal(e)
				}
				a := u.Matrix()
				lo, hi, slack := leaf.Bounds.Min(), leaf.Bounds.Max(), leaf.Bounds.MaximumAxisPadding()
				for i := 0; i < 3; i++ {
					if leaf.Family == "cylinder" {
						offset = new(big.Rat).Mul(big.NewRat(3, 2), new(big.Rat).Abs(a[i][2]))
					}
					extent := new(big.Rat).Sub(hi[i], center[i])
					if new(big.Rat).Sub(center[i], lo[i]).Cmp(extent) != 0 {
						t.Fatal("symmetry")
					}
					upper := new(big.Rat).Quo(new(big.Rat).Sub(extent, offset), r)
					lower := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Sub(extent, slack[i]), offset), r)
					q := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Mul(a[i][2], a[i][2]))
					if lower.Sign() < 0 || new(big.Rat).Mul(lower, lower).Cmp(q) > 0 || new(big.Rat).Mul(upper, upper).Cmp(q) < 0 {
						t.Fatal("continuous support enclosure")
					}
				}
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); oracle != "" {
				path := filepath.Join(t.TempDir(), "bounds.step")
				os.WriteFile(path, raw, 0600)
				data, e := exec.Command(oracle, "read", path).CombinedOutput()
				if e != nil {
					t.Fatal(e, string(data))
				}
				at := bytes.LastIndex(data, []byte("{"))
				var q struct{ Bounds [6]float64 }
				if at < 0 || json.Unmarshal(data[at:], &q) != nil {
					t.Fatal(string(data))
				}
				for i := 0; i < 3; i++ {
					a, _ := min[i].Float64()
					b, _ := max[i].Float64()
					if math.Abs(q.Bounds[i]-a) > 2e-7 || math.Abs(q.Bounds[i+3]-b) > 2e-7 {
						t.Fatal("OCCT", q.Bounds, min, max)
					}
				}
			}
		})
	}
}
func TestHierarchySqrtEnclosure(t *testing.T) {
	for _, q := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(9, 25), big.NewRat(2, 3), big.NewRat(399, 625)} {
		lo, hi, e := hierarchySqrtEnclosure(q, 80, 2048)
		if e != nil {
			t.Fatal(e)
		}
		if new(big.Rat).Mul(lo, lo).Cmp(q) > 0 || new(big.Rat).Mul(hi, hi).Cmp(q) < 0 {
			t.Fatal("not enclosing")
		}
		if new(big.Rat).Sub(hi, lo).Cmp(new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))) > 0 {
			t.Fatal("width")
		}
	}
	if _, _, e := hierarchySqrtEnclosure(big.NewRat(-1, 1), 64, 2048); e == nil {
		t.Fatal("negative")
	}
	if _, _, e := hierarchySqrtEnclosure(big.NewRat(2, 3), 128, 128); e == nil {
		t.Fatal("bits")
	}
}
func TestRationalHierarchyBoundsReject(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/occt-rational-spherical-hierarchy.step")
	m, e := ReadOCCTRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []RationalHierarchyBoundsOptions{{FractionBits: 129}, {FractionBits: -1}, {MaxPointEvaluations: 1}, {MaxSquareRoots: 1}, {MaxCoefficientBits: 255}, {Source: Limits{MaxEntities: 16}}} {
		if q, e := BoundOCCTRationalSphericalHierarchy(ctx, m, o); e == nil || q.Leaves != nil {
			t.Fatal("budget partial")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := BoundOCCTRationalSphericalHierarchy(cancelled, m, RationalHierarchyBoundsOptions{}); e == nil || q.Leaves != nil {
		t.Fatal("cancel")
	}
	m.Instances[0].Path[0]++
	if q, e := BoundOCCTRationalSphericalHierarchy(ctx, m, RationalHierarchyBoundsOptions{}); e == nil || q.Leaves != nil {
		t.Fatal("stale")
	}
}
func FuzzRationalHierarchyBounds(f *testing.F) {
	raw, _ := os.ReadFile("testdata/occt-rational-spherical-hierarchy.step")
	f.Add(raw, uint8(64))
	f.Add([]byte("invalid"), uint8(1))
	f.Fuzz(func(t *testing.T, b []byte, bits uint8) {
		if len(b) > 131072 {
			return
		}
		ctx := context.Background()
		l := Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}
		m, e := ReadOCCTRationalSphericalHierarchy(ctx, bytes.NewReader(b), l, 1e-9)
		if e != nil {
			return
		}
		q, e := BoundOCCTRationalSphericalHierarchy(ctx, m, RationalHierarchyBoundsOptions{FractionBits: int(bits%128) + 1, MaxPointEvaluations: 1024, Source: l})
		if e != nil && q.Leaves != nil {
			t.Fatal("partial")
		}
	})
}
