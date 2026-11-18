package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func pairedOwnerRoofShell(t *testing.T, second bool) Solid {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var reversed []bool
	add := func(nu, nv int, rev bool, point func(int, int) geom.Vec3) {
		var poles []geom.Vec3
		for i := 0; i < nu; i++ {
			for j := 0; j < nv; j++ {
				poles = append(poles, point(i, j))
			}
		}
		knots := func(n int) []float64 {
			k := make([]float64, 2*n)
			for i := n; i < 2*n; i++ {
				k[i] = 1
			}
			return k
		}
		p, e := geom.NewBSplineSurface(nu-1, nv-1, nu, nv, poles, knots(nu), knots(nv), nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces = append(surfaces, p)
		reversed = append(reversed, rev)
	}
	z := func(i, j int) float64 {
		v := float64(i*j) / 256
		if second {
			v += []float64{3.0 / 16, -5.0 / 16, 3.0 / 16}[i] + []float64{0, 1.0 / 128, 0}[j]
		}
		return v
	}
	floor := -1.
	if second {
		floor = -2
	}
	add(2, 2, true, func(i, j int) geom.Vec3 { return geom.Vec3{X: float64(i), Y: float64(j), Z: floor} })
	add(3, 3, false, func(i, j int) geom.Vec3 { return geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z(i, j)} })
	for _, edge := range []int{0, 2} {
		add(3, 2, edge == 0, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: float64(edge) / 2, Y: float64(i) / 2, Z: []float64{floor, z(edge, i)}[j]}
		})
	}
	for _, edge := range []int{0, 2} {
		add(3, 2, edge == 2, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: float64(i) / 2, Y: float64(edge) / 2, Z: []float64{floor, z(i, edge)}[j]}
		})
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, make([]geom.ParameterRectangle, len(surfaces)), reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestAlgebraicShellFacePairOriginalOwners(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b := pairedOwnerRoofShell(t, false), pairedOwnerRoofShell(t, true)
	o := AlgebraicShellFacePairOwnerOptions{}
	o.Trace.UniformGraphFallback = intersect.UniformGraphDiscoveryOptions{GridDivisions: 16, AxisDivisions: [2]int{8, 16}}
	o.Trace.UniformGraphFallback.Enabled = true
	o.Trace.UniformGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.UniformGraphFallback.MaxArcs = 1024
	o.Trace.UniformGraphFallback.MaxTubes = 1024
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	o.Trace.Coverage.MaxCandidateTests = 4096
	for _, pair := range [][2]Solid{{a, b}, {b, a}} {
		report, e := SectionSplineShellFacePairAlgebraicOwners(ctx, pair[0], pair[1], [2]int{1, 1}, o)
		if e != nil {
			t.Fatal("real shell owners", e)
		}
		rows := report.Branches()
		if len(rows) != 2 {
			t.Fatal("omitted branch")
		}
		for _, row := range rows {
			if row.Closed {
				t.Fatal("unexpected closed")
			}
			for _, end := range row.Ends {
				for operand, owner := range end.Owners {
					if owner.Operand != operand || owner.Face != 1 || owner.Edge < 0 || owner.EdgeParameter == nil {
						t.Fatal("lost source edge", owner)
					}
					if owner.Incidents[0].Face == owner.Incidents[1].Face || (owner.Incidents[0].Reversed != owner.Incidents[0].FaceReversed) == (owner.Incidents[1].Reversed != owner.Incidents[1].FaceReversed) {
						t.Fatal("invalid incidence pair")
					}
					found := 0
					for _, incident := range owner.Incidents {
						face := pair[operand].faces[incident.Face]
						use := face.Outer[incident.LocalEdge]
						if use.Edge != owner.Edge || use.Reversed != incident.Reversed {
							t.Fatal("stale coedge identity")
						}
						if incident.Face == 1 {
							found++
						}
						expected := owner.EdgeParameter
						if incident.Reversed {
							expected, _, e = expected.AffineMap(ctx, big.NewRat(1, 1), big.NewRat(-1, 1), geom.ExactCurvePlaneEventOptions{})
							if e != nil {
								t.Fatal(e)
							}
						}
						order, err := expected.Compare(ctx, incident.Parameter, geom.ExactCurvePlaneEventOptions{})
						if err != nil || !order.Proven || order.Order != 0 {
							t.Fatal("coedge parameter orientation", err)
						}

						same, _, err := incident.ModelEvent.CompareModelPoint(ctx, owner.ModelEvent, geom.ExactCurvePlaneEventOptions{})
						if err != nil || !same {
							t.Fatal("neighbor does not share exact model event", err)
						}
					}
					if found != 1 {
						t.Fatal("selected face occurrence missing")
					}
				}
			}
		}
		rows[0].Ends[0].Owners[0].Edge = -1
		if report.Branches()[0].Ends[0].Owners[0].Edge < 0 {
			t.Fatal("mutable ledger")
		}
	}
	broken := a
	broken.faces = append([]Face(nil), a.faces...)
	broken.faces[4].Outer = append([]Use(nil), a.faces[4].Outer...)
	broken.faces[4].Outer[0].UVStart.X += .125
	if result, e := SectionSplineShellFacePairAlgebraicOwners(ctx, broken, b, [2]int{1, 1}, o); e == nil || result != nil {
		t.Fatal("mismatched neighbor accepted")
	}

	duplicate := a
	duplicate.faces = append(append([]Face(nil), a.faces...), a.faces[4])
	if result, e := SectionSplineShellFacePairAlgebraicOwners(ctx, duplicate, b, [2]int{1, 1}, o); e == nil || result != nil {
		t.Fatal("extra incident face accepted")
	}
	missing := a
	missing.faces = append([]Face(nil), a.faces[:5]...)
	if result, e := SectionSplineShellFacePairAlgebraicOwners(ctx, missing, b, [2]int{1, 1}, o); e == nil || result != nil {
		t.Fatal("missing incident face accepted")
	}
	limited := o
	limited.MaxWork = 1
	if result, e := SectionSplineShellFacePairAlgebraicOwners(ctx, a, b, [2]int{1, 1}, limited); e == nil || result != nil {
		t.Fatal("atomic owner work exhaustion")
	}
}
