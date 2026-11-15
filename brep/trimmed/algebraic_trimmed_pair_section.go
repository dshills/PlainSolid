package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type AlgebraicTrimmedPairSectionOptions struct {
	Trace                     intersect.SplinePairTraceOptions
	Clip                      SplineTubeClipOptions
	MaxSegments, MaxIntervals int
}

type AlgebraicTrimmedSectionEventRef struct {
	Segment, Event int
}

// AlgebraicTrimmedSectionNode combines events at one proved branch junction.
// Event records (including original coedges) are accessible through Clips.
// OnFaces is true for a point in the closure of both trimmed faces.
// Isolated is true for a section point with no incident retained interval.
type AlgebraicTrimmedSectionNode struct {
	Events            []AlgebraicTrimmedSectionEventRef
	OnFaces, Isolated bool
}

type AlgebraicTrimmedSectionInterval struct {
	Segment, Interval int
	Start, End        int // indices in Nodes
}

type AlgebraicTrimmedSectionRun struct {
	Intervals []AlgebraicTrimmedSectionInterval
	Closed    bool
}

type AlgebraicTrimmedSectionBranch struct {
	Clips []SplineTubeClipReport
	Nodes []AlgebraicTrimmedSectionNode
	Runs  []AlgebraicTrimmedSectionRun
}

type AlgebraicTrimmedPairSection struct {
	Trace                                       intersect.SplinePairTrace
	Branches                                    []AlgebraicTrimmedSectionBranch
	ExactOperations, LoopWork, LoopSubdivisions int
	Complete                                    bool
	native                                      *nativeTrimmedPairSection
	partitionSource                             *trimmedSectionPartitionSource
	exactPartitionSource                        *trimmedSectionPartitionSource
}

func (r AlgebraicTrimmedPairSection) runSource() *trimmedSectionPartitionSource {
	if r.partitionSource != nil {
		return r.partitionSource
	}
	return r.exactPartitionSource
}

