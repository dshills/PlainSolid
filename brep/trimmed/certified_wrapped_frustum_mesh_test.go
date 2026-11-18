package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func certifiedWrappedFrustumFixture(t *testing.T, circular bool, upper float64) *geom.WrappedSurfacePatch {
	t.Helper()
	q := [][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	weights := []float64{1, .75, 1, .75, 1, .75, 1, .75, 1}
	if circular {
		weights = []float64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	}
	var poles []geom.Vec3
	var w []float64
	for i, p := range q {
		for j, r := range []float64{2, upper} {
			poles = append(poles, geom.Vec3{X: r * p[0], Y: r * p[1], Z: float64(j)*5 - 2})
			w = append(w, weights[i])
		}
	}
	s, e := geom.NewBSplineSurface(2, 1, 9, 2, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{-1, -1, 1, 1}, w)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	out, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func TestCertifiedClosedWrappedFrustum(t *testing.T) {
	slowtest.Skip(t)
	for _, tc := range []struct {
		name     string
		circular bool
		upper    float64
	}{{"circle-cylinder", true, 2}, {"rational-cylinder", false, 2}, {"circle-draft", true, 1}, {"rational-draft", false, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			w := certifiedWrappedFrustumFixture(t, tc.circular, tc.upper)
			options := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}}
			m, e := TessellateWrappedFrustumClosedCertified(ctx, w, options)
			if e != nil {
				t.Fatal(e)
			}
			if e := m.Mesh.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			if !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
				t.Fatal("embedding")
			}
			if len(m.OriginalFace) != len(m.Mesh.Triangles) || len(m.SquaredErrors) != len(m.Mesh.Triangles) {
				t.Fatal("ancestry")
			}
			edges := map[[2]int]int{}
			owners := map[[2]int]map[int]bool{}
			for i, tri := range m.Mesh.Triangles {
				for j, a := range tri {
					b := tri[(j+1)%3]
					if a > b {
						a, b = b, a
					}
					key := [2]int{a, b}
					edges[key]++
					if owners[key] == nil {
						owners[key] = map[int]bool{}
					}
					owners[key][m.OriginalFace[i]] = true
				}
			}
			if len(m.Mesh.Vertices)-len(edges)+len(m.Mesh.Triangles) != 2 {
				t.Fatal("Euler")
			}
			for cap, ring := range m.RimVertices {
				for i, a := range ring {
					b := ring[(i+1)%len(ring)]
					if a > b {
						a, b = b, a
					}
					key := [2]int{a, b}
					if edges[key] != 2 || !owners[key][0] || !owners[key][cap+1] {
						t.Fatal("common rim")
					}
				}
			}

			for cap, ring := range m.RimVertices {
				for i, a := range ring {
					b := ring[(i+1)%len(ring)]
					u := new(big.Rat).Quo(new(big.Rat).Add(ratInset(m.Lateral.UParameters[i]), ratInset(m.Lateral.UParameters[i+1])), big.NewRat(2, 1))
					vv := m.Lateral.VParameters[0]
					if cap == 1 {
						vv = m.Lateral.VParameters[len(m.Lateral.VParameters)-1]
					}
					exact, e := w.FundamentalPatch().EvaluateRational(ctx, u, ratInset(vv), geom.SurfacePatchEvaluationOptions{})
					if e != nil {
						t.Fatal(e)
					}
					center := [3]*big.Rat{m.Embedding.CenterX, m.Embedding.CenterY, m.Embedding.LowerZ}
					if cap == 1 {
						center[2] = m.Embedding.UpperZ
					}
					pa, pb := exactBoundPoint(m.Mesh.Vertices[a]), exactBoundPoint(m.Mesh.Vertices[b])
					squared := new(big.Rat)
					for axis := 0; axis < 3; axis++ {
						source := new(big.Rat).Add(center[axis], new(big.Rat).Mul(big.NewRat(3, 8), new(big.Rat).Sub(exact.Point[axis], center[axis])))
						chord := new(big.Rat).Quo(new(big.Rat).Add(pa[axis], pb[axis]), big.NewRat(2, 1))
						display := new(big.Rat).Add(center[axis], new(big.Rat).Mul(big.NewRat(3, 8), new(big.Rat).Sub(chord, center[axis])))
						d := new(big.Rat).Sub(source, display)
						squared.Add(squared, new(big.Rat).Mul(d, d))
					}
					if squared.Cmp(m.SourceImageSquaredDistance) > 0 {
						t.Fatal("independent radial correspondence violates certificate")
					}
				}
			}
			area := math.Pi
			if !tc.circular { // Independent dense shoelace integral of one normalized quadratic arc.
				const n = 65536
				px, py := 1.0, 0.0
				sum := 0.0
				for i := 1; i <= n; i++ {
					u := float64(i) / n
					a, b, c := (1-u)*(1-u), 1.5*u*(1-u), u*u
					d := a + b + c
					x, y := (a+b)/d, (b+c)/d
					sum += px*y - py*x
					px, py = x, y
				}
				area = 2 * sum
			}
			want := area * 5 * (4 + 2*tc.upper + tc.upper*tc.upper) / 3
			got, _ := m.SignedDisplayVolume.Float64()
			if math.Abs(got-want) > .04*want {
				t.Fatal(got, want)
			}
			if m.SourceImageSquaredDistance.Cmp(ratInset(.0025)) > 0 {
				t.Fatal("deflection")
			}
			t.Log(len(m.Mesh.Vertices), len(m.Mesh.Triangles), got, want)
			options.MaxTriangles = 1
			bad, e := TessellateWrappedFrustumClosedCertified(ctx, w, options)
			if e == nil || len(bad.Mesh.Vertices) != 0 {
				t.Fatal("partial budget result", e)
			}
		})
	}
}

func TestCertifiedWrappedFrustumAtomicLimits(t *testing.T) {
	w := certifiedWrappedFrustumFixture(t, false, 1)
	base := CertifiedWrappedFrustumMeshOptions{Patch: CertifiedPatchMeshOptions{Deflection: .05}}
	for _, name := range []string{"predicates", "checks", "contacts"} {
		t.Run(name, func(t *testing.T) {
			o := base
			switch name {
			case "predicates":
				o.MaxPredicates = 1
			case "checks":
				o.Patch.MaxTriangleChecks = 1
			case "contacts":
				o.Collision.MaxPairs = 1
			}
			m, e := TessellateWrappedFrustumClosedCertified(context.Background(), w, o)
			if e == nil || len(m.Mesh.Vertices) != 0 || m.SourceImageSquaredDistance != nil {
				t.Fatal("partial or false certificate", e)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m, e := TessellateWrappedFrustumClosedCertified(ctx, w, base)
	if e != context.Canceled || len(m.Mesh.Vertices) != 0 {
		t.Fatal(e)
	}
}
