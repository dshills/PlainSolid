package occtcheck

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func oracleOpenRat(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }

type oracleOpenFace struct {
	axis, u, v int
	fixed      float64
	domain     geom.ParameterRectangle
	surface    *geom.BSplineSurface
}

func oracleOpenBox(t *testing.T, lo, hi [3]float64) (trimmed.Solid, []oracleOpenFace) {
	t.Helper()
	var f []oracleOpenFace
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
					p = append(p, geom.Vec3{X: c[0], Y: c[1], Z: c[2]})
				}
			}
			s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, []float64{lo[u], lo[u], hi[u], hi[u]}, []float64{lo[v], lo[v], hi[v], hi[v]}, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces, d, rev = append(surfaces, s), append(d, domain), append(rev, side == 0)
			f = append(f, oracleOpenFace{axis, u, v, x, domain, s})
		}
	}
	s, _, e := trimmed.SewSplineRectangles(context.Background(), surfaces, d, rev, trimmed.SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s, f
}

func TestOCCTOpenSplineShells(t *testing.T) {
	refs := runOracle(t, "open-spline-box")
	ctx := context.Background()
	al, ah := [3]float64{0, 0, 0}, [3]float64{1, 1, 1}
	bl, bh := [3]float64{.5, .5, -.5}, [3]float64{1.5, 1.5, .5}
	a, af := oracleOpenBox(t, al, ah)
	b, bf := oracleOpenBox(t, bl, bh)
	var sections []trimmed.SplineShellOpenSection
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
			free := func(f oracleOpenFace, index int) int {
				if f.u == axis {
					return 2 * index
				}
				return 2*index + 1
			}
			patch := func(f oracleOpenFace) intersect.Patch {
				d := f.domain
				return intersect.Patch{Surface: f.surface, Min: geom.Vec2{X: d.UMin, Y: d.VMin}, Max: geom.Vec2{X: d.UMax, Y: d.VMax}}
			}
			s := trimmed.SplineShellOpenSection{FaceA: i, FaceB: j}
			for half := range 2 {
				operand := startOperand ^ half
				axis := free([2]oracleOpenFace{x, y}[operand], operand)
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
			ends := [2]trimmed.SplineShellOpenEndpoint{}
			for n := range 2 {
				p, e := segments[n].EvaluatePair(float64(n))
				if e != nil {
					t.Fatal(e)
				}
				var found bool
				for operand, uv := range [2]geom.Vec2{p.A, p.B} {
					f := [2]oracleOpenFace{x, y}[operand]
					d := f.domain
					for side, hit := range [4]bool{uv.X == d.UMin, uv.X == d.UMax, uv.Y == d.VMin, uv.Y == d.VMax} {
						if hit {
							if found {
								t.Fatal("edge/edge endpoint")
							}
							found = true
							ends[n] = trimmed.SplineShellOpenEndpoint{Operand: operand, Edge: trimmed.SplineTubeEndpoint{Side: geom.SurfaceEdge(side),
								Hint: [4]*big.Rat{oracleOpenRat(p.A.X), oracleOpenRat(p.A.Y), oracleOpenRat(p.B.X), oracleOpenRat(p.B.Y)}}}
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
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		name := []string{"open-union", "open-common", "open-cut"}[i]
		t.Run(name, func(t *testing.T) {
			s, h, e := trimmed.CombineOpenSplineShells(ctx, a, b, op, sections, trimmed.SplineShellOpenOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if len(h.SectionEdges) != 12 || len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
				t.Fatal("topology", s.Topology())
			}
			p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			compare(t, refs[name], p.Area, p.Volume, p.Centroid)
		})
	}
}
