package geom

import (
	"context"
	"errors"
	"math/big"
)

// WrappedSurfaceSeamJoin retains the exact evidence linking fundamental contour
// endpoints. End is 0 at the start, 1 at the end of that contour's traversal.
type WrappedSurfaceSeamJoin struct {
	Contours, Ends [2]int
	Axis           int
	// Certificate is reserved for optional rational endpoint materialization.
	// The assembler leaves it zero; RootCertificate or CornerCertificate carries the proof.
	Certificate     WrappedArcSeamJunction
	RootCertificate WrappedArcSeamRootJunction
	// CornerCertificate is used instead when Axis==2 (both seams).
	CornerCertificate WrappedArcCornerJunction
}

// WrappedSurfacePlaneTrace preserves fundamental isolation and join evidence.
// Contours reference Fundamental.Isolation.Arcs and may cross a periodic seam.
// Complete means covered fundamental tracing and unique transverse joins, not
// spatial injectivity. Fundamental may retain corner-only unresolved diagnostics
// when ResolvedFundamentalCorners proves those points already covered modulo seams.
// Components with any ambiguous endpoint are withheld.
type WrappedSurfacePlaneTrace struct {
	Fundamental                     SurfacePlaneTrace
	Contours                        []SurfaceContour
	SeamJoins                       []WrappedSurfaceSeamJoin
	Unresolved                      []SurfaceJoinIssue
	SeamComparisons, SeamOperations int
	Complete                        bool
	ResolvedFundamentalCorners      bool
}

func TraceWrappedSurfacePlaneContext(ctx context.Context, w *WrappedSurfacePatch, p Plane, o SurfaceTraceOptions) (WrappedSurfacePlaneTrace, error) {
	if w == nil || w.patch == nil {
		return WrappedSurfacePlaneTrace{}, ErrInvalid
	}
	f, e := TraceSurfacePlaneContext(ctx, w.patch, p, o)
	return assembleWrappedSurfaceTrace(ctx, w, f, e, o)
}
func TraceWrappedSurfaceRawPlaneContext(ctx context.Context, w *WrappedSurfacePatch, p RawSupportPlane, o SurfaceTraceOptions) (WrappedSurfacePlaneTrace, error) {
	if w == nil || w.patch == nil {
		return WrappedSurfacePlaneTrace{}, ErrInvalid
	}
	f, e := TraceSurfaceRawPlaneContext(ctx, w.patch, p, o)
	return assembleWrappedSurfaceTrace(ctx, w, f, e, o)
}

type wrappedTraceEnd struct {
	arc, end, axis int
	upper          bool
	point          SurfaceArcExactEndpoint
}

