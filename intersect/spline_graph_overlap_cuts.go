package intersect

import (
	"math"
	"sort"

	"plainsolid/geom"
)

// retainedGraphCuts validates the full original lists before discarding any
// cut outside the exact retained rectangle. Cuts only guide subdivision, so
// deletion cannot supply coverage evidence; coverage remains independently
// mandatory. Copying avoids mutating caller-owned guidance.
func retainedGraphCuts(o SplinePlanePairDiscoveryOptions, p Patch) (SplinePlanePairDiscoveryOptions, error) {
	if len(o.UCuts)+len(o.VCuts) > 64 {
		return o, geom.ErrInvalid
	}
	lists := [2][]float64{o.UCuts, o.VCuts}
	lo := [2]float64{p.Min.X, p.Min.Y}
	hi := [2]float64{p.Max.X, p.Max.Y}
	for i, list := range lists {
		kept := make([]float64, 0, len(list))
		for _, cut := range list {
			if math.IsNaN(cut) || math.IsInf(cut, 0) {
				return o, geom.ErrInvalid
			}
			if cut > lo[i] && cut < hi[i] {
				kept = append(kept, cut)
			}
		}
		sort.Float64s(kept)
		n := 0
		for _, cut := range kept {
			if n == 0 || kept[n-1] != cut {
				kept[n] = cut
				n++
			}
		}
		lists[i] = kept[:n]
	}
	o.UCuts, o.VCuts = lists[0], lists[1]
	return o, nil
}
