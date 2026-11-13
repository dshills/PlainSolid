package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type splineBoxFace struct {
	axis, u, v int
	fixed      float64
	domain     geom.ParameterRectangle
	surface    *geom.BSplineSurface
}

func makeOpenSplineBox(t *testing.T, lo, hi [3]float64, warp float64) (Solid, []splineBoxFace) {
	t.Helper()
	var f []splineBoxFace
	var surfaces []*geom.BSplineSurface
	var d []geom.ParameterRectangle
	var rev []bool
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		for side, x := range [2]float64{lo[axis], hi[axis]} {
			domain := geom.ParameterRectangle{UMin: lo[u], UMax: hi[u], VMin: lo[v], VMax: hi[v]}
			p := []geom.Vec3{}
			for _, a := range [2]float64{lo[u], hi[u]} {
				for _, b := range [2]float64{lo[v], hi[v]} {
					c := [3]float64{}
					c[axis], c[u], c[v] = x, a, b
					c[2] += warp * c[0] * c[1]
					p = append(p, geom.Vec3{X: c[0], Y: c[1], Z: c[2]})
				}
			}
			s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, []float64{lo[u], lo[u], hi[u], hi[u]}, []float64{lo[v], lo[v], hi[v], hi[v]}, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces, d, rev = append(surfaces, s), append(d, domain), append(rev, side == 0)
			f = append(f, splineBoxFace{axis, u, v, x, domain, s})
		}
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, d, rev, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s, f
}

func TestCombineOpenSplineShellsBoxes(t *testing.T) {
	testCombineOpenSplineShells(t, 0)
}

func TestCombineOpenSplineShellsWarpedBoxes(t *testing.T) {
	testCombineOpenSplineShells(t, .125)
}

