package intersect

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// SplineQuarticPointSection is a complete zero-dimensional intersection of
// two finite original graph patches. It is not a transverse tube, trim result,
// or Boolean assembly. All state is private; rational accessors return copies.
// Owner 0 and 1 always denote the original arguments, not aligned charts.
type SplineQuarticPointSection struct {
	owners               [2]Patch
	domains              [2]geom.ParameterRectangle
	uv                   [2][2]*big.Rat
	point                [3]*big.Rat
	axes                 [2]int
	residual, side, work int
	complete             bool
}

func (s *SplineQuarticPointSection) Complete() bool { return s != nil && s.complete }

// Owner returns the original immutable supported surface and finite chart.
func (s *SplineQuarticPointSection) Owner(i int) (Patch, error) {
	if !s.Complete() || i < 0 || i > 1 {
		return Patch{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *SplineQuarticPointSection) Domain(i int) (geom.ParameterRectangle, error) {
	if !s.Complete() || i < 0 || i > 1 {
		return geom.ParameterRectangle{}, geom.ErrInvalid
	}
	return s.domains[i], nil
}
func (s *SplineQuarticPointSection) Parameters(i int) (r [2]*big.Rat, err error) {
	if !s.Complete() || i < 0 || i > 1 {
		return r, geom.ErrInvalid
	}
	for j := range r {
		r[j] = new(big.Rat).Set(s.uv[i][j])
	}
	return r, nil
}
func (s *SplineQuarticPointSection) Point() (r [3]*big.Rat, err error) {
	if !s.Complete() {
		return r, geom.ErrInvalid
	}
	for j := range r {
		r[j] = new(big.Rat).Set(s.point[j])
	}
	return r, nil
}

// Separation returns projection axes, residual axis, and the strict sign of
// owner0-owner1 away from the contact at equal projected model coordinates.
func (s *SplineQuarticPointSection) Separation() (axes [2]int, residual, side int, err error) {
	if !s.Complete() {
		return axes, 0, 0, geom.ErrInvalid
	}
	return s.axes, s.residual, s.side, nil
}
func (s *SplineQuarticPointSection) Order() int {
	if !s.Complete() {
		return 0
	}
	return 4
}

// ExactOperations counts cumulative projected-contact proof work. Original
// support recovery is separately bounded by the supplied SurfacePatchOptions.
func (s *SplineQuarticPointSection) ExactOperations() int {
	if !s.Complete() {
		return 0
	}
	return s.work
}

// CertifySplinePairQuarticPointSection re-proves an explicit rational witness
// in A's original UV and positive-side Gram cross entry on the actual finite
// patches. It supports matching projected rectangles with independent signed
// axis/scaled charts, including odd-mixed quartics. Unsupported, singular,
// clipped-away or boundary witnesses return nil; no partial section is exposed.
// Surface recovery uses p at each existing bounded extraction/refinement stage;
// o bounds the complete projected proof, with no per-owner reset of that quota.
func CertifySplinePairQuarticPointSection(ctx context.Context, a, b Patch, axes [2]int, uv [2]*big.Rat, cross *big.Rat, p geom.SurfacePatchOptions, o geom.GraphPointContactOptions) (*SplineQuarticPointSection, error) {
	if ctx == nil || !a.valid() || !b.valid() {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	pa, e := exactSplinePatch(a, p)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	pb, e := exactSplinePatch(b, p)
	if e != nil {
		return nil, e
	}
	proof, e := geom.CertifyProjectedQuarticGramGraphPointContact(ctx, pa, pb, axes, uv, cross, o)
	if e != nil {
		return nil, e
	}
	if !proof.Proven {
		return nil, ErrUnresolved
	}
	return &SplineQuarticPointSection{owners: [2]Patch{a, b}, domains: [2]geom.ParameterRectangle{proof.Domain, proof.TargetDomain}, uv: [2][2]*big.Rat{proof.Parameters, proof.TargetParameters}, point: proof.Point, axes: axes, residual: proof.ResidualAxis, side: proof.Side, work: proof.ExactOperations, complete: true}, nil
}