// SectionTrimmedSplinePair proves all contacts between two trimmed faces on
// the supported regular graph branch subset. The input patches must cover the
// entire original trims; the trace is independently certified on that larger
// domain before classification. Optional seeds provide proposals only.
// Results include closed runs, open runs, and isolated retained points. They
// are section topology and do not edit either input face.
func SectionTrimmedSplinePair(ctx context.Context, patches [2]intersect.Patch, faces [2]Face,
	seeds *intersect.SplineSeedReport, o AlgebraicTrimmedPairSectionOptions) (r AlgebraicTrimmedPairSection, err error) {
	if ctx == nil || o.MaxSegments < 0 || o.MaxSegments > 4096 || o.MaxIntervals < 0 || o.MaxIntervals > 65536 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.Clip.Projection == nil && o.Trace.ProjectedGraphFallback.Enabled {
		o.Clip.Projection = &o.Trace.GraphProjectionAxes
	}
	if p := o.Clip.Projection; p != nil {
		if p[0] < 0 || p[0] > 2 || p[1] < 0 || p[1] > 2 || p[0] == p[1] {
			return r, geom.ErrInvalid
		}
		if !o.Trace.Coverage.ProjectedAffineFallback && !o.Trace.ProjectedGraphFallback.Enabled {
			o.Trace.Coverage.ProjectedAffineFallback = true
			o.Trace.Coverage.ProjectionAxes = *p
		}
	}
	for i, p := range patches {
		if !sameTrimmedSectionChart(p.Surface, faces[i].Surface) {
			return r, geom.ErrDomain
		}
		box, e := faceUVBounds(faces[i])
		if e != nil {
			return r, e
		}
		if box.UMin < p.Min.X || box.VMin < p.Min.Y || box.UMax > p.Max.X || box.VMax > p.Max.Y {
			return r, geom.ErrDomain
		}
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 256
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 4096
	}
	if o.Clip.MaxOperations == 0 {
		o.Clip.MaxOperations = 128000000
	}
	if o.Clip.MaxIntervals < 0 || o.Clip.MaxIntervals > 65536 || o.Clip.MaxOperations < 1 ||
		o.Clip.MaxCoefficientBits != 0 && o.Clip.MaxCoefficientBits < 64 || o.Clip.MaxSubdivisions < 0 || o.Clip.MaxEvents < 0 {
		return r, geom.ErrInvalid
	}
	o.Clip.Loops, err = o.Clip.Loops.WithDefaults()
	if err != nil {
		return r, err
	}
	for _, face := range faces {
		child := o.Clip.Loops
		child.MaxWork -= r.LoopWork
		child.MaxSubdivisions -= r.LoopSubdivisions
		if child.MaxWork < 1 || child.MaxSubdivisions < 1 {
			return r, geom.ErrConvergence
		}
		p, e := ValidateFaceTrimLoops(ctx, face, child)
		r.LoopWork += p.Work
		r.LoopSubdivisions += p.Subdivisions
		if e != nil {
			return r, e
		}
		if p.Status != trimvalidate.Certified {
			return r, geom.ErrConvergence
		}
	}
	if seeds == nil {
		r.Trace, err = intersect.TraceSplinePair(ctx, patches[0], patches[1], o.Trace)
	} else {
		r.Trace, err = intersect.CertifySplinePairBranches(ctx, patches[0], patches[1], *seeds, o.Trace)
	}
	if err != nil {
		return r, err
	}
	if !r.Trace.Complete || len(r.Trace.Curves) != len(r.Trace.BranchGraph.Branches) {
		return r, geom.ErrConvergence
	}
	segments, intervals := 0, 0
	coincident := false
	for i, branch := range r.Trace.Curves {
		if len(branch) == 0 || len(branch) > o.MaxSegments-segments {
			return r, geom.ErrConvergence
		}
		segments += len(branch)
		clips := make([]SplineTubeClipReport, 0, len(branch))
		for _, segment := range branch {
			child := o.Clip
			child.MaxOperations -= r.ExactOperations
			child.Loops.MaxWork -= r.LoopWork
			child.Loops.MaxSubdivisions -= r.LoopSubdivisions
			remaining := o.MaxIntervals - intervals
			if child.MaxIntervals == 0 || child.MaxIntervals > remaining {
				child.MaxIntervals = remaining
			}
			if child.MaxOperations < 1 || child.MaxIntervals < 1 || child.Loops.MaxWork < 1 || child.Loops.MaxSubdivisions < 1 {
				return r, geom.ErrConvergence
			}
			q, e := ClipSplineTubeByFaceTrims(ctx, segment, faces, child)
			r.ExactOperations += q.ExactOperations
			r.LoopWork += q.LoopWork
			r.LoopSubdivisions += q.LoopSubdivisions
			if e != nil {
				return r, e
			}
			if !q.Complete {
				return r, geom.ErrConvergence
			}
			for _, trim := range q.FaceTrims {
				coincident = coincident || len(trim.Coincidences) != 0
				for _, contact := range trim.Contacts {
					coincident = coincident || contact.CoincidentEndpoint
				}
			}
			intervals += len(q.Intervals)
			clips = append(clips, q)
		}
		b, e := assembleAlgebraicTrimmedSectionBranch(clips, r.Trace.BranchGraph.Branches[i].Closed)
		if e != nil {
			return r, e
		}
		r.Branches = append(r.Branches, b)
	}
	r.Complete = true
	r.native = encodeTrimmedPairSection(patches, faces, r.Trace.Curves, o.Clip.Projection, coincident)
	if coincident {
		// A section on an original edge needs a shared-edge imprint plan.
		// The face-interior partitioner accepts only new branch coedges.
		return r, nil
	}
	source := captureTrimmedSectionPartitions(patches, faces, r.Branches, r.Trace.Curves, o.Clip.Projection)
	// Solid assembly still has a separate float B-spline source gate.
	if _, a := faces[0].Surface.(*geom.BSplineSurface); a {
		if _, b := faces[1].Surface.(*geom.BSplineSurface); b {
			r.partitionSource = source
		}
	}
	if r.partitionSource == nil {
		r.exactPartitionSource = source
	}
	return r, nil
}

