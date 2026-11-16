package geom

import (
	"context"
	"math/big"
	"slices"
)

// NormalOffsetSweepJoinedLineOptions bounds the entire joined proof.
// ProfileParameters are optional exact transverse stations in original
// chart order for the curved ruling path.
type NormalOffsetSweepJoinedLineOptions struct {
	Patch             SurfacePatchOptions
	ProfileParameters [3]*big.Rat
}

// NormalOffsetSweepJoinedLineWitness retains an original owner, its raw
// sweep station, and the retained analytic faces incident to the event.
// AlgebraicStation supplies the original root and directed bounds when
// a transverse coordinate cannot be given as an exact rational.
type NormalOffsetSweepJoinedLineWitness struct {
	Owner            int
	Station          [3]*big.Rat
	Faces            []NormalOffsetSweepSide
	AlgebraicStation *NormalOffsetSweepLineAlgebraicStation `json:",omitempty"`
}

// NormalOffsetSweepJoinedLineEvent groups all retained original boundary
// witnesses at one exact model point and line parameter. Events can occur
// within a material interval where the line follows the outer boundary
// across an original owner transition.
type NormalOffsetSweepJoinedLineEvent struct {
	Parameter *big.Rat
	Point     [3]*big.Rat
	Witnesses []NormalOffsetSweepJoinedLineWitness
}

// NormalOffsetSweepJoinedLineBoundarySpan is an original owner's
// positive-length line span on its retained outer faces. Coincident
// spans from distinct original owners each keep their source stations.
type NormalOffsetSweepJoinedLineBoundarySpan struct {
	Owner int
	Span  NormalOffsetSweepLineSpan
}

// NormalOffsetSweepJoinedLineIntersection records the complete exact union
// of the original material line intervals. Material intervals are sorted
// and disjoint, with touching intervals merged. BoundaryEvents and
// BoundarySpans use only retained original outer faces. Incomplete proofs
// publish no intervals or boundary data.
type NormalOffsetSweepJoinedLineIntersection struct {
	Material        []ExactParameterInterval
	BoundaryEvents  []NormalOffsetSweepJoinedLineEvent
	BoundarySpans   []NormalOffsetSweepJoinedLineBoundarySpan
	Complete        bool
	ExactOperations int
}

// IntersectRationalLine proves each original owner's whole-line
// intersection, then certifies the material and outer boundary union.
func (p *VerifiedNormalOffsetSweepPair) IntersectRationalLine(ctx context.Context, origin, direction [3]*big.Rat,
	o NormalOffsetSweepJoinedLineOptions) (NormalOffsetSweepJoinedLineIntersection, error) {
	if p == nil || p.coordinate == nil {
		return NormalOffsetSweepJoinedLineIntersection{}, ErrInvalid
	}
	return intersectJoinedSweepLine(ctx, origin, direction, o, p.sweeps[:], p.Boundary())
}

// IntersectRationalLine composes the three complete original line
// intersections, counting each source chart once.
func (c *VerifiedNormalOffsetSweepChain) IntersectRationalLine(ctx context.Context, origin, direction [3]*big.Rat,
	o NormalOffsetSweepJoinedLineOptions) (NormalOffsetSweepJoinedLineIntersection, error) {
	if c == nil || c.pairs[0] == nil {
		return NormalOffsetSweepJoinedLineIntersection{}, ErrInvalid
	}
	return intersectJoinedSweepLine(ctx, origin, direction, o, c.sweeps[:], c.Boundary())
}

type sweepOwnedLineSpan struct {
	owner int
	span  *NormalOffsetSweepLineSpan
}

