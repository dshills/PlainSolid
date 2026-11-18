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
	"plainsolid/internal/slowtest"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExternallyTangentSplineShells(t *testing.T) {
	slowtest.Skip(t)
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
		if e != nil || len(out.Faces()) != len(stock.Faces()) || len(h.FacesA) != 6 || len(h.FacesB[1]) != 0 ||
			out.faces[4].Surface != stock.faces[4].Surface || len(h.EdgesA) != len(stock.edges) || h.EdgesA[0][0] != 0 {
			t.Fatal("cut by externally tangent tool altered stock", e, h)
		}
		out, h, e = CombineSplineShells(ctx, stock, tool, parametric.Intersection, SplineShellCombineOptions{})
		if e != nil || len(out.faces) != 0 || h.Method != "isolated-graph-tangency" {
			t.Fatal("native shell dispatcher did not select the complete sole-contact proof", h.Method, e)
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
	checkOCCTExternalSplineTangency(t, "isolated-tangent-shells")
}

func TestOCCTExternallyTangentQuarticSplineShells(t *testing.T) {
	checkOCCTExternalSplineTangency(t, "isolated-quartic-tangent-shells")
}

func TestOCCTExternallyTangentSeparableSplineShells(t *testing.T) {
	checkOCCTExternalSplineTangency(t, "isolated-separable-tangent-shells")
}

func TestOCCTExternallyTangentSplineShellsMultipleKnotContacts(t *testing.T) {
	checkOCCTExternalSplineTangency(t, "multiple-knot-tangent-shells")
}

func TestOCCTExternallyTangentSplineShellsDistinctPairs(t *testing.T) {
	checkOCCTExternalSplineTangency(t, "distinct-tangent-face-pairs")
}

func checkOCCTExternalSplineTangency(t *testing.T, mode string) {
	t.Helper()
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE to the optional OCCT executable")
	}
	path := filepath.Join(t.TempDir(), "contact.json")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if b, e := exec.CommandContext(ctx, program, path, mode).CombinedOutput(); e != nil {
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

func TestExternallyTangentQuarticSplineShells(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	// z=-(9/16)*((x-.5)²+(y-.5)²)² is strictly below the plane
	// except at its unique zero. The Hessian at that point vanishes.
	tool := quarticRoofSolid(t, 0, 0)
	out, h, p, e := CombineExternallyTangentSplineShells(ctx, stock, tool, parametric.Intersection, SplineShellTangencyOptions{Faces: [2]int{4, 1}})
	if e != nil || len(out.faces) != 0 || p.Point[0].Cmp(big.NewRat(1, 2)) != 0 || p.Point[1].Cmp(big.NewRat(1, 2)) != 0 ||
		p.Point[2].Sign() != 0 || p.StrictOrder != 1 || h.Method != "isolated-graph-tangency" {
		t.Fatal("exact quartic contact", h, p, e)
	}
}

func TestExternallyTangentSeparableSplineShells(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	// The roof is z=-(x-.5)^4-(y-.5)^2. The mixed orders have no
	// definite Hessian and the complete numerator is not homogeneous quartic.
	// A separate U knot in the roof and matching original side curves make
	// the exact contact occur at an original span junction.
	tool := separableTangencyShell(t)
	o := SplineShellTangencyOptions{Faces: [2]int{4, 5}}
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		s, h, p, e := CombineExternallyTangentSplineShells(ctx, stock, tool, op, o)
		if e != nil || h.Method != "isolated-graph-tangency" || p.SeparatedPairs != 35 || p.StrictOrder != 1 ||
			p.Point[0].Cmp(big.NewRat(1, 2)) != 0 || p.Point[1].Cmp(big.NewRat(1, 2)) != 0 || p.Point[2].Sign() != 0 ||
			p.UV[1][0].Cmp(big.NewRat(1, 2)) != 0 || p.UV[1][1].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal("separable high-order original knot contact", h.Method, p, e)
		}
		if op == parametric.Intersection && len(s.faces) != 0 || op == parametric.Difference &&
			(len(s.faces) != len(stock.faces) || s.faces[4].Surface != stock.faces[4].Surface) {
			t.Fatal("regularized operation changed material", op, s.Topology())
		}
	}
	if s, h, e := CombineSplineShells(ctx, stock, tool, parametric.Intersection, SplineShellCombineOptions{}); e != nil ||
		len(s.faces) != 0 || h.Method != "isolated-graph-tangency" {
		t.Fatal("native dispatch did not preserve the separable contact", h.Method, e)
	}
}

