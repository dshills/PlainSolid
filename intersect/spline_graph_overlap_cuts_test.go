package intersect

import (
	"math"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestRetainedGraphCuts(t *testing.T) {
	p := Patch{Min: geom.Vec2{X: .25, Y: .25}, Max: geom.Vec2{X: .75, Y: .75}}
	original := []float64{1, .5, 0, .25, .75, .5, .375}
	saved := append([]float64(nil), original...)
	o, e := retainedGraphCuts(SplinePlanePairDiscoveryOptions{UCuts: original}, p)
	if e != nil || !reflect.DeepEqual(o.UCuts, []float64{.375, .5}) || !reflect.DeepEqual(original, saved) {
		t.Fatal(o, e)
	}
	for _, cuts := range [][]float64{{math.NaN()}, {math.Inf(1)}, make([]float64, 65)} {
		if _, e = retainedGraphCuts(SplinePlanePairDiscoveryOptions{UCuts: cuts}, p); e == nil {
			t.Fatal("invalid discarded guidance accepted")
		}
	}
}
