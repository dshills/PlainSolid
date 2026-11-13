package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AlgebraicTubeLens is the bounded material between two original affine-XY
// spline graphs over the disk enclosed by their one regular section. The
// boundary has two faces sharing one closed wire. Exact section edges retain
// their individual parameter units and both chart owners.
type AlgebraicTubeLens struct {
	section AlgebraicTrimmedPairSection
	faces   [2]AlgebraicTubeFacePartition
	graphs  [2]*geom.AffineXYGraph
	upper   int
	branch  int
}

// LocateRational classifies a point in exact model units. The closed rim
// and both graph faces are boundary; only a strict vertical interior is
// classified as material interior.
func (l AlgebraicTubeLens) LocateRational(ctx context.Context, p [3]*big.Rat,
	o geom.ExactCurvePlaneEventOptions) (parametric.Location, error) {
	if p[2] == nil {
		return parametric.Outside, geom.ErrInvalid
	}
	x, e := l.VerticalInterval(ctx, [2]*big.Rat{p[0], p[1]}, o)
	if e != nil {
		return parametric.Outside, e
	}
	return locateTubeVerticalInterval(p[2], x)
}

func locateTubeVerticalInterval(z *big.Rat, x AlgebraicTubeLensVerticalInterval) (parametric.Location, error) {
	if !x.Complete {
		return parametric.Outside, geom.ErrConvergence
	}
	if !x.Present {
		return parametric.Outside, nil
	}
	a, b := z.Cmp(x.Lower), z.Cmp(x.Upper)
	if a < 0 || b > 0 {
		return parametric.Outside, nil
	}
	if a == 0 || b == 0 {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}

type AlgebraicTubeLensOptions struct {
	Partition AlgebraicTubeFacePartitionOptions
	Graph     geom.SurfacePatchOptions
}

type AlgebraicTubeLensTopology struct {
	Vertices, Edges, Faces, EulerCharacteristic int
}

// Topology counts the section's exact junctions and subedges separately.
// Both graph faces have disk topology.
func (l AlgebraicTubeLens) Topology() AlgebraicTubeLensTopology {
	if l.graphs[0] == nil {
		return AlgebraicTubeLensTopology{}
	}
	n := len(l.faces[0].Cut)
	return AlgebraicTubeLensTopology{n, n, 2, 2}
}

type AlgebraicTubeLensFace struct {
	Support               *geom.BSplineSurface
	SourceChart           int
	Reversed, CutReversed bool
}

func (l AlgebraicTubeLens) Faces() [2]AlgebraicTubeLensFace {
	var out [2]AlgebraicTubeLensFace
	if l.graphs[0] == nil {
		return out
	}
	for i, f := range l.faces {
		out[i] = AlgebraicTubeLensFace{Support: f.Support, SourceChart: i,
			Reversed: (i != l.upper) == (l.graphs[i].Orientation() > 0), CutReversed: f.Fragments[0].CutReversed}
	}
	return out
}

func (l AlgebraicTubeLens) Boundary() []AlgebraicTubeFaceCutPiece {
	return append([]AlgebraicTubeFaceCutPiece(nil), l.faces[0].Cut...)
}

// SealAlgebraicTubeLens forms a bounded two-face solid without fitting a
// rational approximation of the section. Both charts must be affine-XY
// graphs and the independently covered trimmed section must be exactly one
// closed run strictly inside both convex straight-trimmed disk faces. The affine projections map
// the common Jordan loop and its bounded interior bijectively. The strict
// local height sign extends over the disk because complete coverage
// excludes additional equality there. This is a graph lens, not a general
// curved shell sewing operation.
func (r AlgebraicTrimmedPairSection) SealAlgebraicTubeLens(ctx context.Context, o AlgebraicTubeLensOptions) (AlgebraicTubeLens, error) {
	var l AlgebraicTubeLens
	if ctx == nil || r.partitionSource == nil {
		return l, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return l, e
	}
	source := r.partitionSource
	if source.projection != [2]int{0, 1} {
		return l, geom.ErrConvergence
	}
	branch := -1
	for i, b := range source.branches {
		for _, n := range b.Nodes {
			if n.Isolated {
				return l, geom.ErrConvergence
			}
		}
		if len(b.Runs) == 0 {
			continue
		}
		if branch >= 0 || len(b.Runs) != 1 || !b.Runs[0].Closed {
			return l, geom.ErrConvergence
		}
		branch = i
	}
	if branch < 0 {
		return l, geom.ErrConvergence
	}
	for i, f := range source.faces {
		if !convexStraightTubeLensFace(f) {
			return AlgebraicTubeLens{}, geom.ErrConvergence
		}
		s, e := geom.NewSurfacePatch(facespline(f), o.Graph)
		if e != nil {
			return AlgebraicTubeLens{}, e
		}
		l.graphs[i], e = geom.NewAffineXYGraph(ctx, s, o.Graph)
		if e != nil {
			return AlgebraicTubeLens{}, e
		}
	}
	p, e := r.PartitionBothFacesByTubeRun(ctx, branch, 0, o.Partition)
	if e != nil {
		return AlgebraicTubeLens{}, e
	}
	s0, s1 := p.Faces[0].FragmentHeightSigns[0], p.Faces[1].FragmentHeightSigns[0]
	if s0 == 0 || s0 != -s1 || !p.Faces[0].CutClosed || !p.Faces[1].CutClosed {
		return AlgebraicTubeLens{}, geom.ErrConvergence
	}
	l.faces, l.section, l.branch = p.Faces, r, branch
	if s0 < 0 {
		l.upper = 1
	}
	f := l.Faces()
	if (f[0].Reversed != f[0].CutReversed) == (f[1].Reversed != f[1].CutReversed) {
		return AlgebraicTubeLens{}, geom.ErrConvergence
	}
	return l, nil
}

func convexStraightTubeLensFace(f Face) bool {
	n := len(f.Outer)
	if len(f.Holes) != 0 || n < 3 {
		return false
	}
	for i, a := range f.Outer {
		if a.Curve != nil {
			return false
		}
		z := f.Outer[(i+1)%n]
		dx := new(big.Rat).Sub(ratInset(a.UVEnd.X), ratInset(a.UVStart.X))
		dy := new(big.Rat).Sub(ratInset(a.UVEnd.Y), ratInset(a.UVStart.Y))
		x := new(big.Rat).Sub(ratInset(z.UVEnd.X), ratInset(z.UVStart.X))
		y := new(big.Rat).Sub(ratInset(z.UVEnd.Y), ratInset(z.UVStart.Y))
		if new(big.Rat).Sub(new(big.Rat).Mul(dx, y), new(big.Rat).Mul(dy, x)).Sign() < 0 {
			return false
		}
	}
	return true
}

type AlgebraicTubeLensVerticalInterval struct {
	Complete        bool
	Present, Rim    bool
	Lower, Upper    *big.Rat
	Parameters      [2][2]*big.Rat // original charts
	ExactOperations int
}

// VerticalInterval returns the exact material section above rational model
// XY. Complete and !Present proves exclusion. Rim denotes a singleton on
// the common section. Original trim membership and exact height sign are
// evaluated at the query; global interpretation comes from section coverage.
func (l AlgebraicTubeLens) VerticalInterval(ctx context.Context, xy [2]*big.Rat,
	o geom.ExactCurvePlaneEventOptions) (r AlgebraicTubeLensVerticalInterval, err error) {
	if ctx == nil || l.graphs[0] == nil || xy[0] == nil || xy[1] == nil {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 0 {
		return r, geom.ErrInvalid
	}
	ev := func() geom.SurfacePatchEvaluationOptions {
		return geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - r.ExactOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	}
	for i, g := range l.graphs {
		p := ev()
		if p.MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		u, e := g.InvertRational(ctx, xy, p)
		r.ExactOperations += u.Operations
		if e != nil {
			return r, e
		}
		if !u.Inside {
			r.Complete = true
			return r, nil
		}
		r.Parameters[i] = u.UV
		if !insideConvexTubeFace(l.section.partitionSource.faces[i], u.UV) {
			r.Complete = true
			return r, nil
		}
	}
	p := ev()
	if p.MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	heights, e := l.graphs[0].CompareHeightAtRational(ctx, l.graphs[1], r.Parameters[0], p)
	r.ExactOperations += heights.Operations
	if e != nil {
		return r, e
	}
	if heights.TargetOutside || !heights.Complete {
		return r, geom.ErrConvergence
	}
	// The intersection of the two convex projected faces is convex.
	// The single section separates it into a bounded disk and a
	// connected exterior with the opposite strict height sign.
	if heights.Sign == 0 {
		r.Rim, r.Present, r.Complete = true, true, true
		r.Lower = new(big.Rat).Set(heights.SourcePoint[2])
		r.Upper = new(big.Rat).Set(r.Lower)
		return r, nil
	}
	if heights.Sign == l.faces[0].FragmentHeightSigns[0] {
		z := [2]*big.Rat{heights.SourcePoint[2], heights.TargetPoint[2]}
		if z[l.upper].Cmp(z[l.upper^1]) <= 0 {
			return r, geom.ErrConvergence
		}
		r.Lower, r.Upper = z[l.upper^1], z[l.upper]
		r.Present = true
	}
	r.Complete = true
	return r, nil
}