func intersectJoinedSweepLine(ctx context.Context, origin, direction [3]*big.Rat, o NormalOffsetSweepJoinedLineOptions,
	sweeps []*VerifiedNormalOffsetSweep, faces []VerifiedNormalOffsetSweepBoundaryFace) (result NormalOffsetSweepJoinedLineIntersection, err error) {
	if ctx == nil {
		return result, ErrInvalid
	}
	for _, seed := range o.ProfileParameters[len(sweeps):] {
		if seed != nil {
			return result, ErrInvalid
		}
	}
	limits, err := patchOptions(o.Patch)
	if err != nil {
		return result, err
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { result.ExactOperations = budget.operations }()
	var spans []sweepOwnedLineSpan
	for owner, sweep := range sweeps {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		remaining := limits
		remaining.MaxOperations -= budget.operations
		if remaining.MaxOperations <= 0 {
			return result, ErrConvergence
		}
		part, err := sweep.IntersectRationalLine(ctx, origin, direction, NormalOffsetSweepLineOptions{
			Patch: remaining, ProfileParameter: o.ProfileParameters[owner],
		})
		budget.operations += part.ExactOperations
		if err != nil {
			return result, err
		}
		if !part.Complete {
			return result, ErrConvergence
		}
		if part.Span != nil {
			spans = append(spans, sweepOwnedLineSpan{owner, part.Span})
		}
	}
	joined, err := assembleJoinedSweepLine(ctx, spans, faces, budget)
	if err != nil {
		return result, err
	}
	return joined, nil
}

type sweepJoinedEndpointCandidate struct {
	owner     int
	endpoint  NormalOffsetSweepLineEndpoint
	algebraic *NormalOffsetSweepLineAlgebraicStation
}

func assembleJoinedSweepLine(ctx context.Context, spans []sweepOwnedLineSpan, faces []VerifiedNormalOffsetSweepBoundaryFace,
	b *polynomialBudget) (result NormalOffsetSweepJoinedLineIntersection, err error) {
	var outer [3][6]bool
	for _, face := range faces {
		outer[face.Owner][face.Side] = true
	}
	slices.SortFunc(spans, func(a, c sweepOwnedLineSpan) int {
		if order := a.span.Start.Parameter.Cmp(c.span.Start.Parameter); order != 0 {
			return order
		}
		return a.owner - c.owner
	})
	retained := func(owner int, raw []NormalOffsetSweepSide) (result []NormalOffsetSweepSide) {
		for _, side := range raw {
			if outer[owner][side] {
				result = append(result, side)
			}
		}
		return result
	}
	var endpoints []sweepJoinedEndpointCandidate
	for _, item := range spans {
		p := *item.span
		lo, hi := p.Start.Parameter, p.End.Parameter
		for _, q := range [2]*big.Rat{lo, hi} {
			if err := b.check(q); err != nil {
				return result, err
			}
		}
		if lo.Cmp(hi) > 0 {
			return result, ErrConvergence
		}
		i := len(result.Material) - 1
		if i < 0 || result.Material[i].Upper.Cmp(lo) < 0 {
			result.Material = append(result.Material, copySweepInterval(exactInterval{lo, hi}))
		} else if result.Material[i].Upper.Cmp(hi) < 0 {
			result.Material[i].Upper = new(big.Rat).Set(hi)
		}
		p.Start.Faces = retained(item.owner, p.Start.Faces)
		p.End.Faces = retained(item.owner, p.End.Faces)
		p.BoundaryFaces = retained(item.owner, p.BoundaryFaces)
		if len(p.Start.Faces) != 0 {
			endpoints = append(endpoints, sweepJoinedEndpointCandidate{item.owner, p.Start, p.AlgebraicStation})
		}
		if lo.Cmp(hi) < 0 {
			if len(p.End.Faces) != 0 {
				endpoints = append(endpoints, sweepJoinedEndpointCandidate{item.owner, p.End, p.AlgebraicStation})
			}
			if len(p.BoundaryFaces) != 0 {
				p.Start, p.End = copySweepJoinedLineEndpoint(p.Start), copySweepJoinedLineEndpoint(p.End)
				p.AlgebraicStation = copySweepJoinedAlgebraicStation(p.AlgebraicStation)
				result.BoundarySpans = append(result.BoundarySpans, NormalOffsetSweepJoinedLineBoundarySpan{Owner: item.owner, Span: p})
			}
		}
	}
	result.BoundaryEvents, err = groupSweepJoinedLineEvents(endpoints, b)
	if err != nil {
		return result, err
	}
	// In a sealed joined sweep only the retained outer boundary can
	// terminate material on the line. Internal seam-only events are absent.
	for _, span := range result.Material {
		for _, boundary := range [2]*big.Rat{span.Lower, span.Upper} {
			if !slices.ContainsFunc(result.BoundaryEvents, func(event NormalOffsetSweepJoinedLineEvent) bool {
				return event.Parameter.Cmp(boundary) == 0
			}) {
				return result, ErrConvergence
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	result.Complete = true
	return result, nil
}

func groupSweepJoinedLineEvents(candidates []sweepJoinedEndpointCandidate, b *polynomialBudget) ([]NormalOffsetSweepJoinedLineEvent, error) {
	slices.SortFunc(candidates, func(a, c sweepJoinedEndpointCandidate) int {
		if order := a.endpoint.Parameter.Cmp(c.endpoint.Parameter); order != 0 {
			return order
		}
		return a.owner - c.owner
	})
	var events []NormalOffsetSweepJoinedLineEvent
	for _, candidate := range candidates {
		end := candidate.endpoint
		i := len(events) - 1
		if i < 0 || events[i].Parameter.Cmp(end.Parameter) != 0 {
			if err := b.check(end.Parameter); err != nil {
				return nil, err
			}
			events = append(events, NormalOffsetSweepJoinedLineEvent{
				Parameter: new(big.Rat).Set(end.Parameter), Point: copySweepJoinedLineVector(end.Point),
			})
			i++
		} else {
			for axis, q := range end.Point {
				if events[i].Point[axis].Cmp(q) != 0 {
					return nil, ErrConvergence
				}
			}
		}
		event := &events[i]
		event.Witnesses = append(event.Witnesses, NormalOffsetSweepJoinedLineWitness{
			Owner: candidate.owner, Station: copySweepJoinedLineVector(end.Station), Faces: end.Faces,
			AlgebraicStation: copySweepJoinedAlgebraicStation(candidate.algebraic),
		})
	}
	return events, nil
}

func copySweepJoinedLineEndpoint(endpoint NormalOffsetSweepLineEndpoint) NormalOffsetSweepLineEndpoint {
	endpoint.Parameter = new(big.Rat).Set(endpoint.Parameter)
	endpoint.Point = copySweepJoinedLineVector(endpoint.Point)
	endpoint.Station = copySweepJoinedLineVector(endpoint.Station)
	endpoint.Faces = slices.Clone(endpoint.Faces)
	return endpoint
}

func copySweepJoinedLineVector(q [3]*big.Rat) [3]*big.Rat {
	for i, x := range q {
		if x != nil {
			q[i] = new(big.Rat).Set(x)
		}
	}
	return q
}

func copySweepJoinedAlgebraicStation(q *NormalOffsetSweepLineAlgebraicStation) *NormalOffsetSweepLineAlgebraicStation {
	if q == nil {
		return nil
	}
	result := *q
	result.OriginalProfile = copySweepInterval(exactInterval{q.OriginalProfile.Lower, q.OriginalProfile.Upper})
	result.ProfileParameter = copySweepInterval(exactInterval{q.ProfileParameter.Lower, q.ProfileParameter.Upper})
	result.SignedDisplacement = copySweepInterval(exactInterval{q.SignedDisplacement.Lower, q.SignedDisplacement.Upper})
	result.ProfileRoot.Coefficients = slices.Clone(q.ProfileRoot.Coefficients)
	return &result
}
