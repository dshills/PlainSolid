package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicTubeFaceBoundaryUse is one exact restriction of an original
// coedge, in loop order. The parameters use the coedge's normalized [0,1]
// traversal; Start and End may be nonrational.
type AlgebraicTubeFaceBoundaryUse struct {
	Loop, Use, Edge int // outer is loop zero
	Reversed        bool
	Start, End      *geom.ExactAlgebraicRoot
}

// AlgebraicTubeFaceCutPiece is one exact restriction of a verified tube.
// FreeStart and FreeEnd follow the run orientation, in original chart units.
type AlgebraicTubeFaceCutPiece struct {
	Segment              intersect.TubeSegment
	FreeStart, FreeEnd   *geom.ExactAlgebraicRoot
	StartPoint, EndPoint *geom.ExactComposedCurveEvent
}

type AlgebraicTubeFaceFragment struct {
	Outer []AlgebraicTubeFaceBoundaryUse
	Holes [][]AlgebraicTubeFaceBoundaryUse
	// CutReversed means the appended section uses End-to-Start order.
	CutReversed bool
	// CutIsHole applies to a closed section when this fragment retains
	// the original exterior loop and traverses the cut as an inner loop.
	CutIsHole  bool
	CutIsOuter bool // a closed section forms this fragment's complete exterior
}

// AlgebraicTubeFacePartition contains exact fragment loops on the original
// support. It retains every original coedge portion and both opposite cut
// uses. A nonseparating cut can merge two boundary components; a separating
// cut returns two fragments. Closed cut orientation is proved in chart UV.
type AlgebraicTubeFacePartition struct {
	Chart, Branch, Run int
	Support            *geom.BSplineSurface
	// SourceSurface retains the original chart for both support types. Support
	// is also set when the original chart is a float B-spline.
	SourceSurface geom.Surface
	FaceReversed  bool
	Terminals     [2]SplineTubeTrimContact
	Cut           []AlgebraicTubeFaceCutPiece
	CutClosed     bool
	CutWinding    int // original run winding in chart UV, zero for open cuts
	Fragments     [2]AlgebraicTubeFaceFragment
	// FragmentHeightSigns are signs of this support minus the other
	// support along ResidualAxis at common projected coordinates. Zero means the
	// oriented residual was not proved. This does not classify a solid.
	FragmentHeightSigns [2]int
	ProjectionAxes      [2]int
	ResidualAxis        int
	// ConnectedBoundary is set when a single arc joins two original
	// boundary components. The material stays connected; the two cut
	// uses occur in one merged boundary loop instead of two fragments.
	ConnectedBoundary []AlgebraicTubeMergedBoundaryUse
	RemainingHoles    [][]AlgebraicTubeFaceBoundaryUse
	ExactOperations   int
}

type AlgebraicTubeMergedBoundaryUse struct {
	Original    *AlgebraicTubeFaceBoundaryUse
	CutReversed bool // Original == nil denotes the complete section run
}

type AlgebraicTubeFacePartitionOptions struct {
	geom.ExactCurvePlaneEventOptions
	Patches     geom.SurfacePatchOptions
	Injectivity geom.SurfaceInjectivityOptions
}

// AlgebraicTubePairedPartition splits both original charts at a single
// shared section run. Both fragment pairs use the same exact free roots and
// tube sequence; they are not sewn by comparing displayed curves.
type AlgebraicTubePairedPartition struct {
	Faces           [2]AlgebraicTubeFacePartition
	ExactOperations int
}

func (r AlgebraicTrimmedPairSection) PartitionBothFacesByTubeRun(ctx context.Context, branch, run int,
	o AlgebraicTubeFacePartitionOptions) (p AlgebraicTubePairedPartition, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
	}
	for i := range p.Faces {
		child := o
		child.MaxOperations -= p.ExactOperations
		if child.MaxOperations < 1 {
			return AlgebraicTubePairedPartition{ExactOperations: p.ExactOperations}, geom.ErrConvergence
		}
		f, e := r.PartitionFaceByTubeRun(ctx, i, branch, run, child)
		p.ExactOperations += f.ExactOperations
		if e != nil {
			return AlgebraicTubePairedPartition{ExactOperations: p.ExactOperations}, e
		}
		p.Faces[i] = f
	}
	return p, nil
}

