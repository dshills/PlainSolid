package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestOverlappingCurvedGraphLens(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1.0 / 64}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	x, z := [3]float64{0, .5, 1}, [3]float64{.25, -.25, .25}
	var poles []geom.Vec3
	for i := range x {
		for j := range x {
			poles = append(poles, geom.Vec3{X: x[i] + .125, Y: x[j], Z: z[i] + z[j] - 25.0/256 + (x[i]+.125)*x[j]/64})
		}
	}
	k = []float64{0, 0, 0, 1, 1, 1}
	b, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patches := [2]intersect.Patch{{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, {Surface: b, Max: geom.Vec2{X: 1, Y: 1}}}
	loop := func(p [4]geom.Vec2) []Use {
		r := make([]Use, 4)
		for i := range r {
			r[i] = Use{Edge: i, UVStart: p[i], UVEnd: p[(i+1)%4]}
		}
		return r
	}
	outer := loop([4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}})
	faces := [2]Face{{Surface: a, Outer: outer}, {Surface: b, Outer: outer}}
	o := AlgebraicTrimmedPairSectionOptions{}
	cuts := []float64{.25, .3125, .6875, .75}
	o.Trace.GraphFallback.UCuts = []float64{.375, .4375, .8125, .875}
	o.Trace.GraphFallback.VCuts = cuts
	o.Trace.GraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000

	lens, e := SealOverlappingAffineGraphLens(ctx, patches, faces, o.Trace.GraphFallback, o, AlgebraicTubeLensOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if lens.Topology().EulerCharacteristic != 2 || lens.Topology().Faces != 2 {
		t.Fatal(lens.Topology())
	}
	q, e := lens.VerticalInterval(ctx, [2]*big.Rat{big.NewRat(5, 8), big.NewRat(1, 2)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !q.Present || !q.Complete || q.Upper.Cmp(big.NewRat(5, 1024)) != 0 || q.Lower.Cmp(big.NewRat(-95, 1024)) != 0 {
		t.Fatal(q, e)
	}
	if lens.Faces()[0].Support != a || lens.Faces()[1].Support != b {
		t.Fatal("original supports replaced")
	}
	var buffer bytes.Buffer
	if e = WriteAlgebraicTubeLensJSON(&buffer, lens); e != nil {
		t.Fatal(e)
	}
	o.Trace.Coverage.OverlappingAffineXYFallback = true
	restored, e := ReadAlgebraicTubeLensJSON(ctx, &buffer, JSONLimits{}, o, AlgebraicTubeLensOptions{})
	if e != nil || restored.Topology() != lens.Topology() {
		t.Fatal("native full-domain reproof", e)
	}
	m, e := lens.TessellateAffineClosedCertified(ctx, AlgebraicTubeLensMeshOptions{Boundary: AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .04}}}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02}})
	if e != nil || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal("display", e)
	}
	want := math.Pi * math.Pow(5.0/16, 4) / 2
	if m.Mesh.ValidateSolid() != nil || math.Abs(m.Mesh.SignedVolume()-want) > .15*want {
		t.Fatal("analytic volume", m.Mesh.SignedVolume(), want)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := SealOverlappingAffineGraphLens(canceled, patches, faces, o.Trace.GraphFallback, o, AlgebraicTubeLensOptions{})
	if !errors.Is(e, context.Canceled) || bad.Topology().Faces != 0 {
		t.Fatal(e)
	}

}
