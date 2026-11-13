package geom

import (
	"math/big"
	"sort"
)

func validateSurfaceInsertion(degree, count int, knots []float64, t float64, insertions int) error {
	if !finite(t) || t <= knots[degree] || t >= knots[count] || insertions < 0 {
		return ErrDomain
	}
	first := sort.SearchFloat64s(knots, t)
	last := sort.Search(len(knots), func(i int) bool { return knots[i] > t })
	if insertions > degree+1-(last-first) {
		return ErrInvalid
	}
	return nil
}

// InsertUKnot returns an independent surface with unchanged rational geometry
// up to float64 refinement error. Insertion is strictly interior; multiplicity
// may not exceed degree+1. The global homogeneous weight scale is preserved
// across columns, avoiding independent-column normalization errors.
func (s *BSplineSurface) InsertUKnot(t float64, count int) (*BSplineSurface, error) {
	if s == nil || len(s.poles) == 0 {
		return nil, ErrInvalid
	}
	if e := validateSurfaceInsertion(s.degreeU, s.countU, s.knotsU, t, count); e != nil {
		return nil, e
	}
	if count == 0 {
		return NewBSplineSurface(s.degreeU, s.degreeV, s.countU, s.countV, s.poles, s.knotsU, s.knotsV, s.weights)
	}
	h := toHom(s.poles, s.weights)
	newCount := s.countU + count
	if newCount > int(^uint(0)>>1)/s.countV {
		return nil, ErrInvalid
	}
	refined := make([]homogeneous, newCount*s.countV)
	var knots []float64
	for v := 0; v < s.countV; v++ {
		column := make([]homogeneous, s.countU)
		for u := range column {
			column[u] = h[u*s.countV+v]
		}
		next, k := refineHomogeneous(column, s.knotsU, s.degreeU, t, count)
		knots = k
		for u, p := range next {
			refined[u*s.countV+v] = p
		}
	}
	return surfaceFromHom(s.degreeU, s.degreeV, newCount, s.countV, refined, knots, s.knotsV)
}

// InsertVKnot is the v-direction counterpart of InsertUKnot.
func (s *BSplineSurface) InsertVKnot(t float64, count int) (*BSplineSurface, error) {
	if s == nil || len(s.poles) == 0 {
		return nil, ErrInvalid
	}
	if e := validateSurfaceInsertion(s.degreeV, s.countV, s.knotsV, t, count); e != nil {
		return nil, e
	}
	if count == 0 {
		return NewBSplineSurface(s.degreeU, s.degreeV, s.countU, s.countV, s.poles, s.knotsU, s.knotsV, s.weights)
	}
	h := toHom(s.poles, s.weights)
	newCount := s.countV + count
	if s.countU > int(^uint(0)>>1)/newCount {
		return nil, ErrInvalid
	}
	refined := make([]homogeneous, s.countU*newCount)
	var knots []float64
	for u := 0; u < s.countU; u++ {
		next, k := refineHomogeneous(h[u*s.countV:(u+1)*s.countV], s.knotsV, s.degreeV, t, count)
		knots = k
		copy(refined[u*newCount:], next)
	}
	return surfaceFromHom(s.degreeU, s.degreeV, s.countU, newCount, refined, s.knotsU, knots)
}

// Transposed swaps the two spline parameters without changing the geometry.
// It exchanges knot vectors and transposes poles and weights exactly.
func (s *BSplineSurface) Transposed() (*BSplineSurface, error) {
	if s == nil || len(s.poles) == 0 {
		return nil, ErrInvalid
	}
	poles := make([]Vec3, len(s.poles))
	weights := make([]float64, len(s.weights))
	for i := range s.countU {
		for j := range s.countV {
			a, b := i*s.countV+j, j*s.countU+i
			poles[b], weights[b] = s.poles[a], s.weights[a]
		}
	}
	return NewBSplineSurface(s.degreeV, s.degreeU, s.countV, s.countU, poles, s.knotsV, s.knotsU, weights)
}

// ReversedU reverses the parameter on the clamped active U interval.
// Each reflected knot must be exactly representable. This avoids substituting
// a nearby knot vector that would change the original geometry.
func (s *BSplineSurface) ReversedU() (*BSplineSurface, error) {
	if s == nil || len(s.poles) == 0 {
		return nil, ErrInvalid
	}
	k, e := reverseSurfaceKnots(s.knotsU, s.degreeU, s.countU)
	if e != nil {
		return nil, e
	}
	poles := make([]Vec3, len(s.poles))
	weights := make([]float64, len(s.weights))
	for i := range s.countU {
		a, b := i*s.countV, (s.countU-1-i)*s.countV
		copy(poles[b:b+s.countV], s.poles[a:a+s.countV])
		copy(weights[b:b+s.countV], s.weights[a:a+s.countV])
	}
	return NewBSplineSurface(s.degreeU, s.degreeV, s.countU, s.countV, poles, k, s.knotsV, weights)
}

// ReversedV reverses the parameter on the clamped active V interval.
// Each reflected knot must be exactly representable.
func (s *BSplineSurface) ReversedV() (*BSplineSurface, error) {
	if s == nil || len(s.poles) == 0 {
		return nil, ErrInvalid
	}
	k, e := reverseSurfaceKnots(s.knotsV, s.degreeV, s.countV)
	if e != nil {
		return nil, e
	}
	poles := make([]Vec3, len(s.poles))
	weights := make([]float64, len(s.weights))
	for i := range s.countU {
		for j := range s.countV {
			a, b := i*s.countV+j, i*s.countV+(s.countV-1-j)
			poles[b], weights[b] = s.poles[a], s.weights[a]
		}
	}
	return NewBSplineSurface(s.degreeU, s.degreeV, s.countU, s.countV, poles, s.knotsU, k, weights)
}

func reverseSurfaceKnots(knots []float64, degree, count int) ([]float64, error) {
	if !projectionKnotsSupported(degree, count, knots) {
		return nil, ErrDomain
	}
	sum := new(big.Rat).Add(new(big.Rat).SetFloat64(knots[degree]), new(big.Rat).SetFloat64(knots[count]))
	r := make([]float64, len(knots))
	for i, x := range knots {
		v, e := exactRepresentableFloat(new(big.Rat).Sub(sum, new(big.Rat).SetFloat64(x)))
		if e != nil {
			return nil, ErrConvergence
		}
		r[len(r)-1-i] = v
	}
	return r, nil
}
