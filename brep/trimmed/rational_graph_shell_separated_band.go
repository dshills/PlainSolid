package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// RationalGraphHalfspace identifies the kept side of the oriented plane and
// the original affine spline chart in which to build its cap. KeepPositive
// selects the side pointed to by the normal (Points[1]-Points[0]) ×
// (Points[2]-Points[0]); false selects the opposite side.
type RationalGraphHalfspace struct {
	Points       [3]geom.Vec3
	KeepPositive bool
	Cap          geom.Surface
	CapDomain    geom.ParameterRectangle
}

// RationalGraphSeparatedBandOptions limits the two independent sections and
// their exact cap trim arrangements.
type RationalGraphSeparatedBandOptions = RationalGraphParallelBandOptions

// RationalGraphSeparatedBand retains independent exact sections on the
// original shell and identifies the surviving original and generated uses.
type RationalGraphSeparatedBand struct {
	Solid                    Solid
	Sections                 [2]RationalGraphShellSection
	SourceFaces, SourceEdges [][]int
	CapFaces, CapEdges       [2][]int
}

// ClipSeparatedRationalGraphSplineShell intersects a rectangular spline shell
// with two halfspaces. The planes may be nonparallel, but every component of
// each exact original section must be strictly inside the other kept side.
// The plane intersection then cannot meet material anywhere on either cap.
// A curved face crossed by both planes must have two independent continuous
// graph branches along the same original free axis, with matching knot spans.
// Touching, crossing, or unproved cap configurations reject atomically.
func (s Solid) ClipSeparatedRationalGraphSplineShell(ctx context.Context, cuts [2]RationalGraphHalfspace, o RationalGraphSeparatedBandOptions) (RationalGraphSeparatedBand, error) {
	var zero RationalGraphSeparatedBand
	so := o.Sections
	if so.MaxPieces == 0 {
		so.MaxPieces = 128
	}
	if so.Regions == nil {
		so.Regions = &RationalGraphShellRegionOptions{}
	}
	if ctx == nil || so.MaxPieces < 1 || so.MaxPieces > 1024 || so.Spans.MaxSpans < 0 || so.Spans.MaxSpans > 128 {
		return zero, geom.ErrInvalid
	}
	var planes [2]polynomialPlaneEquation
	var keep [2]int
	var caps [2]geom.Surface
	var domains [2]geom.ParameterRectangle
	for i, cut := range cuts {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if cut.Cap == nil || !cut.CapDomain.Valid() {
			return zero, geom.ErrInvalid
		}
		for _, x := range cut.Points {
			if !x.IsFinite() {
				return zero, geom.ErrInvalid
			}
		}
		p, e := polynomialPlaneFromPoints(cut.Points)
		if e != nil {
			return zero, e
		}
		planes[i], keep[i], caps[i], domains[i] = p, -1, cut.Cap, cut.CapDomain
		if cut.KeepPositive {
			keep[i] = 1
		}
	}
	var faces []splineEmbeddingFace
	proof, e := s.validateSplineRectangleEmbedding(ctx, so.Spans.Faces, &faces)
	if e != nil {
		return zero, e
	}
	if !proof.Complete {
		return zero, geom.ErrConvergence
	}
	var sections [2]RationalGraphShellSection
	var signs [2][]int
	for i, cut := range cuts {
		sections[i], signs[i], e = s.discoverRationalGraphShellSection(ctx, cut.Points, planes[i], faces, so)
		if e != nil {
			return zero, fmt.Errorf("trimmed: section %d: %w", i, e)
		}
		if e = sections[i].Regions.proveStrictCapHalfspace(ctx, planes[1-i], keep[1-i]); e != nil {
			return zero, fmt.Errorf("trimmed: separation of cap %d: %w", i, e)
		}
	}
	result, e := s.assembleRationalGraphBand(ctx, sections, faces, planes, signs, keep, caps, domains, o)
	if e != nil {
		return zero, e
	}
	return RationalGraphSeparatedBand{result.Solid, sections, result.SourceFaces, result.SourceEdges, result.CapFaces, result.CapEdges}, nil
}

// Substituting the source plane equation into the other plane's oriented
// residual gives a homogeneous linear form over the original projected
// section controls. Positive weights and equally strict residual signs put
// every boundary curve, and so every bounded enclosed region, in the open
// kept halfspace. This test is deliberately conservative.
func (r *RationalGraphShellRegions) proveStrictCapHalfspace(ctx context.Context, other polynomialPlaneEquation, keep int) error {
	if r == nil || r.certificate == nil || len(r.material) == 0 {
		return geom.ErrConvergence
	}
	axes := orientedRationalGraphProjection(r.plane)
	outer := other.normal[r.plane.dropped]
	form := [3]*big.Rat{nil, nil, new(big.Rat).Sub(new(big.Rat).Mul(outer, r.plane.offset), other.offset)}
	for i, a := range axes {
		form[i] = new(big.Rat).Sub(other.normal[a], new(big.Rat).Mul(outer, r.plane.normal[a]))
	}
	valid := func(x *big.Rat) bool {
		return x.Num().BitLen() <= r.maxBits && x.Denom().BitLen() <= r.maxBits
	}
	for _, a := range form {
		if !valid(a) {
			return geom.ErrConvergence
		}
	}
	for _, wire := range r.wires {
		for _, piece := range wire {
			for _, h := range piece.control {
				if e := ctx.Err(); e != nil {
					return e
				}
				v := new(big.Rat)
				for k, c := range form {
					u := new(big.Rat).Mul(c, h[k])
					if !valid(u) {
						return geom.ErrConvergence
					}
					v.Add(v, u)
					if !valid(v) {
						return geom.ErrConvergence
					}
				}
				if h[2].Sign() <= 0 || v.Sign()*other.orientation != keep {
					return geom.ErrConvergence
				}
			}
		}
	}
	return nil
}
