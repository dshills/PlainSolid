package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// One connected rectangular shell has two quadratic roof lobes. The C0
// seam at u=1/2 is below the lower face of the other shell. Each positive
// lobe projects to a disjoint disk of radius 5/64; its geometry does not
// depend on the hints sent to the contact solver.
func disjointLoopRoof(t *testing.T) Solid {
	t.Helper()
	x := [5]float64{.125, .3125, .5, .6875, .875}
	y := [3]float64{.25, .5, .75}
	x2 := [5]float64{9.0 / 256, -9.0 / 256, 9.0 / 256, -9.0 / 256, 9.0 / 256}
	y2 := [3]float64{1.0 / 16, -1.0 / 16, 1.0 / 16}
	const floor = -.5
	height := func(i, j int) float64 { return 25.0/4096 - x2[i] - y2[j] }
	xk := []float64{x[0], x[0], x[0], x[2], x[2], x[4], x[4], x[4]}
	yk := []float64{y[0], y[0], y[0], y[2], y[2], y[2]}
	xl, yl, zk := []float64{x[0], x[0], x[4], x[4]}, []float64{y[0], y[0], y[2], y[2]}, []float64{0, 0, 1, 1}
	var supports []*geom.BSplineSurface
	var reversed []bool
	add := func(du, dv, nu, nv int, uk, vk []float64, rev bool, at func(int, int) geom.Vec3) {
		var p []geom.Vec3
		for i := range nu {
			for j := range nv {
				p = append(p, at(i, j))
			}
		}
		s, e := geom.NewBSplineSurface(du, dv, nu, nv, p, uk, vk, nil)
		if e != nil {
			t.Fatal(e)
		}
		supports, reversed = append(supports, s), append(reversed, rev)
	}
	add(1, 1, 2, 2, xl, yl, true, func(i, j int) geom.Vec3 { return geom.Vec3{X: x[4*i], Y: y[2*j], Z: floor} })
	add(2, 2, 5, 3, xk, yk, false, func(i, j int) geom.Vec3 { return geom.Vec3{X: x[i], Y: y[j], Z: height(i, j)} })
	for _, edge := range []int{0, 4} {
		add(2, 1, 3, 2, yk, zk, edge == 0, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: x[edge], Y: y[i], Z: [2]float64{floor, height(edge, i)}[j]}
		})
	}
	for _, edge := range []int{0, 2} {
		add(2, 1, 5, 2, xk, zk, edge != 0, func(i, j int) geom.Vec3 {
			return geom.Vec3{X: x[i], Y: y[edge], Z: [2]float64{floor, height(i, edge)}[j]}
		})
	}
	s, _, e := SewSplineRectangles(context.Background(), supports, make([]geom.ParameterRectangle, len(supports)), reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestImprintDisjointClosedRoofLoops(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, 0)
	b := disjointLoopRoof(t)
	// The stock builder stores its lower Z rectangle at index four.
	aFace, bFace := a.faces[4], b.faces[1]
	d, e := faceUVBounds(bFace)
	if e != nil {
		t.Fatal(e)
	}
	// The tube proposals are discovered separately on each dome span;
	// Section rechecks coverage across both full original face domains.
	o := AlgebraicShellSectionOptions{}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	var hints intersect.SplineSeedReport
	for i, center := range []float64{5.0 / 16, 11.0 / 16} {
		span := d
		if i == 0 {
			span.UMax = .5
		} else {
			span.UMin = .5
		}
		p, e := intersectSplineFace(aFace, span)
		if e != nil {
			t.Fatal(e)
		}
		q, e := intersectSplineFace(bFace, span)
		if e != nil {
			t.Fatal(e)
		}
		opts := o.Trace.GraphFallback
		opts.UCuts = []float64{center - 1.0/16, center - 3.0/64, center + 3.0/64, center + 1.0/16}
		opts.VCuts = []float64{.5 - 1.0/16, .5 - 3.0/64, .5 + 3.0/64, .5 + 1.0/16}
		opts.Plane.Arcs.ResolveSubdivisionCorners = true
		r, e := intersect.DiscoverCommonXYGraphPairTubes(ctx, p, q, geom.SurfacePatchOptions{}, opts)
		if e != nil || len(r.Attempts) < 4 {
			t.Fatal("dome branch proposals", i, len(r.Attempts), e)
		}
		hints.Attempts = append(hints.Attempts, r.Attempts...)
	}
	o.Seeds = []AlgebraicShellSectionSeed{{Faces: [2]int{4, 1}, Tubes: hints}}
	r, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !r.Complete || len(r.Wires) != 2 || len(r.Branches) != 2 {
		t.Fatal("two complete closed sections", r.Complete, r.Pairs, len(r.Wires), len(r.Branches), e)
	}
	p, e := r.ImprintOriginalShells(ctx, AlgebraicShellImprintOptions{})
	if e != nil || p[0].Topology.InnerWires != 2 || p[1].Topology.InnerWires != 2 || p[0].Topology.Faces != 8 || p[1].Topology.Faces != 8 {
		t.Fatal("two disjoint holes and disks on each chart", p[0].Topology, p[1].Topology, e)
	}
	common, e := r.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil || common.Topology.Faces != 4 || common.Topology.EulerCharacteristic != 4 {
		t.Fatal("common should contain two independent closed lenses", common.Topology, e)
	}
	components, e := common.NativeTubeLensComponents(ctx, 1e-8, edgecheck.Options{})
	if e != nil || len(components) != 2 {
		t.Fatal("two native lens components", len(components), e)
	}
	for _, component := range components {
		if topology := component.Topology(); len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 {
			t.Fatal("native component lost its closed shell", topology)
		}
	}
	if _, e := common.NativeTubeLens(ctx, 1e-8, edgecheck.Options{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("single-lens conversion selected only part of the material", e)
	}
	if failed, e := r.ImprintOriginalShells(ctx, AlgebraicShellImprintOptions{MaxPieces: 1}); !errors.Is(e, geom.ErrConvergence) || len(failed[0].Faces)+len(failed[1].Faces) != 0 {
		t.Fatal("imprinting quota must fail atomically", e)
	}
	// Each disk contains an analytic volume πr⁴/2. The total should
	// agree with one independent rational bracket of πr⁴.
	volume, e := common.BoundVolume(ctx, AlgebraicShellVolumeOptions{MaxDepth: 8, TubeSubdivisions: 4})
	if e != nil {
		t.Fatal(e)
	}
	factor := big.NewRat(625, 16777216)
	lo := new(big.Rat).Mul(big.NewRat(3141592653589793, 1e15), factor)
	hi := new(big.Rat).Mul(big.NewRat(3141592653589794, 1e15), factor)
	if volume.Volume.Lower.Cmp(lo) >= 0 || volume.Volume.Upper.Cmp(hi) <= 0 || volume.Volume.Upper.Cmp(big.NewRat(1, 50)) >= 0 {
		t.Fatal("both components' analytic volume must lie inside certified enclosure", volume)
	}
}
