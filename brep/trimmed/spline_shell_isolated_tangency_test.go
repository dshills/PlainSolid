package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExternallyTangentSplineShells(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	roof := func(delta float64) Solid {
		var surfaces []*geom.BSplineSurface
		var reversed []bool
		knots := func(n int) []float64 {
			k := make([]float64, 2*n)
			for i := n; i < 2*n; i++ {
				k[i] = 1
			}
			return k
		}
		add := func(nu, nv int, rev bool, f func(i, j int) geom.Vec3) {
			t.Helper()
			var p []geom.Vec3
			for i := range nu {
				for j := range nv {
					p = append(p, f(i, j))
				}
			}
			s, e := geom.NewBSplineSurface(nu-1, nv-1, nu, nv, p, knots(nu), knots(nv), nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces, reversed = append(surfaces, s), append(reversed, rev)
		}
		coeff := [3]float64{.25, -.25, .25}
		z := func(i, j int) float64 { return delta - coeff[i] - coeff[j] }
		add(2, 2, true, func(i, j int) geom.Vec3 { return geom.Vec3{X: float64(i), Y: float64(j), Z: -1} })
		add(3, 3, false, func(i, j int) geom.Vec3 { return geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z(i, j)} })
		for _, edge := range []int{0, 2} {
			add(3, 2, edge == 0, func(i, j int) geom.Vec3 {
				return geom.Vec3{X: float64(edge) / 2, Y: float64(i) / 2, Z: []float64{-1, z(edge, i)}[j]}
			})
		}
		for _, edge := range []int{0, 2} {
			add(3, 2, edge != 0, func(i, j int) geom.Vec3 {
				return geom.Vec3{X: float64(i) / 2, Y: float64(edge) / 2, Z: []float64{-1, z(i, edge)}[j]}
			})
		}
		s, _, e := SewSplineRectangles(ctx, surfaces, make([]geom.ParameterRectangle, len(surfaces)), reversed, SewOptions{Tolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	b := roof(0)
	wide, _ := makeOpenSplineBox(t, [3]float64{-.25, 0, 0}, [3]float64{.875, 1, 1}, 0)
	if out, _, p, e := CombineExternallyTangentSplineShells(ctx, wide, b, parametric.Intersection,
		SplineShellTangencyOptions{Faces: [2]int{4, 1}}); e != nil || len(out.Faces()) != 0 ||
		p.SeparatedPairs != 35 || p.Point[0].Cmp(big.NewRat(1, 2)) != 0 || p.UV[1][1].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("sole contact on unequal original graph rectangles", p, e)
	}
	for _, rotated := range []bool{false, true} {
		stock, tool := a, b
		o := SplineShellTangencyOptions{Faces: [2]int{4, 1}}
		want := [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), new(big.Rat)}
		if rotated {
			m := geom.Affine{{0, 0, 1, 0}, {1, 0, 0, 0}, {0, 1, 0, 0}}
			var e error
			stock, e = a.Transformed(m)
			if e != nil {
				t.Fatal(e)
			}
			tool, e = b.Transformed(m)
			if e != nil {
				t.Fatal(e)
			}
			o.Projection = [2]int{1, 2}
			want = [3]*big.Rat{want[2], want[0], want[1]}
		}
		out, h, p, e := CombineExternallyTangentSplineShells(ctx, stock, tool, parametric.Intersection, o)
		if e != nil || len(out.Faces()) != 0 || p.StrictOrder != 1 || p.SeparatedPairs != 35 || h.Method != "isolated-graph-tangency" {
			t.Fatal("regularized common and exact tangency", rotated, p, e)
		}
		for i, v := range want {
			if p.Point[i].Cmp(v) != 0 {
				t.Fatal("world contact", p.Point, want)
			}
		}
		for _, uv := range p.UV {
			if uv[0].Cmp(big.NewRat(1, 2)) != 0 || uv[1].Cmp(big.NewRat(1, 2)) != 0 {
				t.Fatal("original owner UV", uv)
			}
		}
		out, h, _, e = CombineExternallyTangentSplineShells(ctx, stock, tool, parametric.Difference, o)
		if e != nil || len(out.Faces()) != len(stock.Faces()) || len(h.FacesA) != 6 || len(h.FacesB[1]) != 0 || out.faces[4].Surface != stock.faces[4].Surface {
			t.Fatal("cut by externally tangent tool altered stock", e, h)
		}
		if out, _, _, e := CombineExternallyTangentSplineShells(ctx, stock, tool, parametric.Union, o); !errors.Is(e, geom.ErrConvergence) || len(out.Faces()) != 0 {
			t.Fatal("point-only union should not emit a manifold", e)
		}
	}
	for _, delta := range []float64{-.03125, .03125} {
		out, _, p, e := CombineExternallyTangentSplineShells(ctx, a, roof(delta), parametric.Intersection, SplineShellTangencyOptions{Faces: [2]int{4, 1}})
		if e == nil || len(out.Faces()) != 0 || p.Point[0] != nil {
			t.Fatal("separation or overlap was misclassified as isolated tangency", delta, p, e)
		}
	}
}

func TestOCCTExternallyTangentSplineShells(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE to the optional OCCT executable")
	}
	path := filepath.Join(t.TempDir(), "contact.json")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if b, e := exec.CommandContext(ctx, program, path, "isolated-tangent-shells").CombinedOutput(); e != nil {
		t.Fatal("independent OCCT tangency", e, string(b))
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var p struct {
		Name         string
		Area, Volume float64
		Centroid     [3]float64
	}
	// The reference independently checks an empty common and publishes
	// the cut result only after validating its native OCCT boundary.
	if e := json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	if p.Name != "isolated-tangent-stock-cut" || math.Abs(p.Area-6) > 1e-10 ||
		math.Abs(p.Volume-1) > 1e-10 || math.Abs(p.Centroid[0]-.5)+math.Abs(p.Centroid[1]-.5)+math.Abs(p.Centroid[2]-.5) > 1e-10 {
		t.Fatal("cut at isolated contact should retain the unit stock", p)
	}
}
