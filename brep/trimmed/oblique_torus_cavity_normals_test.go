package trimmed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestNativeObliqueCavityNormals(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	frame := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	body, q, e := ObliqueTorusCavity(ctx, 3, 1, 8, -6, 6, frame, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s, e := PrepareObliqueTorusCavity(ctx, body, 3, 1, 8, -6, 6, frame, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	patch, e := q.TorusQuadrant(ctx, 0, 0, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	jet, e := patch.ExactPatch().EvaluateRational(ctx, r(1, 3), r(2, 7), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pose, e := placement.NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	for i, point := range [][3]*big.Rat{jet.Point, {r(17, 5), r(0, 1), r(-16, 5)}, {r(24, 5), r(32, 5), r(0, 1)}, {r(0, 1), r(0, 1), r(-6, 1)}, {r(0, 1), r(0, 1), r(6, 1)}, {r(8, 1), r(0, 1), r(6, 1)}} {
		n, e := s.BoundaryUnitNormalsRational(ctx, point, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(i, e)
		}
		if i == 0 {
			if len(n.OriginalFaces[0]) != 1 || n.OriginalFaces[0][0] != 3 {
				t.Fatal("interior chart")
			}
			for _, d := range [][3]*big.Rat{jet.Du, jet.Dv} {
				dot := new(big.Rat)
				for j := 0; j < 3; j++ {
					dot.Add(dot, new(big.Rat).Mul(n.Units[0][j], d[j]))
				}
				if dot.Sign() != 0 {
					t.Fatal("original exact tangency")
				}
			}
		}
		if i == 1 && len(n.OriginalFaces[0]) != 4 {
			t.Fatal("quarter vertex")
		}
		if i == 5 && n.Owners != (geom.TorusCylinderWall|geom.TorusCylinderUpperCap) {
			t.Fatal("crease owners")
		}
		for owner, normal := range n.Units {
			if n.Owners&geom.BoreBoundaryMask(1<<owner) == 0 {
				continue
			}
			norm := new(big.Rat)
			for _, x := range normal {
				norm.Add(norm, new(big.Rat).Mul(x, x))
			}
			if norm.Cmp(r(1, 1)) != 0 {
				t.Fatal("not exact unit")
			}
			if i < 5 {
				epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
				for _, sign := range []int64{-1, 1} {
					var probe [3]*big.Rat
					for j := 0; j < 3; j++ {
						probe[j] = new(big.Rat).Add(point[j], new(big.Rat).Mul(new(big.Rat).Mul(epsilon, r(sign, 1)), normal[j]))
					}
					location, _, e := q.ClassifyRational(ctx, probe, geom.SurfacePatchOptions{})
					want := geom.BoreInside
					if sign > 0 {
						want = geom.BoreOutside
					}
					if e != nil || location != want {
						t.Fatal("outward material direction", i, sign, location, e)
					}
				}
			}
		}
		world, _ := pose.TransformPoint(ctx, point, false, 4096)
		placed, e := s.BoundaryPlacedUnitNormalsRational(ctx, pose, world, geom.SurfacePatchOptions{})
		if e != nil || placed.Owners != n.Owners || placed.ExactOperations != n.ExactOperations {
			t.Fatal("one source query", e)
		}
		if !reflect.DeepEqual(placed.OriginalFaces, n.OriginalFaces) {
			t.Fatal("world owner changed")
		}
		for owner, unit := range n.Units {
			if n.Owners&geom.BoreBoundaryMask(1<<owner) == 0 {
				continue
			}
			expected, err := pose.TransformVector(ctx, unit, false, 4096)
			if err != nil {
				t.Fatal(err)
			}
			for j := 0; j < 3; j++ {
				if expected[j].Cmp(placed.Units[owner][j]) != 0 {
					t.Fatal("exact world unit")
				}
			}
		}
		if path := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY_NORMALS"); path != "" {
			args := []string{}
			for _, p := range world {
				f, _ := p.Float64()
				args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
			}
			args = append(args, "oblique")
			data, e := exec.Command(path, args...).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			reader := strings.NewReader(string(data))
			mask := geom.BoreBoundaryMask(0)
			for {
				var owner int
				var v [3]float64
				_, e := fmt.Fscan(reader, &owner, &v[0], &v[1], &v[2])
				if e == io.EOF {
					break
				}
				if e != nil {
					t.Fatal(e, string(data))
				}
				slot := 0
				for (1<<slot) != owner && slot < 4 {
					slot++
				}
				if slot == 4 {
					t.Fatal("oracle owner")
				}
				mask |= geom.BoreBoundaryMask(owner)
				for j := 0; j < 3; j++ {
					got, _ := placed.Units[slot][j].Float64()
					if math.Abs(v[j]-got) > 1e-8 {
						t.Fatal("OCCT normal", i, slot, v, got)
					}
				}
			}
			if mask != placed.Owners {
				t.Fatal("OCCT owners", mask, placed.Owners)
			}
		}
	}
	if _, e := s.BoundaryUnitNormalsRational(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("interior accepted")
	}
	if bad, e := s.BoundaryUnitNormalsRational(ctx, jet.Point, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || bad.Owners != 0 {
		t.Fatal("atomic quota")
	}
}

func TestNativeObliqueCavityNormalsGuards(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := big.NewRat
	frame := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	source, e := geom.NewObliqueCylinderTorusCavityMaterial(context.Background(), 3, 1, 8, -6, 6, frame, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point := [3]*big.Rat{r(17, 5), r(0, 1), r(-16, 5)}
	if out, e := source.BoundaryUnitNormalsRational(ctx, point, geom.SurfacePatchOptions{}); !errors.Is(e, context.Canceled) || out.Owners != 0 {
		t.Fatal("cancel atomic", e)
	}
	first, e := source.BoundaryUnitNormalsRational(context.Background(), point, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	first.Units[0][0].SetInt64(999)
	second, e := source.BoundaryUnitNormalsRational(context.Background(), point, geom.SurfacePatchOptions{})
	if e != nil || second.Units[0][0].Cmp(r(-3, 5)) != 0 {
		t.Fatal("owned output", e)
	}
	point[0] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 1000))
	if out, e := source.BoundaryUnitNormalsRational(context.Background(), point, geom.SurfacePatchOptions{MaxCoefficientBits: 128}); e == nil || out.Owners != 0 {
		t.Fatal("coefficient atomic")
	}
}