func TestExternallyTangentSplineShellsMultipleKnotContacts(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	// An independent tensor chart consists of C0 bilinear pieces. The
	// exact height gap is nonnegative and vanishes at two interior knot
	// vertices; no full knot edge vanishes.
	tool := multipleKnotTangencyShell(t, []float64{0, .25, .5, .75, 1}, []float64{.125, 0, .125, 0, .125})
	o := SplineShellTangencyOptions{Faces: [2]int{4, 5}}
	s, h, p, e := CombineExternallyTangentSplineShells(ctx, stock, tool, parametric.Intersection, o)
	if e != nil || len(s.faces) != 0 || len(p.Additional) != 1 || p.StrictOrder != 1 || p.SeparatedPairs != 35 || h.Method != "isolated-graph-tangency" {
		t.Fatal("complete multiple-contact proof", h.Method, p, e)
	}
	for i, v := range []SplineShellTangencyPoint{{Point: p.Point, UV: p.UV}, p.Additional[0]} {
		x := big.NewRat(int64(1+2*i), 4)
		if v.Point[0].Cmp(x) != 0 || v.Point[1].Cmp(big.NewRat(1, 2)) != 0 || v.Point[2].Sign() != 0 ||
			v.UV[0][0].Cmp(x) != 0 || v.UV[1][0].Cmp(x) != 0 {
			t.Fatal("original ordered knot owners", i, v)
		}
	}
	if s, h, e = CombineSplineShells(ctx, stock, tool, parametric.Difference, SplineShellCombineOptions{}); e != nil ||
		len(s.faces) != 6 || s.faces[4].Surface != stock.faces[4].Surface || h.Method != "isolated-graph-tangency" {
		t.Fatal("native dispatch changed a cut with no common material", h.Method, e)
	}
	line := multipleKnotTangencyShell(t, []float64{0, .25, .5, .75, 1}, []float64{.125, 0, 0, 0, .125})
	if r, _, p, e := CombineExternallyTangentSplineShells(ctx, stock, line, parametric.Intersection, o); e == nil || len(r.faces) != 0 || p.Point[0] != nil {
		t.Fatal("a complete zero side was misreported as isolated contacts", p, e)
	}
}

