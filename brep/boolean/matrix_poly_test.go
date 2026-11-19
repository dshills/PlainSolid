package boolean

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/csg/poly"
)

// TestMatrixAgainstPoly compares, for every planar leaf whose stated
// outcome is a solid or the empty solid, the volume of the result with
// the volume of csg/poly.Combine on the same operands, within the bound β
// of V5 (PL2). C26 is tolerance-scale and is not compared.
func TestMatrixAgainstPoly(t *testing.T) {
	operations := [...]poly.Operation{Union: poly.Union, Common: poly.Intersection, Cut: poly.Difference}
	compared := 0
	for _, m := range matrix {
		if m.variant != "planar" || m.row == "C26" {
			continue
		}
		for i, c := range m.build(t) {
			for op := Union; op <= Cut; op++ {
				if c.want[op].fails != nil {
					continue
				}
				name := m.row + "/" + operationNames[op]
				reference, err := poly.Combine(context.Background(), c.planar[0], c.planar[1], operations[op], poly.Options{})
				if err != nil {
					t.Fatalf("%s operands %d: csg/poly returned %v", name, i, err)
				}
				want, err := planarVolume(reference)
				if err != nil {
					t.Fatalf("%s operands %d: %v", name, i, err)
				}
				r, err := Combine(context.Background(), c.a, c.b, op, matrixOptions)
				if err != nil {
					t.Fatalf("%s operands %d: %v", name, i, err)
				}
				if beta := leafBound(c); !(math.Abs(r.Volume-want) <= beta) {
					t.Errorf("%s operands %d: volume %.12g, csg/poly %.12g, bound %g", name, i, r.Volume, want, beta)
				}
				compared++
			}
		}
	}
	t.Logf("%d leaves compared with csg/poly", compared)
}

// planarVolume is the volume of a planar solid, zero for the empty one.
func planarVolume(s brep.Solid) (float64, error) {
	if len(s.Faces()) == 0 {
		return 0, nil
	}
	p, err := s.MassProperties()
	return p.Volume, err
}