type trimmedSectionPartitionSource struct {
	patches    [2]intersect.Patch
	faces      [2]Face
	branches   []AlgebraicTrimmedSectionBranch
	segments   [][]intersect.TubeSegment
	projection [2]int
}

func captureTrimmedSectionPartitions(patches [2]intersect.Patch, faces [2]Face,
	branches []AlgebraicTrimmedSectionBranch, segments [][]intersect.TubeSegment, projection *[2]int) *trimmedSectionPartitionSource {
	s := &trimmedSectionPartitionSource{patches: patches, faces: faces, projection: [2]int{0, 1}}
	if projection != nil {
		s.projection = *projection
	}
	for i, f := range s.faces {
		copyUses := func(l []Use) []Use {
			out := append([]Use(nil), l...)
			for j := range out {
				switch q := out[j].Curve.(type) {
				case *SplineUV:
					if q != nil {
						out[j].Curve = *q
					}
				case *QuadraticUV:
					if q != nil {
						out[j].Curve = *q
					}
				}
			}
			return out
		}
		f.Outer = copyUses(f.Outer)
		holes := f.Holes
		f.Holes = make([][]Use, len(f.Holes))
		for j, h := range holes {
			f.Holes[j] = copyUses(h)
		}
		s.faces[i] = f
	}
	for i, b := range branches {
		x := AlgebraicTrimmedSectionBranch{}
		for _, n := range b.Nodes {
			n.Events = append([]AlgebraicTrimmedSectionEventRef(nil), n.Events...)
			x.Nodes = append(x.Nodes, n)
		}
		for _, run := range b.Runs {
			run.Intervals = append([]AlgebraicTrimmedSectionInterval(nil), run.Intervals...)
			x.Runs = append(x.Runs, run)
		}
		for _, clip := range b.Clips {
			c := SplineTubeClipReport{}
			for j, f := range clip.FaceTrims {
				c.FaceTrims[j].Coincidences = append([]SplineTubeTrimCoincidence(nil), f.Coincidences...)
			}
			for _, event := range clip.Events {
				for j := range event.Contacts {
					event.Contacts[j] = append([]SplineTubeTrimContact(nil), event.Contacts[j]...)
				}
				c.Events = append(c.Events, event)
			}
			for _, iv := range clip.Intervals {
				iv.Witness = nil
				c.Intervals = append(c.Intervals, iv)
			}
			x.Clips = append(x.Clips, c)
		}
		s.branches = append(s.branches, x)
		s.segments = append(s.segments, append([]intersect.TubeSegment(nil), segments[i]...))
	}
	return s
}

