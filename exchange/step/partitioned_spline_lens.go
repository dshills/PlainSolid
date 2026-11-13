package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// LensSeamPiece maps one source edge to an exact interval of a canonical conic
// quarter. Cells retain the original upper/lower support knot lineage.
type LensSeamPiece struct {
	Quarter      int
	Start, End   *big.Rat
	SourceEdgeID int64
	Cells        [2]geom.ParameterRectangle
}

// PartitionedSplineLens retains explicit STEP seam-piece provenance. Body.Solid
// uses the proved canonical four-quarter native topology; Pieces describes the
// potentially finer source topology instead of conflating the two.
type PartitionedSplineLens struct {
	Body   SplineLensBody
	Pieces []LensSeamPiece
}

// ReadPartitionedSplineLensBody validates one closed lens whose source edges
// are precisely the union of both supports' certified rational knot cuts.
// Unresolved algebraic cuts and differently parameterized source pieces fail.
func ReadPartitionedSplineLensBody(r io.Reader, l Limits, o CurvedOvalOptions) (PartitionedSplineLens, error) {
	m, e := readSplineLensBody(r, l, o, true)
	if e != nil {
		return PartitionedSplineLens{}, e
	}
	return PartitionedSplineLens{Body: m, Pieces: m.sourcePieces}, nil
}

type lensPieceGeometry struct {
	LensSeamPiece
	UV, Model                 *geom.ExactSplineCurve
	UVSpline, ModelSpline     *geom.BSpline
	UVRational, ModelRational RationalLensCurve
}

func lensSegmentOptions(o CurvedOvalOptions) geom.TubeAffineLineOptions {
	return geom.TubeAffineLineOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations / 16, MaxCoefficientBits: o.MaxCoefficientBits}
}
func lensPiecePlan(m SplineLensBody, o CurvedOvalOptions) ([]lensPieceGeometry, error) {
	o, e := o.defaults()
	if e != nil {
		return nil, e
	}
	if o.MaxOperations < 64 {
		return nil, geom.ErrConvergence
	}
	uv, e := curvedEllipseUV(QuadraticOval{Center: m.Center, Radii: m.Radii, QuarterWeight: .5}, m.Shear)
	if e != nil {
		return nil, e
	}
	var patches [2]*geom.SurfacePatch
	for i, s := range []*geom.BSplineSurface{m.Upper, m.Lower} {
		patches[i], e = geom.NewSurfacePatch(s, o.patch())
		if e != nil {
			return nil, e
		}
	}
	result := []lensPieceGeometry{}
	for quarter, c := range uv {
		exact, e := geom.NewExactSplineCurve(c, o.patch())
		if e != nil {
			return nil, e
		}
		cuts := map[string]*big.Rat{"0": new(big.Rat), "1": big.NewRat(1, 1)}
		for _, patch := range patches {
			pieces, e := patch.ComposeParameterCurvePieces(context.Background(), exact, geom.SurfaceCurveCompositionOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations / 16, MaxCoefficientBits: o.MaxCoefficientBits}, MaxPieces: 32, MaxRootSubdivisions: 512, MaxHullDepth: 8})
			if e != nil {
				return nil, e
			}
			if !pieces.Complete || !pieces.Continuous {
				return nil, geom.ErrConvergence
			}
			for _, p := range pieces.Pieces {
				cuts[p.Start.RatString()] = new(big.Rat).Set(p.Start)
				cuts[p.End.RatString()] = new(big.Rat).Set(p.End)
			}
		}
		ordered := make([]*big.Rat, 0, len(cuts))
		for _, v := range cuts {
			ordered = append(ordered, v)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].Cmp(ordered[j]) < 0 })
		if len(ordered) > 33 {
			return nil, geom.ErrConvergence
		}
		phase := o
		phase.MaxOperations = o.MaxOperations / (16 + 16*len(ordered))
		if phase.MaxOperations < 10 {
			return nil, geom.ErrConvergence
		}
		for i := 0; i+1 < len(ordered); i++ {
			segment, e := exact.RationalSegment(context.Background(), ordered[i], ordered[i+1], lensSegmentOptions(o))
			if e != nil {
				return nil, e
			}
			entry := lensPieceGeometry{LensSeamPiece: LensSeamPiece{Quarter: quarter, Start: ordered[i], End: ordered[i+1]}, UV: segment}
			var curves [2]*geom.ExactSplineCurve
			for owner, patch := range patches {
				pieces, e := patch.ComposeParameterCurvePieces(context.Background(), segment, geom.SurfaceCurveCompositionOptions{SurfacePatchOptions: phase.patch(), MaxPieces: 1, MaxRootSubdivisions: 512, MaxHullDepth: 8})
				if e != nil {
					return nil, e
				}
				if !pieces.Complete || !pieces.Continuous || len(pieces.Pieces) != 1 || pieces.Pieces[0].Start.Sign() != 0 || pieces.Pieces[0].End.Cmp(big.NewRat(1, 1)) != 0 {
					return nil, geom.ErrConvergence
				}
				curves[owner] = pieces.Pieces[0].Curve
				entry.Cells[owner] = pieces.Pieces[0].Cell
			}
			same, e := geom.CompareExactCurvesContext(context.Background(), curves[0], curves[1], geom.ExactCurveComparisonOptions{MaxOperations: phase.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return nil, e
			}
			if !same.Complete || !same.Equal {
				return nil, geom.ErrConvergence
			}
			entry.Model = curves[0]
			result = append(result, entry)
		}
	}
	if len(result) > 128 {
		return nil, geom.ErrConvergence
	}
	return result, nil
}
