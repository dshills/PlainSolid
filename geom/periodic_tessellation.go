package geom

import (
	"context"
	"math/big"
)

// PeriodicTessellationOptions bounds the extraction and tessellation of an
// exact periodic window. Zero MaxSpans allows 256 knot pieces. Zero MaxPoles
// allows 4096 controls, including the duplicated auxiliary control row.
type PeriodicTessellationOptions struct {
	CertifiedTessellationOptions
	MaxSpans, MaxPoles int
}

// CertifiedPeriodicCurveSegment corresponds to a closed rational interval
// inside a single original knot span. ErrorUpper bounds the distance to the
// rounded chord under linear interpolation in that physical interval.
type CertifiedPeriodicCurveSegment struct {
	Domain     ExactParameterInterval
	Cycle      *big.Int
	Span       int
	Start, End Vec3
	ErrorUpper float64
}

// CertifiedPeriodicCurveTessellation retains the certified prefix on search
// exhaustion. Complete indicates coverage of the entire requested window.
type CertifiedPeriodicCurveTessellation struct {
	Segments        []CertifiedPeriodicCurveSegment
	Complete        bool
	ExactOperations int
}

// TessellateCertifiedWindow returns deflection-bounded chords for the original
// periodic curve. Every interior knot and cyclic seam is retained, including
// knots at rational parameters that have no exact binary64 representation.
func (c *PeriodicBSpline) TessellateCertifiedWindow(ctx context.Context, start, end *big.Rat, options PeriodicTessellationOptions) (result CertifiedPeriodicCurveTessellation, err error) {
	o := options.CertifiedTessellationOptions
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if !finite(o.Deflection) || o.Deflection < 0 || o.MaxSegments < 0 || o.MaxDepth < 0 || o.MaxDepth > 128 {
		return result, ErrInvalid
	}
	limits, err := patchOptions(SurfacePatchOptions{
		MaxPoles: options.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if err != nil {
		return result, err
	}
	curve, spans, work, err := c.clampedPeriodicWindow(ctx, start, end, ExactBezierSpanOptions{
		SurfacePatchOptions: limits, MaxSpans: options.MaxSpans,
	})
	result.ExactOperations = work
	if err != nil {
		return result, err
	}
	o.MaxOperations -= work
	if o.MaxOperations < 1 {
		return result, ErrConvergence
	}
	mesh, searchError := tessellateExactCurve(ctx, curve.patch, o, func(segment CertifiedCurveSegment, budget *polynomialBudget) error {
		index := int(segment.Domain.Start)
		piece := spans[index]
		a := periodicWindowParameter(piece, index, segment.Domain.Start)
		z := periodicWindowParameter(piece, index, segment.Domain.End)
		for _, endpoint := range []*big.Rat{a, z} {
			if err := budget.check(endpoint); err != nil {
				return err
			}
		}
		result.Segments = append(result.Segments, CertifiedPeriodicCurveSegment{
			Domain: ExactParameterInterval{Lower: a, Upper: z}, Cycle: new(big.Int).Set(piece.Cycle), Span: piece.Span,
			Start: segment.Start, End: segment.End, ErrorUpper: segment.ErrorUpper,
		})
		return nil
	})
	result.ExactOperations += mesh.ExactOperations
	result.Complete = mesh.Complete
	return result, searchError
}