func sameTrimmedSectionChart(a, b geom.Surface) bool {
	a, m := parametric.SurfaceMapping(a)
	b, n := parametric.SurfaceMapping(b)
	if m != n {
		return false
	}
	switch s := a.(type) {
	case *geom.BSplineSurface:
		t, ok := b.(*geom.BSplineSurface)
		return ok && geom.SameSplineSurface(s, t)
	case *geom.ExactBezierSurface:
		t, ok := b.(*geom.ExactBezierSurface)
		return ok && s.ExactPatch() != nil && s.ExactPatch() == t.ExactPatch()
	case *geom.ExactTensorSurface:
		t, ok := b.(*geom.ExactTensorSurface)
		return ok && s.ExactPatch() != nil && s.ExactPatch() == t.ExactPatch()
	default:
		return false
	}
}

func assembleAlgebraicTrimmedSectionBranch(clips []SplineTubeClipReport, closed bool) (r AlgebraicTrimmedSectionBranch, err error) {
	r.Clips = clips
	var elements []AlgebraicTrimmedSectionInterval
	var retained []bool
	for segment, clip := range clips {
		if len(clip.Events) < 2 || len(clip.Intervals) != len(clip.Events)-1 {
			return r, geom.ErrConvergence
		}
		for event := range clip.Events {
			ref := AlgebraicTrimmedSectionEventRef{segment, event}
			if event == 0 && segment > 0 {
				n := &r.Nodes[len(r.Nodes)-1]
				n.Events = append(n.Events, ref)
			} else if closed && segment == len(clips)-1 && event == len(clip.Events)-1 {
				r.Nodes[0].Events = append(r.Nodes[0].Events, ref)
			} else {
				r.Nodes = append(r.Nodes, AlgebraicTrimmedSectionNode{Events: []AlgebraicTrimmedSectionEventRef{ref}})
			}
			if event != len(clip.Events)-1 {
				a := len(r.Nodes) - 1
				z := a + 1
				if closed && segment == len(clips)-1 && event == len(clip.Events)-2 {
					z = 0
				}
				elements = append(elements, AlgebraicTrimmedSectionInterval{segment, event, a, z})
				retained = append(retained, clip.Intervals[event].Retained)
			}
		}
	}
	type nodeEvidence struct {
		locations [2][2]trimvalidate.Location
		sides     int
		incident  bool
	}
	evidence := make([]nodeEvidence, len(r.Nodes))
	for i, e := range elements {
		loc := clips[e.Segment].Intervals[e.Interval].Locations
		for _, n := range [2]int{e.Start, e.End} {
			p := &evidence[n]
			if p.sides >= 2 {
				return r, geom.ErrConvergence
			}
			p.locations[p.sides] = loc
			p.sides++
			p.incident = p.incident || retained[i]
		}
	}
	for i := range r.Nodes {
		n, p := &r.Nodes[i], evidence[i]
		if p.sides == 0 {
			return r, geom.ErrConvergence
		}
		inside := [2]bool{}
		for chart := range inside {
			boundary := false
			for _, ref := range n.Events {
				boundary = boundary || len(clips[ref.Segment].Events[ref.Event].Contacts[chart]) > 0
			}
			if !boundary && p.sides == 2 && p.locations[0][chart] != p.locations[1][chart] {
				return r, geom.ErrConvergence
			}
			inside[chart] = boundary || closedFaceLocation(p.locations[0][chart])
		}
		n.OnFaces = inside[0] && inside[1]
		n.Isolated = n.OnFaces && !p.incident
		if p.incident && !n.OnFaces {
			return r, geom.ErrConvergence
		}
	}
	start := 0
	if closed {
		for start < len(elements) && retained[start] {
			start++
		}
		if start == len(elements) {
			r.Runs = []AlgebraicTrimmedSectionRun{{Intervals: elements, Closed: true}}
			return r, nil
		}
	}
	var run []AlgebraicTrimmedSectionInterval
	for k := range elements {
		i := (start + k) % len(elements)
		if retained[i] {
			run = append(run, elements[i])
		} else if len(run) > 0 {
			r.Runs = append(r.Runs, AlgebraicTrimmedSectionRun{Intervals: run})
			run = nil
		}
	}
	if len(run) > 0 {
		r.Runs = append(r.Runs, AlgebraicTrimmedSectionRun{Intervals: run})
	}
	return r, nil
}