func multipleKnotTangencyShell(t *testing.T, x, heightX []float64) Solid {
	t.Helper()
	y := []float64{0, .5, 1}
	heightY := []float64{.125, 0, .125}
	charts := [3][]float64{x, y, {0, 1}}
	var surfaces []*geom.BSplineSurface
	var reversed []bool
	knots := func(p []float64) []float64 {
		k := append([]float64{p[0]}, p...)
		return append(k, p[len(p)-1])
	}
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		for side := range 2 {
			var poles []geom.Vec3
			for i := range charts[u] {
				for j := range charts[v] {
					p := [3]int{}
					p[axis], p[u], p[v] = side*(len(charts[axis])-1), i, j
					z := -1.0
					if p[2] != 0 {
						z = -heightX[p[0]] - heightY[p[1]]
					}
					poles = append(poles, geom.Vec3{X: x[p[0]], Y: y[p[1]], Z: z})
				}
			}
			s, e := geom.NewBSplineSurface(1, 1, len(charts[u]), len(charts[v]), poles, knots(charts[u]), knots(charts[v]), nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces, reversed = append(surfaces, s), append(reversed, side == 0)
		}
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, make([]geom.ParameterRectangle, len(surfaces)), reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestExternallyTangentSplineShellsDistinctPairs(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	stock, af := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	var source []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	keep := func(s *geom.BSplineSurface, r bool) {
		source = append(source, s)
		domains = append(domains, geom.ParameterRectangle{})
		reversed = append(reversed, r)
	}
	for i, f := range af {
		if f.u != 0 && f.v != 0 {
			keep(f.surface, i%2 == 0)
			continue
		}
		p, e := geom.NewSurfacePatch(f.surface, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		var x, y *geom.SurfacePatch
		if f.u == 0 {
			x, y, e = p.SplitU(.5)
		} else {
			x, y, e = p.SplitV(.5)
		}
		if e != nil {
			t.Fatal(e)
		}
		for _, q := range [2]*geom.SurfacePatch{x, y} {
			s, e := q.ToBSplineSurface()
			if e != nil {
				t.Fatal(e)
			}
			keep(s, i%2 == 0)
		}
	}
	stock, _, e := SewSplineRectangles(ctx, source, domains, reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	tool := multipleKnotTangencyShell(t, []float64{0, .25, .5, .75, 1}, []float64{.125, 0, .125, 0, .125})
	pairs := []SplineShellTangencyPair{{Faces: [2]int{6, 5}}, {Faces: [2]int{7, 5}}}
	o := SplineShellTangencyOptions{}
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		s, h, p, e := CombineExternallyTangentSplineShellPairs(ctx, stock, tool, op, pairs, o)
		if e != nil || len(p.Pairs) != 2 || p.SeparatedPairs != 58 || h.Method != "isolated-graph-tangency" {
			t.Fatal("all original contacts and all strict separations", h.Method, p, e)
		}
		if op == parametric.Intersection && len(s.faces) != 0 || op == parametric.Difference &&
			(len(s.faces) != len(stock.faces) || s.faces[7].Surface != stock.faces[7].Surface) {
			t.Fatal("point contacts contributed material", op, s.Topology())
		}
		for i, q := range p.Pairs {
			if q.Faces != pairs[i].Faces || q.Point[0].Cmp(big.NewRat(int64(1+2*i), 4)) != 0 || q.Point[2].Sign() != 0 {
				t.Fatal("exact original face pair ownership", i, q)
			}
		}
	}
	if s, h, e := CombineSplineShells(ctx, stock, tool, parametric.Intersection, SplineShellCombineOptions{}); e != nil ||
		len(s.faces) != 0 || h.Method != "isolated-graph-tangency" {
		t.Fatal("native dispatch did not identify both original pairs", h.Method, e)
	}
	budget := o
	budget.Contact.MaxOperations = 1
	if s, _, p, e := CombineExternallyTangentSplineShellPairs(ctx, stock, tool, parametric.Intersection, pairs, budget); !errors.Is(e, geom.ErrConvergence) ||
		len(s.faces) != 0 || p.Pairs != nil {
		t.Fatal("budget exhaustion returned partial geometry or proof", p, e)
	}
	bad := []SplineShellTangencyPair{pairs[0], {Faces: [2]int{8, 5}}}
	if s, _, p, e := CombineExternallyTangentSplineShellPairs(ctx, stock, tool, parametric.Intersection, bad, o); e == nil || len(s.faces) != 0 || p.Pairs != nil {
		t.Fatal("omitting one original contact treated it as separated", p, e)
	}
}

func separableTangencyShell(t *testing.T) Solid {
	t.Helper()
	degree := [3]int{4, 2, 1}
	var surfaces []*geom.BSplineSurface
	var reversed []bool
	knots := func(n int) []float64 {
		k := make([]float64, 2*n)
		for i := n; i < len(k); i++ {
			k[i] = 1
		}
		return k
	}
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		for side := range 2 {
			var poles []geom.Vec3
			for i := 0; i <= degree[u]; i++ {
				for j := 0; j <= degree[v]; j++ {
					p := [3]int{}
					p[axis], p[u], p[v] = side*degree[axis], i, j
					z := -1.0
					if p[2] != 0 {
						z = -[5]float64{1, -1, 1, -1, 1}[p[0]]/16 - [3]float64{1, -1, 1}[p[1]]/4
					}
					poles = append(poles, geom.Vec3{X: float64(p[0]) / 4, Y: float64(p[1]) / 2, Z: z})
				}
			}
			nu, nv := degree[u]+1, degree[v]+1
			s, e := geom.NewBSplineSurface(nu-1, nv-1, nu, nv, poles, knots(nu), knots(nv), nil)
			if e != nil {
				t.Fatal(e)
			}
			if axis != 0 {
				p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if u == 0 {
					p, e = p.InsertUKnot(.5, 4, geom.SurfacePatchOptions{})
				} else {
					p, e = p.InsertVKnot(.5, 4, geom.SurfacePatchOptions{})
				}
				if e != nil {
					t.Fatal(e)
				}
				s, e = p.ToBSplineSurface()
				if e != nil {
					t.Fatal(e)
				}
			}
			surfaces = append(surfaces, s)
			reversed = append(reversed, side == 0)
		}
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, make([]geom.ParameterRectangle, len(surfaces)), reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