// PartitionFaceByTubeRun splits an injective face by a simple open run with
// two regular coedge crossings, or a simple closed run strictly interior
// to all trims. An open cut between different boundary components merges
// their loops without separating material. A same-hole return is unresolved.
// Existing disjoint holes retain their original ranges. The complete trace
// proves simplicity; shell embedding and Boolean selection are separate.
func (r AlgebraicTrimmedPairSection) PartitionFaceByTubeRun(ctx context.Context, chart, branch, run int,
	o AlgebraicTubeFacePartitionOptions) (p AlgebraicTubeFacePartition, err error) {
	defer func() {
		if err != nil {
			p = AlgebraicTubeFacePartition{ExactOperations: p.ExactOperations}
		}
	}()
	s := r.runSource()
	if ctx == nil || s == nil || chart < 0 || chart > 1 || branch < 0 || branch >= len(s.branches) ||
		run < 0 || run >= len(s.branches[branch].Runs) {
		return p, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return p, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 0 {
		return p, geom.ErrInvalid
	}
	f := s.faces[chart]
	b := s.branches[branch]
	path := b.Runs[run]
	if len(path.Intervals) == 0 {
		return p, geom.ErrConvergence
	}
	p.Chart, p.Branch, p.Run, p.FaceReversed = chart, branch, run, f.Reversed
	p.ProjectionAxes = s.projection
	p.ResidualAxis = 3 - s.projection[0] - s.projection[1]
	p.SourceSurface = f.Surface
	p.Support, _ = f.Surface.(*geom.BSplineSurface)
	limits := o.Patches
	if limits.MaxOperations == 0 || limits.MaxOperations > o.MaxOperations {
		limits.MaxOperations = o.MaxOperations
	}
	if limits.MaxCoefficientBits == 0 || limits.MaxCoefficientBits > o.MaxCoefficientBits {
		limits.MaxCoefficientBits = o.MaxCoefficientBits
	}
	makePatch := func(s geom.Surface, o geom.SurfacePatchOptions) (*geom.SurfacePatch, error) {
		if q, ok := s.(*geom.ExactBezierSurface); ok {
			return q.ExactPatch().WithLimits(o)
		}
		return exactSplineSupportPatch(s, o)
	}
	support, e := makePatch(p.SourceSurface, limits)
	if e != nil {
		return p, e
	}
	p.ExactOperations += support.ExactOperations()
	if p.ExactOperations >= o.MaxOperations {
		return p, geom.ErrConvergence
	}
	inj := o.Injectivity
	if inj.ProjectionAxes == [2]int{} {
		inj.ProjectionAxes = s.projection
	}
	if inj.MaxOperations == 0 || inj.MaxOperations > o.MaxOperations-p.ExactOperations {
		inj.MaxOperations = o.MaxOperations - p.ExactOperations
	}
	if inj.MaxCoefficientBits == 0 || inj.MaxCoefficientBits > o.MaxCoefficientBits {
		inj.MaxCoefficientBits = o.MaxCoefficientBits
	}
	proof, e := geom.CertifySurfaceInjectivity(support, inj)
	p.ExactOperations += proof.ExactOperations
	if e != nil {
		return p, e
	}
	if !proof.ProvenInjective {
		return p, geom.ErrConvergence
	}
	// For [Au,Av,-Bu,-Bv], the first two signed cofactors equal
	// det(B_xy)*(r_v,-r_u), where r=ZA-ZB at common XY. The last
	// two equal -det(A_xy)*(r_v,-r_u) for r=ZB-ZA. The trace
	// follows the canonical cofactor orientation.
	leftSign := 0
	otherLimits := limits
	otherLimits.MaxOperations = o.MaxOperations - p.ExactOperations
	if otherLimits.MaxOperations < 1 {
		return p, geom.ErrConvergence
	}
	other, e := makePatch(s.faces[chart^1].Surface, otherLimits)
	if e != nil {
		return p, e
	}
	p.ExactOperations += other.ExactOperations()
	otherLimits.MaxOperations = o.MaxOperations - p.ExactOperations
	if otherLimits.MaxOperations < 1 {
		return p, geom.ErrConvergence
	}
	sign, used, e := geom.AffineProjectedGraphOrientation(ctx, other, s.projection, otherLimits)
	p.ExactOperations += used
	if e == nil {
		// The canonical tangent in model XYZ changes sign under an
		// odd coordinate permutation. Preserve that orientation.
		order := [3]int{s.projection[0], s.projection[1], p.ResidualAxis}
		for i, x := range order {
			for _, y := range order[i+1:] {
				if x > y {
					sign = -sign
				}
			}
		}
		leftSign = sign
		if chart == 1 {
			leftSign = -leftSign
		}
	} else if !isUnresolvedOpenSpline(e) && e != geom.ErrDegenerate {
		return p, e
	}
	exact := func() geom.ExactCurvePlaneEventOptions {
		q := o.ExactCurvePlaneEventOptions
		q.MaxOperations -= p.ExactOperations
		return q
	}
	compare := func(a, b *geom.ExactAlgebraicRoot) (int, error) {
		q := exact()
		if q.MaxOperations < 1 {
			return 0, geom.ErrConvergence
		}
		c, e := a.Compare(ctx, b, q)
		p.ExactOperations += c.Operations
		if e == nil && !c.Proven {
			e = geom.ErrConvergence
		}
		return c.Order, e
	}
	rational := func(value int64) (*geom.ExactAlgebraicRoot, error) {
		q := exact()
		if q.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		root, used, e := geom.NewExactRationalRoot(ctx, big.NewRat(value, 1), q)
		p.ExactOperations += used
		return root, e
	}
	zero, e := rational(0)
	if e != nil {
		return p, e
	}
	one, e := rational(1)
	if e != nil {
		return p, e
	}
	contacts := func(n int) []SplineTubeTrimContact {
		var out []SplineTubeTrimContact
		for _, ref := range b.Nodes[n].Events {
			out = append(out, b.Clips[ref.Segment].Events[ref.Event].Contacts[chart]...)
		}
		return out
	}
	loopAt := func(i int) []Use {
		if i == 0 {
			return f.Outer
		}
		if i < 1 || i > len(f.Holes) {
			return nil
		}
		return f.Holes[i-1]
	}
	endpoint := func(node int) (*geom.ExactComposedCurveEvent, error) {
		for _, ref := range b.Nodes[node].Events {
			ev := b.Clips[ref.Segment].Events[ref.Event]
			for _, list := range ev.Contacts {
				if len(list) > 0 {
					return list[0].TrimEvent, nil
				}
			}
		}
		ref := b.Nodes[node].Events[0]
		ev := b.Clips[ref.Segment].Events[ref.Event]
		q := exact()
		if ev.TubeEnd < 0 || q.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		options := geom.TubeAlgebraicGraphEndpointOptions{SurfaceCurveAgreementOptions: geom.SurfaceCurveAgreementOptions{
			SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: q.MaxOperations, MaxCoefficientBits: q.MaxCoefficientBits},
			MaxSubdivisions:     q.MaxSubdivisions}}
		var point geom.TubeAlgebraicGraphEndpointCertificate
		var e error
		if s.projection == [2]int{0, 1} {
			point, e = s.segments[branch][ref.Segment].DiscoverAlgebraicGraphEndpoint(ctx, ev.TubeEnd, options)
		} else {
			point, e = s.segments[branch][ref.Segment].DiscoverAlgebraicProjectedGraphEndpoint(ctx, ev.TubeEnd, s.projection, options)
		}
		p.ExactOperations += point.ExactOperations
		if e != nil {
			return nil, e
		}
		if !point.Proven || point.ModelEvent == nil {
			return nil, geom.ErrConvergence
		}
		return point.ModelEvent, nil
	}
	addCut := func() error {
		a, e := endpoint(path.Intervals[0].Start)
		if e != nil {
			return e
		}
		for i, x := range path.Intervals {
			if e := ctx.Err(); e != nil {
				return e
			}
			if i > 0 && path.Intervals[i-1].End != x.Start || (i > 0 || path.Closed) && len(contacts(x.Start)) != 0 {
				return geom.ErrConvergence
			}
			c := b.Clips[x.Segment]
			y := c.Intervals[x.Interval]
			if y.Locations[chart] != trimvalidate.Inside {
				return geom.ErrConvergence
			}
			z, e := endpoint(x.End)
			if e != nil {
				return e
			}
			p.Cut = append(p.Cut, AlgebraicTubeFaceCutPiece{
				Segment: s.segments[branch][x.Segment], FreeStart: c.Events[y.Start].Free, FreeEnd: c.Events[y.End].Free,
				StartPoint: a, EndPoint: z})
			a = z
		}
		if path.Closed {
			p.Cut[len(p.Cut)-1].EndPoint = p.Cut[0].StartPoint
		}
		return nil
	}
	if path.Closed {
		if path.Intervals[0].Start != path.Intervals[len(path.Intervals)-1].End {
			return p, geom.ErrConvergence
		}
		if e := addCut(); e != nil {
			return p, e
		}
		intervals := make([]SplineTubeClipInterval, 0, len(path.Intervals))
		for _, x := range path.Intervals {
			intervals = append(intervals, b.Clips[x.Segment].Intervals[x.Interval])
		}
		winding, used, e := closedTubeWinding(ctx, chart, support, p.Cut, intervals, leftSign, exact(), s.projection)
		p.ExactOperations += used
		if e != nil {
			return p, e
		}
		for i, u := range f.Outer {
			p.Fragments[1].Outer = append(p.Fragments[1].Outer, AlgebraicTubeFaceBoundaryUse{0, i, u.Edge, u.Reversed, zero, one})
		}
		if e := assignTubePartitionHoles(ctx, f, &p, exact, func(x [2]*big.Rat, o geom.ExactCurvePlaneEventOptions) (bool, int, error) {
			return classifyHoleAgainstClosedTube(ctx, chart, support, p.Cut, x, o, s.projection)
		}); e != nil {
			return p, e
		}
		p.CutClosed = true
		p.CutWinding = winding
		p.Fragments[0].CutIsOuter = true
		p.Fragments[0].CutReversed = winding < 0
		p.Fragments[1].CutIsHole = true
		p.Fragments[1].CutReversed = winding > 0
		p.FragmentHeightSigns = [2]int{leftSign * winding, -leftSign * winding}
		return p, nil
	}
	var terminalParameters [2]*geom.ExactAlgebraicRoot
	terminalNodes := [2]int{path.Intervals[0].Start, path.Intervals[len(path.Intervals)-1].End}
	if terminalNodes[0] == terminalNodes[1] {
		return p, geom.ErrConvergence
	}
	for i, n := range terminalNodes {
		cs := contacts(n)
		if len(cs) == 0 {
			return p, geom.ErrConvergence
		}
		x := cs[0]
		l := loopAt(x.Source.Loop)
		if !x.Transverse || x.Source.Use < 0 || x.Source.Use >= len(l) {
			return p, geom.ErrConvergence
		}
		for _, c := range cs[1:] {
			if !c.Transverse || c.Source.Loop != x.Source.Loop || c.Source.Use != x.Source.Use || c.Source.Span != x.Source.Span {
				return p, geom.ErrConvergence
			}
			order, e := compare(x.Parameter, c.Parameter)
			if e != nil {
				return p, e
			}
			if order != 0 {
				return p, geom.ErrConvergence
			}
		}
		p.Terminals[i] = x
		t := x.Parameter
		if spline, ok := shortSplineTrim(l[x.Source.Use].Curve); ok {
			a, z := ratInset(spline.Range.Min), ratInset(spline.Range.Max)
			if spline.Reversed {
				a, z = z, a
			}
			scale := new(big.Rat).Inv(new(big.Rat).Sub(z, a))
			offset := new(big.Rat).Neg(new(big.Rat).Mul(a, scale))
			q := exact()
			if q.MaxOperations < 1 {
				return p, geom.ErrConvergence
			}
			mapped, used, e := t.AffineMap(ctx, offset, scale, q)
			p.ExactOperations += used
			if e != nil {
				return p, e
			}
			t = mapped
		}
		terminalParameters[i] = t
		a, e := compare(zero, t)
		if e != nil {
			return p, e
		}
		z, e := compare(t, one)
		if e != nil {
			return p, e
		}
		if a != -1 || z != -1 {
			return p, geom.ErrConvergence
		}
	}
	if e := addCut(); e != nil {
		return p, e
	}
	l0, l1 := p.Terminals[0].Source.Loop, p.Terminals[1].Source.Loop
	if l0 != l1 {
		// A proper arc between two different boundary components is
		// nonseparating. Traverse each original component once, entering
		// and leaving the same cut on opposite sides.
		for i, term := range p.Terminals {
			l := loopAt(term.Source.Loop)
			first := term.Source.Use
			t := terminalParameters[i]
			add := func(j int, a, z *geom.ExactAlgebraicRoot) {
				u := l[j]
				x := AlgebraicTubeFaceBoundaryUse{term.Source.Loop, j, u.Edge, u.Reversed, a, z}
				p.ConnectedBoundary = append(p.ConnectedBoundary, AlgebraicTubeMergedBoundaryUse{Original: &x})
			}
			add(first, t, one)
			for j := (first + 1) % len(l); j != first; j = (j + 1) % len(l) {
				add(j, zero, one)
			}
			add(first, zero, t)
			p.ConnectedBoundary = append(p.ConnectedBoundary, AlgebraicTubeMergedBoundaryUse{CutReversed: i == 1})
		}
		for i, h := range f.Holes {
			if i+1 == l0 || i+1 == l1 {
				continue
			}
			var loop []AlgebraicTubeFaceBoundaryUse
			for j, u := range h {
				loop = append(loop, AlgebraicTubeFaceBoundaryUse{i + 1, j, u.Edge, u.Reversed, zero, one})
			}
			p.RemainingHoles = append(p.RemainingHoles, loop)
		}
		return p, nil
	}
	if l0 != 0 {
		return p, geom.ErrConvergence
	}
	for i := range p.Fragments {
		j := i ^ 1
		first, last := p.Terminals[i].Source.Use, p.Terminals[j].Source.Use
		a, z := terminalParameters[i], terminalParameters[j]
		fragment := &p.Fragments[i]
		fragment.CutReversed = i == 0
		add := func(use int, x, y *geom.ExactAlgebraicRoot) {
			u := f.Outer[use]
			fragment.Outer = append(fragment.Outer, AlgebraicTubeFaceBoundaryUse{0, use, u.Edge, u.Reversed, x, y})
		}
		if first == last {
			order, e := compare(a, z)
			if e != nil {
				return p, e
			}
			if order == 0 {
				return p, geom.ErrConvergence
			}
			if order < 0 {
				add(first, a, z)
				continue
			}
		}
		add(first, a, one)
		for k := (first + 1) % len(f.Outer); k != last; k = (k + 1) % len(f.Outer) {
			add(k, zero, one)
		}
		add(last, zero, z)
	}
	if e := ctx.Err(); e != nil {
		return p, e
	}
	if e := assignTubePartitionHoles(ctx, f, &p, exact, func(x [2]*big.Rat, o geom.ExactCurvePlaneEventOptions) (bool, int, error) {
		return classifyHoleAgainstOpenFragment(ctx, chart, support, f, p.Cut, p.Fragments[0].Outer, x, o, s.projection)
	}); e != nil {
		return p, e
	}
	p.FragmentHeightSigns = [2]int{-leftSign, leftSign}
	return p, nil
}

