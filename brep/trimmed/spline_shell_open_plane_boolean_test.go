package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/intersect"
)

func TestOpenPlaneSplineShellBoolean(t *testing.T) {
	ctx := context.Background()
	al, ah := [3]float64{0, 0, 0}, [3]float64{1, 1, 1}
	bl, bh := [3]float64{.5, .5, -.5}, [3]float64{1.5, 1.5, .5}
	a, af := makeOpenSplineBox(t, al, ah, .125)
	b, bf := makeOpenSplineBox(t, bl, bh, .125)
	var pairs [][2]int
	for i, x := range af {
		for j, y := range bf {
			if x.axis != y.axis && x.fixed > bl[x.axis] && x.fixed < bh[x.axis] && y.fixed > al[y.axis] && y.fixed < ah[y.axis] {
				pairs = append(pairs, [2]int{i, j})
			}
		}
	}
	for _, tc := range []struct {
		op     parametric.BooleanOp
		volume float64
	}{{parametric.Union, 1.875}, {parametric.Intersection, .125}, {parametric.Difference, .875}} {
		s, history, e := CombineOpenPlaneSplineShells(ctx, a, b, tc.op, pairs, intersect.SplinePlanePairDiscoveryOptions{}, SplineShellOpenOptions{})
		if e != nil {
			t.Fatal(tc.op, e)
		}
		if s.Topology().EulerCharacteristic != 2 || len(history.Pairs) != 6 || len(history.SectionEdges) == 0 {
			t.Fatal("native provenance", s.Topology(), history)
		}
		if tc.op == parametric.Intersection {
			var buffer bytes.Buffer
			if e := WriteJSON(&buffer, s); e != nil {
				t.Fatal(e)
			}
			restored, e := ReadJSONContext(ctx, &buffer, JSONLimits{})
			if e != nil || !reflect.DeepEqual(restored.Topology(), s.Topology()) {
				t.Fatal("native reload", e)
			}
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-tc.volume) > 1e-7 {
			t.Fatal(p, e)
		}
	}
	bad, _, e := CombineOpenPlaneSplineShells(ctx, a, b, parametric.Union, pairs[:len(pairs)-1], intersect.SplinePlanePairDiscoveryOptions{}, SplineShellOpenOptions{})
	if e == nil || len(bad.Faces()) != 0 {
		t.Fatal("omitted contact accepted", e)
	}
	duplicate := append(append([][2]int(nil), pairs...), pairs[0])
	bad, _, e = CombineOpenPlaneSplineShells(ctx, a, b, parametric.Union, duplicate, intersect.SplinePlanePairDiscoveryOptions{}, SplineShellOpenOptions{})
	if e == nil || len(bad.Faces()) != 0 {
		t.Fatal("duplicate pair", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, e = CombineOpenPlaneSplineShells(canceled, a, b, parametric.Union, pairs, intersect.SplinePlanePairDiscoveryOptions{}, SplineShellOpenOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
