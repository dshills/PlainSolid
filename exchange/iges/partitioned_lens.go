package iges

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// LensSeamPiece relates one source edge to an exact canonical quarter interval
// and both owners' original knot cells. IDs are source records, not native edges.
type LensSeamPiece struct {
	Quarter           int
	Start, End        *big.Rat
	Cells             [2]geom.ParameterRectangle
	ModelCurveID      int
	ParameterCurveIDs [2]int
}

// PartitionedRationalSplineLens retains 4–32 closed conjugate-axis ellipse seam pieces
// and independent raw model/upper-UV/lower-UV bases for each source edge.
type PartitionedRationalSplineLens struct {
	Body   SplineLensSolid
	Pieces []LensSeamPiece
	Curves [][3]RationalLensCurve
}
type igesLensPiece struct {
	LensSeamPiece
	UV, Model *geom.ExactSplineCurve
}

func igesLensPlan(m SplineLensSolid) ([]igesLensPiece, error) {
	uv, e := lensAffineUV(m.Center, m.Radii, m.Shear, m.Tilt)
	if e != nil {
		return nil, e
	}
	var patches [2]*geom.SurfacePatch
	for i, s := range []*geom.BSplineSurface{m.Upper, m.Lower} {
		patches[i], e = geom.NewSurfacePatch(s, curvedOvalOptions())
		if e != nil {
			return nil, e
		}
	}
	var out []igesLensPiece
	for q, c := range uv {
		x, e := geom.NewExactSplineCurve(c, curvedOvalOptions())
		if e != nil {
			return nil, e
		}
		cuts := map[string]*big.Rat{"0": new(big.Rat), "1": big.NewRat(1, 1)}
		for _, p := range patches {
			v, e := p.ComposeParameterCurvePieces(context.Background(), x, geom.SurfaceCurveCompositionOptions{SurfacePatchOptions: curvedOvalOptions(), MaxPieces: 16, MaxRootSubdivisions: 512, MaxHullDepth: 8})
			if e != nil {
				return nil, e
			}
			if !v.Complete || !v.Continuous {
				return nil, geom.ErrConvergence
			}
			for _, s := range v.Pieces {
				cuts[s.Start.RatString()] = s.Start
				cuts[s.End.RatString()] = s.End
			}
		}
		var ordered []*big.Rat
		for _, v := range cuts {
			ordered = append(ordered, v)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].Cmp(ordered[j]) < 0 })
		for i := 0; i+1 < len(ordered); i++ {
			segment, e := x.RationalSegment(context.Background(), ordered[i], ordered[i+1], geom.TubeAffineLineOptions{MaxPoles: 256, MaxOperations: 1000000, MaxCoefficientBits: 8192})
			if e != nil {
				return nil, e
			}
			part := igesLensPiece{LensSeamPiece: LensSeamPiece{Quarter: q, Start: new(big.Rat).Set(ordered[i]), End: new(big.Rat).Set(ordered[i+1])}, UV: segment}
			for owner, p := range patches {
				v, e := p.ComposeParameterCurvePieces(context.Background(), segment, geom.SurfaceCurveCompositionOptions{SurfacePatchOptions: curvedOvalOptions(), MaxPieces: 1, MaxRootSubdivisions: 512, MaxHullDepth: 8})
				if e != nil {
					return nil, e
				}
				if !v.Complete || !v.Continuous || len(v.Pieces) != 1 || v.Pieces[0].Start.Sign() != 0 || v.Pieces[0].End.Cmp(big.NewRat(1, 1)) != 0 {
					return nil, geom.ErrConvergence
				}
				part.Cells[owner] = v.Pieces[0].Cell
				if owner == 0 {
					part.Model = v.Pieces[0].Curve
				} else {
					eq, e := geom.CompareExactCurvesContext(context.Background(), part.Model, v.Pieces[0].Curve, geom.ExactCurveComparisonOptions{MaxOperations: 1000000, MaxCoefficientBits: 8192})
					if e != nil || !eq.Complete || !eq.Equal {
						return nil, geom.ErrInvalid
					}
				}
			}
			out = append(out, part)
			if len(out) > 32 {
				return nil, geom.ErrConvergence
			}
		}
	}
	return out, nil
}
