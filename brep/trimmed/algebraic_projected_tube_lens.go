package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AlgebraicProjectedTubeLens retains a closed exact lens in two original
// charts with a selected model-coordinate projection. Upper means larger
// coordinate along ResidualAxis, which need not be model Z.
type AlgebraicProjectedTubeLens struct {
	section         AlgebraicTrimmedPairSection
	faces           [2]AlgebraicTubeFacePartition
	graphs          [2]*geom.AffineProjectedGraph
	axes            [2]int
	residual, upper int
}

func (l AlgebraicProjectedTubeLens) ProjectionAxes() [2]int { return l.axes }
func (l AlgebraicProjectedTubeLens) ResidualAxis() int      { return l.residual }

func (l AlgebraicProjectedTubeLens) Topology() AlgebraicTubeLensTopology {
	if l.graphs[0] == nil {
		return AlgebraicTubeLensTopology{}
	}
	n := len(l.faces[0].Cut)
	return AlgebraicTubeLensTopology{n, n, 2, 2}
}

func (l AlgebraicProjectedTubeLens) Faces() (out [2]AlgebraicTubeLensFace) {
	if l.graphs[0] == nil {
		return out
	}
	parity := projectedAxisParity(l.axes)
	for i, f := range l.faces {
		out[i] = AlgebraicTubeLensFace{Support: f.Support, SourceChart: i,
			Reversed: (i != l.upper) == (parity*l.graphs[i].Orientation() > 0), CutReversed: f.Fragments[0].CutReversed}
	}
	return out
}

func projectedAxisParity(axes [2]int) int {
	order := [3]int{axes[0], axes[1], 3 - axes[0] - axes[1]}
	p := 1
	for i, x := range order {
		for _, y := range order[i+1:] {
			if x > y {
				p = -p
			}
		}
	}
	return p
}

