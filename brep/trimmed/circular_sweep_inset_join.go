package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// CircularSweepInset retains both inputs' original edge and vertex order.
// GuestEdgeOffset and GuestVertexOffset locate guest topology in Solid.
// HostFaces maps each original host face to all its surviving fragments;
// GuestFaces maps each original guest face, or -1 for its consumed cap.
type CircularSweepInset struct {
	Solid                              Solid
	HostFaces                          [][]int
	GuestFaces                         []int
	GuestEdgeOffset, GuestVertexOffset int
	HostCap, GuestCap                  int
}

// JoinExactCircularSweepInset forms the exact union of two positive
// extrusions at a shared cap elevation. The guest's outer wire must lie
// strictly inside the host material, clear of host holes. Guest holes expose
// new faces on the original host cap support. A direction-independent exact
// trim proof certifies the complete interface before the union is published.
func JoinExactCircularSweepInset(ctx context.Context, host, guest ExactCircularSweep, o ExactCircularLoopOptions) (CircularSweepInset, error) {
	var zero CircularSweepInset
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	var hostCap, guestCap int
	switch {
	case host.Top == guest.Bottom:
		hostCap, guestCap = 1, 0
	case host.Bottom == guest.Top:
		hostCap, guestCap = 0, 1
	default:
		return zero, fmt.Errorf("trimmed: sweep inset requires exactly abutting caps: %w", geom.ErrDomain)
	}
	h, err := ExtrudeExactDirectedCircularLoops(ctx, host.Outer, host.Holes, host.Bottom, host.Top, o)
	if err != nil {
		return zero, fmt.Errorf("trimmed: sweep inset host: %w", err)
	}
	g, err := ExtrudeExactDirectedCircularLoops(ctx, guest.Outer, guest.Holes, guest.Bottom, guest.Top, o)
	if err != nil {
		return zero, fmt.Errorf("trimmed: sweep inset guest: %w", err)
	}

	vOffset, eOffset := len(h.vertices), len(h.edges)
	vertices := append(h.Vertices(), g.vertices...)
	edges := append(h.Edges(), make([]parametric.Edge, len(g.edges))...)
	for i, edge := range g.edges {
		edge.Start += vOffset
		edge.End += vOffset
		edges[eOffset+i] = edge
	}
	remap := func(loop []Use, reverse bool) []Use {
		out := make([]Use, len(loop))
		for i, u := range loop {
			u.Edge += eOffset
			j := i
			if reverse {
				j = len(loop) - 1 - i
				u.Reversed = !u.Reversed
				u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
				// The sweep only generates linear and exact rational
				// quadratic cap pcurves. Reversal preserves those forms.
				u.Curve = reversePolynomialPlaneCurve(u.Curve)
			}
			out[j] = u
		}
		return out
	}
	faces := h.Faces()
	originalHostCap := faces[hostCap]
	consumedGuestCap := g.faces[guestCap]
	faces[hostCap].Holes = append(faces[hostCap].Holes, remap(consumedGuestCap.Outer, true))
	verify := func(f Face) error {
		proof, err := ValidateFaceTrimLoops(ctx, f, o.Trims)
		if err != nil {
			return err
		}
		if proof.Status == trimvalidate.Certified {
			return nil
		}
		cause := geom.ErrConvergence
		if proof.Status == trimvalidate.Invalid {
			cause = geom.ErrDomain
		}
		return fmt.Errorf("trimmed: unproved sweep inset cap (%s): %w", proof.Reason, cause)
	}
	// A negative exterior wire must be strictly contained and disjoint
	// from every existing host hole. This also excludes nested host holes.
	if err := verify(faces[hostCap]); err != nil {
		return zero, err
	}
	result := CircularSweepInset{
		HostFaces: make([][]int, len(h.faces)), GuestFaces: make([]int, len(g.faces)),
		GuestEdgeOffset: eOffset, GuestVertexOffset: vOffset,
		HostCap: hostCap, GuestCap: guestCap,
	}
	for i := range result.HostFaces {
		result.HostFaces[i] = []int{i}
	}
	for i, original := range g.faces {
		if i == guestCap {
			result.GuestFaces[i] = -1
			continue
		}
		result.GuestFaces[i] = len(faces)
		f := Face{Surface: original.Surface, Outer: remap(original.Outer, false), Reversed: original.Reversed}
		for _, ring := range original.Holes {
			f.Holes = append(f.Holes, remap(ring, false))
		}
		faces = append(faces, f)
	}
	for _, ring := range consumedGuestCap.Holes {
		// The host reaches this cap plane under the opening, so every
		// original guest hole supplies an independent exposed floor wire.
		f := Face{Surface: originalHostCap.Surface, Outer: remap(ring, true), Reversed: originalHostCap.Reversed}
		if err := verify(f); err != nil {
			return zero, err
		}
		result.HostFaces[hostCap] = append(result.HostFaces[hostCap], len(faces))
		faces = append(faces, f)
	}
	solid, err := NewSolidContext(ctx, vertices, edges, faces, h.tolerance)
	if err != nil {
		return zero, err
	}
	proof, err := solid.ValidateEdgeAgreement(ctx, o.Edges)
	if err != nil {
		return zero, err
	}
	if !proof.Complete {
		return zero, fmt.Errorf("trimmed: unresolved sweep inset coedges: %w", geom.ErrConvergence)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	result.Solid = solid
	return result, nil
}
