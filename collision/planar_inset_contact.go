package collision

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
)

// PlanarInsetContact proves that exactly one whole original tool face meets
// the strict interior of a larger original host face. Tool perimeter IDs
// are in original wire order and remain the only seam after attachment.
type PlanarInsetContact struct {
	HostFace, ToolFace           int
	ToolVertices, ToolEdges      []int
	DropAxis                     int
	TwiceHostArea, TwiceToolArea *big.Rat
	SignedVolume                 *big.Rat
	BroadPairs                   int
}

// ValidatePlanarInsetContact accepts only a complete tool disk strictly
// inside one host face, with opposite orientation and no other contacts.
func ValidatePlanarInsetContact(ctx context.Context, host, tool brep.Solid, hostFace, toolFace int, o PlanarFaceContactOptions) (PlanarInsetContact, error) {
	var zero PlanarInsetContact
	if ctx == nil || o.Cross.MaxPairs < 0 || o.Cross.MaxContacts < 0 || o.MaxPerimeterChecks < 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	options, err := locationDefaults(o.Validation)
	if err != nil {
		return zero, err
	}
	am, bm := host.Tessellate(), tool.Tessellate()
	if len(am.Triangles) > options.MaxTriangles-len(bm.Triangles) {
		return zero, ErrLimit
	}
	af, bf := host.Faces(), tool.Faces()
	if hostFace < 0 || hostFace >= len(af) || toolFace < 0 || toolFace >= len(bf) || len(bf[toolFace].Holes) != 0 {
		return zero, fmt.Errorf("%w: expected a complete original tool face without holes", ErrInvalid)
	}
	ae, be := host.Edges(), tool.Edges()
	h, t := af[hostFace], bf[toolFace]
	v, err := measurePlanarRegionFace(h, ae, am.Vertices, nil)
	if err != nil {
		return zero, err
	}
	w, err := measurePlanarRegionFace(t, be, bm.Vertices, nil)
	if err != nil {
		return zero, err
	}
	if v.drop != w.drop || !vertexParallelOpposite(v.normal, w.normal) || v.area.Cmp(w.area) <= 0 {
		return zero, fmt.Errorf("%w: expected a strictly smaller oppositely oriented tool face", ErrInvalid)
	}
	start := func(e []brep.Edge, u brep.EdgeUse) int {
		p := e[u.Edge]
		if u.Reversed {
			return p.End
		}
		return p.Start
	}
	origin := exact(am.Vertices[start(ae, h.Boundary[0])])
	out := PlanarInsetContact{HostFace: hostFace, ToolFace: toolFace, DropAxis: v.drop, TwiceHostArea: v.area, TwiceToolArea: w.area}
	wire := make([]point, len(t.Boundary))
	for i, u := range t.Boundary {
		j := start(be, u)
		q := exact(bm.Vertices[j])
		if dot(minus(q, origin), v.normal).Sign() != 0 {
			return zero, fmt.Errorf("%w: tool face is not exactly on the original host support", ErrInvalid)
		}
		wire[i] = q
		out.ToolVertices = append(out.ToolVertices, j)
		out.ToolEdges = append(out.ToolEdges, u.Edge)
	}
	m, n := host.TriangleFaces(), tool.TriangleFaces()
	al, err := NewMeshPointLocator(ctx, am, m, options)
	if err != nil {
		return zero, err
	}
	bl, err := NewMeshPointLocator(ctx, bm, n, options)
	if err != nil {
		return zero, err
	}
	if len(al.shells) != 1 || len(bl.shells) != 1 {
		return zero, fmt.Errorf("%w: inset contact requires two single-shell parts", ErrInvalid)
	}
	remaining := o.MaxPerimeterChecks
	if remaining == 0 {
		remaining = DefaultMaxPairs
	}
	area := new(big.Rat)
	report, err := sweepIntersections(ctx, am, bm, false, o.Cross, func(i, j int, k Kind, ps []point) error {
		// Every host outer or inner wire also lies on another original
		// host face. Rejecting all such pairs proves a strictly interior
		// footprint without a distance tolerance.
		if m[i] != hostFace {
			return fmt.Errorf("%w: contact reaches outside the strict host face interior", ErrInvalid)
		}
		if n[j] == toolFace {
			switch k {
			case CoplanarOverlap:
				area.Add(area, absoluteProjectedFaceArea2(ps, w.drop))
				return nil
			case Touching:
				return nil
			}
		}
		if k != Touching || len(ps) == 0 || len(ps) > 2 {
			return fmt.Errorf("%w: unexpected contact outside the original inset face", ErrInvalid)
		}
		for j, p := range wire {
			if remaining == 0 {
				return ErrLimit
			}
			remaining--
			q := wire[(j+1)%len(wire)]
			if onSegment(ps[0], p, q) && (len(ps) == 1 || onSegment(ps[1], p, q)) {
				return nil
			}
		}
		return fmt.Errorf("%w: tool side leaves the original inset perimeter", ErrInvalid)
	})
	if err != nil {
		return zero, err
	}
	if !report.Complete {
		return zero, ErrLimit
	}
	if area.Cmp(w.area) != 0 {
		return zero, fmt.Errorf("%w: original tool face is not fully covered", ErrInvalid)
	}
	as, bs := make([]bool, len(af)), make([]bool, len(bf))
	as[hostFace], bs[toolFace] = true, true
	if err := faceContactExterior(ctx, al, bl, af, len(ae), as, options.MaxWork); err != nil {
		return zero, err
	}
	if err := faceContactExterior(ctx, bl, al, bf, len(be), bs, options.MaxWork); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.BroadPairs = report.BroadPhasePairs
	out.SignedVolume = add(al.shells[0].SignedVolume, bl.shells[0].SignedVolume)
	return out, nil
}

func vertexParallelOpposite(a, b point) bool {
	return nonzeroAxis(cross(a, b)) < 0 && dot(a, b).Sign() < 0
}