func assembleWrappedSurfaceTrace(ctx context.Context, w *WrappedSurfacePatch, f SurfacePlaneTrace, traceErr error, o SurfaceTraceOptions) (r WrappedSurfacePlaneTrace, err error) {
	r.Fundamental = f
	if o.MaxJoinComparisons == 0 {
		o.MaxJoinComparisons = 1000000
	}
	if o.MaxJoinOperations == 0 {
		o.MaxJoinOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxJoinComparisons < 1 || o.MaxJoinOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	if traceErr != nil || !f.Complete {
		if traceErr != nil && !errors.Is(traceErr, ErrConvergence) {
			return r, traceErr
		}
		ok, n, e := wrappedCornerContacts(ctx, w, f, o.MaxJoinComparisons)
		r.SeamComparisons += n
		if e != nil {
			return r, e
		}
		if !ok {
			return r, ErrConvergence
		}
		r.ResolvedFundamentalCorners = true
	}
	d := w.Domain()
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	ends := make([]wrappedTraceEnd, 2*len(f.Contours))
	links := make([]int, len(ends))
	bad := make([]bool, len(f.Contours))
	for i := range links {
		links[i] = -1
		ends[i].axis = -1
	}
	issue := func(i int, why string) {
		bad[i/2] = true
		r.Unresolved = append(r.Unresolved, SurfaceJoinIssue{Arc: ends[i].arc, Endpoint: ends[i].end, Reason: why})
	}
	for ci, c := range f.Contours {
		if c.Closed {
			continue
		}
		if len(c.Arcs) == 0 {
			return r, ErrInvalid
		}
		for side := 0; side < 2; side++ {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			a := c.Arcs[0]
			end := 0
			if side == 1 {
				a = c.Arcs[len(c.Arcs)-1]
				end = 1
			}
			if a.Reversed {
				end = 1 - end
			}
			i := ci*2 + side
			ends[i].arc, ends[i].end = a.Index, end
			id := f.Isolation.Arcs[a.Index].Endpoints[end].identity
			if id == nil {
				issue(i, "missing exact boundary root identity")
				continue
			}
			corner := false
			if w.period[0] != nil && w.period[1] != nil && id.lo.Cmp(id.hi) == 0 {
				fixedBoundary := id.fixed.Cmp(new(big.Rat).SetFloat64(limits[id.axis][0])) == 0 || id.fixed.Cmp(new(big.Rat).SetFloat64(limits[id.axis][1])) == 0
				variableBoundary := id.lo.Cmp(new(big.Rat).SetFloat64(limits[1-id.axis][0])) == 0 || id.lo.Cmp(new(big.Rat).SetFloat64(limits[1-id.axis][1])) == 0
				corner = fixedBoundary && variableBoundary
			}
			if corner {
				ends[i].axis = 2
				continue
			}
			for axis := 0; axis < 2; axis++ {
				if w.period[axis] == nil {
					continue
				}
				for side, bound := range limits[axis] {
					q := new(big.Rat).SetFloat64(bound)
					if id.axis == axis && id.fixed.Cmp(q) == 0 {
						ends[i].axis = axis
						ends[i].upper = side == 1
					}
					if id.axis != axis && id.lo.Cmp(q) <= 0 && id.hi.Cmp(q) >= 0 {
						if id.lo.Cmp(id.hi) == 0 {
							ends[i].axis = axis
							ends[i].upper = side == 1
						} else {
							issue(i, "possible wrapped corner contact")
						}
					}
				}
			}
		}
	}
	for i, a := range ends {
		if a.axis < 0 || bad[i/2] {
			continue
		}
		for j := i + 1; j < len(ends); j++ {
			b := ends[j]
			if b.axis != a.axis || (a.axis != 2 && a.upper == b.upper) || bad[j/2] {
				continue
			}
			if e := ctx.Err(); e != nil {
				return r, e
			}
			r.SeamComparisons++
			if r.SeamComparisons > o.MaxJoinComparisons {
				return r, ErrConvergence
			}
			remaining := o.MaxJoinOperations - r.SeamOperations
			if remaining < 1 {
				return r, ErrConvergence
			}
			var root WrappedArcSeamRootJunction
			var corner WrappedArcCornerJunction
			var e error
			matched, transverse := false, false
			if a.axis == 2 {
				corner, e = w.CompareArcCornerEndpoints(ctx, f.Isolation.Arcs[a.arc], a.end, f.Isolation.Arcs[b.arc], b.end, ArcExactEndpointOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
				r.SeamOperations += corner.ExactOperations
				matched, transverse = corner.Matched, corner.TransverseContinuation
			} else {
				root, e = w.CompareArcSeamRoots(ctx, f.Isolation.Arcs[a.arc], a.end, f.Isolation.Arcs[b.arc], b.end, a.axis, ArcExactEndpointOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
				r.SeamOperations += root.ExactOperations
				if e == nil && root.DistinctParameters {
					continue
				}
				matched, transverse = root.Matched, root.TransverseContinuation
			}
			if e != nil || !matched || !transverse {
				issue(i, "seam continuation unproved")
				issue(j, "seam continuation unproved")
				continue
			}

			if links[i] >= 0 || links[j] >= 0 {
				issue(i, "multiple seam continuations")
				issue(j, "multiple seam continuations")
				if links[i] >= 0 {
					issue(links[i], "multiple seam continuations")
				}
				if links[j] >= 0 {
					issue(links[j], "multiple seam continuations")
				}
				continue
			}
			links[i], links[j] = j, i
			r.SeamJoins = append(r.SeamJoins, WrappedSurfaceSeamJoin{Contours: [2]int{i / 2, j / 2}, Ends: [2]int{i % 2, j % 2}, Axis: a.axis, RootCertificate: root, CornerCertificate: corner})
		}
	}
	for i, a := range ends {
		if a.axis >= 0 && links[i] < 0 && !bad[i/2] {
			issue(i, "missing opposite seam endpoint")
		}
	}
	// Walk components through their unique endpoint links. Prefer an unlinked
	// endpoint for an open component, otherwise close a cycle deterministically.
	visited := make([]bool, len(f.Contours))
	for ci, c := range f.Contours {
		if visited[ci] {
			continue
		}
		if c.Closed {
			visited[ci] = true
			r.Contours = append(r.Contours, c)
			continue
		}
		component := []int{ci}
		seen := map[int]bool{ci: true}
		invalid := false
		start := ci * 2
		for k := 0; k < len(component); k++ {
			q := component[k]
			invalid = invalid || bad[q]
			for side := 0; side < 2; side++ {
				j := links[q*2+side]
				if j < 0 {
					start = q*2 + side
				} else if !seen[j/2] {
					seen[j/2] = true
					component = append(component, j/2)
				}
			}
		}
		for _, q := range component {
			visited[q] = true
		}
		if invalid {
			continue
		}
		out := SurfaceContour{}
		current := start
		for steps := 0; steps <= len(component); steps++ {
			q, reverse := current/2, current%2 == 1
			chain := f.Contours[q].Arcs
			if reverse {
				for k := len(chain) - 1; k >= 0; k-- {
					a := chain[k]
					a.Reversed = !a.Reversed
					out.Arcs = append(out.Arcs, a)
				}
			} else {
				out.Arcs = append(out.Arcs, chain...)
			}
			next := links[current^1]
			if next < 0 {
				break
			}
			if next == start {
				out.Closed = true
				break
			}
			current = next
			if steps == len(component) {
				return r, ErrConvergence
			}
		}
		r.Contours = append(r.Contours, out)
	}
	r.Complete = len(r.Unresolved) == 0
	if !r.Complete {
		return r, ErrConvergence
	}
	return r, nil
}
