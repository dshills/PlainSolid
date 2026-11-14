package geom

import (
	"context"
	"math/big"
)

// LiftedWrappedRectangleArrangement binds exact quotient chart ownership to a
// single global injectivity proof on the original A lifted convex hull. Unlike
// EmbeddedWrappedRectangleArrangement, its support may cross true seams and
// is covered by multiple original fundamental restrictions, never a fabricated
// single source chart. This proves regions only, not a surrounding shell.
type LiftedWrappedRectangleArrangement struct {
	Arrangement     WrappedRectangleArrangement
	Embedding       WrappedLiftedRectangleInjectivity
	ExactOperations int
}

// CertifyLiftedRectangleArrangement is the explicit seam-aware alternative to
// CertifyEmbeddedRectangleArrangement. Both retain identical exact owner UVs.
// One cumulative arithmetic quota covers arrangement and global embedding;
// unsupported cuts, multiple quotient occurrences, or unresolved injectivity
// return no arrangement. The original non-seam guard is unchanged.
func (r *WrappedSurfaceChartRelation) CertifyLiftedRectangleArrangement(ctx context.Context, a, b [2][2]*big.Rat, o WrappedRectangleEmbeddingOptions) (LiftedWrappedRectangleArrangement, error) {
	var zero LiftedWrappedRectangleArrangement
	arrangement, e := r.ArrangeRectanglesRational(ctx, a, b, o.Arrangement)
	if e != nil {
		return zero, e
	}
	maxOps := o.Arrangement.MaxOperations
	if maxOps == 0 {
		maxOps = 1000000
	}
	remaining := maxOps - arrangement.ExactOperations
	if remaining < 1 {
		return zero, ErrConvergence
	}
	var hull [2][2]*big.Rat
	for _, cell := range arrangement.Cells {
		for axis := 0; axis < 2; axis++ {
			for end := 0; end < 2; end++ {
				q := cell.A[axis][end]
				if hull[axis][end] == nil || (end == 0 && q.Cmp(hull[axis][end]) < 0) || (end == 1 && q.Cmp(hull[axis][end]) > 0) {
					hull[axis][end] = new(big.Rat).Set(q)
				}
			}
		}
	}
	var values [2][2]float64
	for axis := 0; axis < 2; axis++ {
		for end := 0; end < 2; end++ {
			values[axis][end], e = exactRepresentableFloat(hull[axis][end])
			if e != nil {
				return zero, e
			}
		}
	}
	domain := ParameterRectangle{UMin: values[0][0], UMax: values[0][1], VMin: values[1][0], VMax: values[1][1]}
	embedding, e := r.source.CertifyLiftedRectangleInjectivity(ctx, domain, SurfaceInjectivityOptions{ProjectionAxes: o.ProjectionAxes, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.Arrangement.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	return LiftedWrappedRectangleArrangement{Arrangement: arrangement, Embedding: embedding, ExactOperations: arrangement.ExactOperations + embedding.ExactOperations}, nil
}