func assignTubePartitionHoles(ctx context.Context, f Face, p *AlgebraicTubeFacePartition,
	exact func() geom.ExactCurvePlaneEventOptions,
	classify func([2]*big.Rat, geom.ExactCurvePlaneEventOptions) (bool, int, error)) error {
	if len(f.Holes) == 0 {
		return nil
	}
	pair := [2]*geom.ExactAlgebraicRoot{}
	for i := range pair {
		o := exact()
		if o.MaxOperations < 1 {
			return geom.ErrConvergence
		}
		x, used, e := geom.NewExactRationalRoot(ctx, big.NewRat(int64(i), 1), o)
		p.ExactOperations += used
		if e != nil {
			return e
		}
		pair[i] = x
	}
	for i, h := range f.Holes {
		inside, classified := false, false
		for _, use := range h {
			x := [2]*big.Rat{ratInset(use.UVStart.X), ratInset(use.UVStart.Y)}
			if c, ok := use.Curve.(ExactQuadraticUV); ok {
				h := c.HomogeneousControls()[0]
				for k := range x {
					x[k] = new(big.Rat).Quo(h[k], h[2])
				}
			}
			where, used, e := classify(x, exact())
			p.ExactOperations += used
			if e == nil {
				inside, classified = where, true
				break
			}
			if !errors.Is(e, geom.ErrConvergence) {
				return e
			}
		}
		if !classified {
			return geom.ErrConvergence
		}
		fragment := 1
		if inside {
			fragment = 0
		}
		loop := make([]AlgebraicTubeFaceBoundaryUse, 0, len(h))
		for j, u := range h {
			loop = append(loop, AlgebraicTubeFaceBoundaryUse{i + 1, j, u.Edge, u.Reversed, pair[0], pair[1]})
		}
		p.Fragments[fragment].Holes = append(p.Fragments[fragment].Holes, loop)
	}
	return nil
}
func facespline(f Face) *geom.BSplineSurface {
	p, _ := f.Surface.(*geom.BSplineSurface)
	return p
}
