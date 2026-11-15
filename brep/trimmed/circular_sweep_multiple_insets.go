package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// CircularSweepInsetGuest identifies a guest's original topology in the
// result. The guest cap used by the join has face index -1.
type CircularSweepInsetGuest struct {
	Faces                    []int
	EdgeOffset, VertexOffset int
	HostCap, GuestCap        int
}

// CircularSweepInsets preserves all original source edges and vertices in
// operand order. HostFaces keeps each original host face and the separate
// floors exposed by through-openings on either cap.
type CircularSweepInsets struct {
	Solid     Solid
	HostFaces [][]int
	Guests    []CircularSweepInsetGuest
}

const maxCircularSweepInsetGuests = 16
const maxCircularSweepInsetSegments = 256

func checkCircularSweepInsetSegments(host ExactCircularSweep, guests []ExactCircularSweep, limit int) error {
	total := 0
	for _, sweep := range append([]ExactCircularSweep{host}, guests...) {
		for _, ring := range append([][]ExactCircularLoopSegment{sweep.Outer}, sweep.Holes...) {
			if len(ring) > limit-total {
				return geom.ErrConvergence
			}
			total += len(ring)
		}
	}
	return nil
}

// JoinExactCircularSweepInsets joins up to sixteen exact circular sweeps to
// either cap of one host, with up to 256 total original profile segments.
// All guest footprints on a cap must lie strictly inside the host material,
// separated from each other and from the original host holes. Guest holes
// expose independent floors on the corresponding original host cap.
func JoinExactCircularSweepInsets(ctx context.Context, host ExactCircularSweep, guests []ExactCircularSweep, o ExactCircularLoopOptions) (CircularSweepInsets, error) {
	var zero CircularSweepInsets
	if ctx == nil || len(guests) == 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if len(guests) > maxCircularSweepInsetGuests {
		return zero, geom.ErrConvergence
	}
	if err := checkCircularSweepInsetSegments(host, guests, maxCircularSweepInsetSegments); err != nil {
		return zero, err
	}
	members := make([]CircularSweepInsetGuest, len(guests))
	for i, g := range guests {
		switch {
		case host.Top == g.Bottom:
			members[i].HostCap = 1
		case host.Bottom == g.Top:
			members[i].GuestCap = 1
		default:
			return zero, fmt.Errorf("trimmed: sweep inset requires exactly abutting caps at guest %d: %w", i, geom.ErrDomain)
		}
	}
	h, err := ExtrudeExactDirectedCircularLoops(ctx, host.Outer, host.Holes, host.Bottom, host.Top, o)
	if err != nil {
		return zero, fmt.Errorf("trimmed: sweep inset host: %w", err)
	}
	vertices, edges, faces := h.Vertices(), h.Edges(), h.Faces()
	hostCaps := [2]Face{faces[0], faces[1]}
	changed := [2]bool{}
	out := CircularSweepInsets{HostFaces: make([][]int, len(faces)), Guests: members}
	for i := range out.HostFaces {
		out.HostFaces[i] = []int{i}
	}
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
	for i, input := range guests {
		g, err := ExtrudeExactDirectedCircularLoops(ctx, input.Outer, input.Holes, input.Bottom, input.Top, o)
		if err != nil {
			return zero, fmt.Errorf("trimmed: sweep inset guest %d: %w", i, err)
		}
		member := &out.Guests[i]
		member.VertexOffset, member.EdgeOffset = len(vertices), len(edges)
		member.Faces = make([]int, len(g.faces))
		vertices = append(vertices, g.vertices...)
		for _, edge := range g.edges {
			edges = append(edges, parametric.Edge{Start: edge.Start + member.VertexOffset, End: edge.End + member.VertexOffset, Curve: edge.Curve, Range: edge.Range})
		}
		remap := func(loop []Use, reverse bool) []Use {
			result := make([]Use, len(loop))
			for i, u := range loop {
				u.Edge += member.EdgeOffset
				j := i
				if reverse {
					j = len(loop) - 1 - i
					u.Reversed = !u.Reversed
					u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
					u.Curve = reversePolynomialPlaneCurve(u.Curve)
				}
				result[j] = u
			}
			return result
		}
		cutout := g.faces[member.GuestCap]
		cap := member.HostCap
		changed[cap] = true
		faces[cap].Holes = append(faces[cap].Holes, remap(cutout.Outer, true))
		for j, original := range g.faces {
			if j == member.GuestCap {
				member.Faces[j] = -1
				continue
			}
			member.Faces[j] = len(faces)
			f := Face{Surface: original.Surface, Outer: remap(original.Outer, false), Reversed: original.Reversed}
			for _, ring := range original.Holes {
				f.Holes = append(f.Holes, remap(ring, false))
			}
			faces = append(faces, f)
		}
		for _, ring := range cutout.Holes {
			f := Face{Surface: hostCaps[cap].Surface, Outer: remap(ring, true), Reversed: hostCaps[cap].Reversed}
			if err := verify(f); err != nil {
				return zero, err
			}
			out.HostFaces[cap] = append(out.HostFaces[cap], len(faces))
			faces = append(faces, f)
		}
	}
	for cap, check := range changed {
		if check {
			if err := verify(faces[cap]); err != nil {
				return zero, err
			}
		}
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
	out.Solid = solid
	return out, nil
}
