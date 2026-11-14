package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestClipIntersectingRationalGraphCapsCurvedVoid(t *testing.T) {
	ctx := context.Background()
	stock := makeWarpedSquareRationalGraphTube(t, 1./8)
	makeCut := func(h, slope float64, keep bool) RationalGraphHalfspace {
		t.Helper()
		const lo, hi = -1., 5.
		k := []float64{lo, lo, hi, hi}
		p := func(x, y float64) geom.Vec3 { return geom.Vec3{X: x, Y: y, Z: h + slope*x} }
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{p(lo, lo), p(lo, hi), p(hi, lo), p(hi, hi)}, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return RationalGraphHalfspace{Points: [3]geom.Vec3{p(0, 0), p(1, 0), p(0, 1)}, KeepPositive: keep,
			Cap: s, CapDomain: geom.ParameterRectangle{UMin: lo, VMin: lo, UMax: hi, VMax: hi}}
	}
	cuts := [2]RationalGraphHalfspace{makeCut(.5, 0, true), makeCut(.3125, .125, false)}
	if out, e := stock.ClipIntersectingRationalGraphCaps(ctx, cuts, RationalGraphSeparatedBandOptions{}); e == nil || len(out.Solid.faces) != 0 {
		t.Fatal("default affine-projection proof cannot authenticate the curved trimming", e)
	}
	r, e := stock.ClipIntersectingRationalGraphCaps(ctx, cuts, RationalGraphSeparatedBandOptions{Sections: RationalGraphShellSectionOptions{ComposeSectionCurves: true}})
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-13./32) > 1e-10 || r.Solid.Topology().EulerCharacteristic != 2 || len(r.CapIntersections) != 2 {
		t.Fatal("quadratic shear has exact Jacobian 1 and preserves volume", m, r.Solid.Topology(), len(r.CapIntersections), e)
	}
	for _, seam := range r.CapIntersections {
		for _, q := range seam.Points {
			if q[0].Cmp(ratInset(1.5)) != 0 || q[2].Cmp(ratInset(.5)) != 0 || !new(big.Rat).Sub(q[1], ratInset(9./32)).IsInt() {
				t.Fatal("exact warped hole seam endpoint", q)
			}
		}
	}
	for _, faces := range r.CapFaces {
		genuine := false
		for _, id := range faces {
			for _, use := range r.Solid.faces[id].Outer {
				q, ok := use.Curve.(ExactQuadraticUV)
				if !ok {
					continue
				}
				h := q.HomogeneousControls()
				if slabUVSide([2]*big.Rat{h[0][0], h[0][1]}, [2]*big.Rat{h[1][0], h[1][1]}, [2]*big.Rat{h[2][0], h[2][1]}).Sign() != 0 {
					genuine = true
				}
			}
		}
		if !genuine {
			t.Fatal("genuine quadratic model curves must survive on both original caps")
		}
	}
	var out bytes.Buffer
	if e = WriteJSON(&out, r.Solid); e != nil {
		t.Fatal(e)
	}
	back, e := ReadJSONContext(ctx, &out, JSONLimits{})
	if e != nil || back.Topology().Edges != r.Solid.Topology().Edges {
		t.Fatal("curved inner cap topology reload", e)
	}
}