// SealProjectedAlgebraicTubeLens proves a closed material boundary for
// one complete regular section on convex original face trims. The
// projection is the one used to certify the original trimmed section.
func (s AlgebraicTrimmedPairSection) SealProjectedAlgebraicTubeLens(ctx context.Context, o AlgebraicTubeLensOptions) (l AlgebraicProjectedTubeLens, err error) {
	defer func() {
		if err != nil {
			l = AlgebraicProjectedTubeLens{}
		}
	}()
	if ctx == nil || s.partitionSource == nil {
		return l, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return l, e
	}
	src := s.partitionSource
	l.axes = src.projection
	l.residual = 3 - l.axes[0] - l.axes[1]
	branch := -1
	for i, b := range src.branches {
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
	for i, f := range src.faces {
		if !convexStraightTubeLensFace(f) {
			return l, geom.ErrConvergence
		}
		p, e := geom.NewSurfacePatch(facespline(f), o.Graph)
		if e != nil {
			return l, e
		}
		l.graphs[i], e = geom.NewAffineProjectedGraph(ctx, p, l.axes, o.Graph)
		if e != nil {
			return l, e
		}
	}
	p, e := s.PartitionBothFacesByTubeRun(ctx, branch, 0, o.Partition)
	if e != nil {
		return l, e
	}
	a, b := p.Faces[0].FragmentHeightSigns[0], p.Faces[1].FragmentHeightSigns[0]
	if a == 0 || b != -a || !p.Faces[0].CutClosed || !p.Faces[1].CutClosed {
		return l, geom.ErrConvergence
	}
	if a < 0 {
		l.upper = 1
	}
	l.faces, l.section = p.Faces, s
	f := l.Faces()
	if (f[0].Reversed != f[0].CutReversed) == (f[1].Reversed != f[1].CutReversed) {
		return l, geom.ErrConvergence
	}
	return l, nil
}

// ProjectionInterval returns material along the residual model axis at
// the supplied exact model coordinates in ProjectionAxes order.
func (l AlgebraicProjectedTubeLens) ProjectionInterval(ctx context.Context, xy [2]*big.Rat,
	o geom.ExactCurvePlaneEventOptions) (r AlgebraicTubeLensVerticalInterval, err error) {
	if ctx == nil || l.graphs[0] == nil || xy[0] == nil || xy[1] == nil {
		return r, geom.ErrInvalid
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
	opts := func() geom.SurfacePatchEvaluationOptions {
		return geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.MaxOperations - r.ExactOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	}
	for i, g := range l.graphs {
		p := opts()
		if p.MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		u, e := g.InvertRational(ctx, xy, p)
		r.ExactOperations += u.Operations
		if e != nil {
			return r, e
		}
		if !u.Inside || !insideConvexTubeFace(l.section.partitionSource.faces[i], u.UV) {
			r.Complete = true
			return r, nil
		}
		r.Parameters[i] = u.UV
	}
	p := opts()
	if p.MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	x, e := l.graphs[0].CompareHeightAtRational(ctx, l.graphs[1], r.Parameters[0], p)
	r.ExactOperations += x.Operations
	if e != nil {
		return r, e
	}
	if !x.Complete || x.TargetOutside {
		return r, geom.ErrConvergence
	}
	z := [2]*big.Rat{x.SourcePoint[l.residual], x.TargetPoint[l.residual]}
	if x.Sign == 0 {
		r.Rim, r.Present = true, true
		r.Lower, r.Upper = z[0], new(big.Rat).Set(z[0])
	} else if x.Sign == l.faces[0].FragmentHeightSigns[0] {
		r.Present = true
		r.Lower, r.Upper = z[l.upper^1], z[l.upper]
	}
	r.Complete = true
	return r, nil
}

func (l AlgebraicProjectedTubeLens) LocateRational(ctx context.Context, p [3]*big.Rat, o geom.ExactCurvePlaneEventOptions) (parametric.Location, error) {
	if l.graphs[0] == nil || p[l.residual] == nil {
		return parametric.Outside, geom.ErrInvalid
	}
	v, e := l.ProjectionInterval(ctx, [2]*big.Rat{p[l.axes[0]], p[l.axes[1]]}, o)
	if e != nil {
		return parametric.Outside, e
	}
	return locateTubeVerticalInterval(p[l.residual], v)
}

func (l AlgebraicProjectedTubeLens) Solid(ctx context.Context, tolerance float64, o edgecheck.Options) (Solid, error) {
	if l.graphs[0] == nil {
		return Solid{}, geom.ErrInvalid
	}
	f := l.Faces()
	var faces [2]Face
	for i, x := range f {
		faces[i] = Face{Surface: x.Support, Reversed: x.Reversed}
	}
	r := tubeRegionSolidRing{cut: l.faces[0].Cut, reversed: [2]bool{f[0].CutReversed, f[1].CutReversed}}
	return assembleTubeRegionSolid(ctx, faces, []tubeRegionSolidRing{r}, tolerance, o)
}

// BoundMassProperties reports exact rational volume, centroid and inertia
// intervals in original model coordinates for matched rectangular charts.
func (l AlgebraicProjectedTubeLens) BoundMassProperties(ctx context.Context, o AlgebraicTubeLensVolumeOptions) (AlgebraicTubeLensMassBounds, error) {
	if l.graphs[0] == nil {
		return AlgebraicTubeLensMassBounds{}, geom.ErrInvalid
	}
	m, e := l.graphs[0].InverseCoefficients()
	if e != nil {
		return AlgebraicTubeLensMassBounds{}, e
	}
	return boundTubeLensMass(ctx, l.section, l.upper, l.axes, m, o, true)
}

func (l AlgebraicProjectedTubeLens) BoundVolume(ctx context.Context, o AlgebraicTubeLensVolumeOptions) (AlgebraicTubeLensVolumeBounds, error) {
	if l.graphs[0] == nil {
		return AlgebraicTubeLensVolumeBounds{}, geom.ErrInvalid
	}
	m, e := l.graphs[0].InverseCoefficients()
	if e != nil {
		return AlgebraicTubeLensVolumeBounds{}, e
	}
	r, e := boundTubeLensMass(ctx, l.section, l.upper, l.axes, m, o, false)
	return r.AlgebraicTubeLensVolumeBounds, e
}