func testCombineOpenSplineShells(t *testing.T, warp float64) {
	t.Helper()
	ctx := context.Background()
	al, ah := [3]float64{0, 0, 0}, [3]float64{1, 1, 1}
	bl, bh := [3]float64{.5, .5, -.5}, [3]float64{1.5, 1.5, .5}
	a, af := makeOpenSplineBox(t, al, ah, warp)
	b, bf := makeOpenSplineBox(t, bl, bh, warp)
	var sections []SplineShellOpenSection
	for i, x := range af {
		for j, y := range bf {
			if x.axis == y.axis || x.fixed <= bl[x.axis] || x.fixed >= bh[x.axis] || y.fixed <= al[y.axis] || y.fixed >= ah[y.axis] {
				continue
			}
			axis := 3 - x.axis - y.axis
			lo, hi := max(al[axis], bl[axis]), min(ah[axis], bh[axis])
			mid := lo/2 + hi/2
			startOperand := 0
			if lo == bl[axis] {
				startOperand = 1
			}
			free := func(f splineBoxFace, index int) int {
				if f.u == axis {
					return 2 * index
				}
				return 2*index + 1
			}
			patch := func(f splineBoxFace) intersect.Patch {
				d := f.domain
				return intersect.Patch{Surface: f.surface, Min: geom.Vec2{X: d.UMin, Y: d.VMin}, Max: geom.Vec2{X: d.UMax, Y: d.VMax}}
			}
			s := SplineShellOpenSection{FaceA: i, FaceB: j}
			for half := range 2 {
				operand := startOperand ^ half
				axis := free([2]splineBoxFace{x, y}[operand], operand)
				d := [2]geom.ParameterRectangle{x.domain, y.domain}
				span := [2]float64{lo, mid}
				if half == 1 {
					span = [2]float64{mid, hi}
				}
				if axis%2 == 0 {
					d[operand].UMin, d[operand].UMax = span[0], span[1]
				} else {
					d[operand].VMin, d[operand].VMax = span[0], span[1]
				}
				p, _ := geom.NewSurfacePatch(x.surface, geom.SurfacePatchOptions{})
				q, _ := geom.NewSurfacePatch(y.surface, geom.SurfacePatchOptions{})
				p, _ = p.Trim(d[0])
				q, _ = q.Trim(d[1])
				c, e := geom.CertifySurfacePairTube(p, q, geom.SurfacePairTubeOptions{FreeAxis: axis})
				if e != nil || !c.ProvenBranch {
					t.Fatal(i, j, half, c, e)
				}
				s.TubeCells.Attempts = append(s.TubeCells.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: d[0], DomainB: d[1]}, Tube: &c})
			}
			branch, e := intersect.JoinSplineTubeBranches(ctx, patch(x), patch(y), s.TubeCells, intersect.TubeBranchOptions{})
			if e != nil || len(branch.Branches) != 1 {
				t.Fatal(i, j, branch, e)
			}
			segments, e := intersect.SplineBranchCurves(ctx, patch(x), patch(y), s.TubeCells, branch.Branches[0], intersect.TubeCurveOptions{})
			if e != nil {
				t.Fatal(e)
			}
			ends := [2]SplineShellOpenEndpoint{}
			for n := range 2 {
				p, e := segments[n].EvaluatePair(float64(n))
				if e != nil {
					t.Fatal(e)
				}
				var found bool
				for operand, uv := range [2]geom.Vec2{p.A, p.B} {
					f := [2]splineBoxFace{x, y}[operand]
					d := f.domain
					for side, hit := range [4]bool{uv.X == d.UMin, uv.X == d.UMax, uv.Y == d.VMin, uv.Y == d.VMax} {
						if hit {
							if found {
								t.Fatal("edge/edge endpoint")
							}
							found = true
							// The fixture is authored by the invertible
							// warp z'=z+kxy, so the analytic intersection
							// parameters are the original box coordinates.
							p := [3]float64{}
							p[x.axis], p[y.axis] = x.fixed, y.fixed
							p[axis] = lo
							if side == int(geom.SurfaceEdgeUMax) || side == int(geom.SurfaceEdgeVMax) {
								p[axis] = hi
							}
							ends[n] = SplineShellOpenEndpoint{Operand: operand, Edge: SplineTubeEndpoint{Side: geom.SurfaceEdge(side),
								Hint: [4]*big.Rat{ratInset(p[x.u]), ratInset(p[x.v]), ratInset(p[y.u]), ratInset(p[y.v])}}}
						}
					}
				}
				if !found {
					t.Fatal("no endpoint edge")
				}
			}
			s.Start, s.End = ends[0], ends[1]
			sections = append(sections, s)
		}
	}
	if len(sections) != 6 {
		t.Fatal(len(sections))
	}
	for _, tc := range []struct {
		op     parametric.BooleanOp
		volume float64
	}{{parametric.Union, 1.875}, {parametric.Intersection, .125}, {parametric.Difference, .875}} {
		s, h, e := CombineOpenSplineShells(ctx, a, b, tc.op, sections, SplineShellOpenOptions{})
		if e != nil || len(h.SectionEdges) != 12 || len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(tc, len(h.SectionEdges), len(h.Pairs), s.Topology(), e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-tc.volume) > 1e-9 {
			t.Fatal(tc, p, e)
		}
		if tc.op == parametric.Intersection {
			var data bytes.Buffer
			if e := WriteJSON(&data, s); e != nil {
				t.Fatal(e)
			}
			raw := append([]byte(nil), data.Bytes()...)
			out, e := ReadJSONContext(ctx, &data, JSONLimits{})
			if e != nil || out.Topology().EulerCharacteristic != 2 {
				t.Fatal(out.Topology(), e)
			}
			if warp != 0 {
				var recipe nativeSolid
				if e := json.Unmarshal(raw, &recipe); e != nil || len(recipe.Tubes) == 0 {
					t.Fatal(e)
				}
				if len(recipe.Tubes[0].ExactEndpoints) == 0 {
					t.Fatal("missing exact endpoint witness")
				}
				recipe.Tubes[0].ExactEndpoints[0].Parameters[0] += 1.0 / 64
				bad, e := json.Marshal(recipe)
				if e != nil {
					t.Fatal(e)
				}
				if _, e := ReadJSON(bytes.NewReader(bad), JSONLimits{}); e == nil {
					t.Fatal("accepted a corrupt exact endpoint witness")
				}
			}
		}
	}
	if _, _, e := CombineOpenSplineShells(ctx, a, b, parametric.Union, sections[:5], SplineShellOpenOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("missing face-pair contact was not rejected", e)
	}
	x := sections[0].Start.Edge.Hint[0]
	sections[0].Start.Edge.Hint[0] = new(big.Rat).Add(x, big.NewRat(1, 16))
	if _, _, e := CombineOpenSplineShells(ctx, a, b, parametric.Union, sections, SplineShellOpenOptions{}); e == nil {
		t.Fatal("incorrect exact endpoint hint was not rejected")
	}
}
