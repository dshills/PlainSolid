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
	set, err := ValidatePlanarInsetContactSet(ctx, host, tool, hostFace, []int{toolFace}, o)
	if err != nil {
		return PlanarInsetContact{}, err
	}
	return set.Tools[0], nil
}

// PlanarInsetContactSet proves a connected tool's disjoint complete disks
// meet the strict interior of one original host face. The disks retain
// independent whole perimeters and do not share vertices or edges.
type PlanarInsetContactSet struct {
	HostFace     int
	Tools        []PlanarInsetContact
	SignedVolume *big.Rat
	BroadPairs   int
}

// ValidatePlanarInsetContactSet proves all declared disks in one exact
// cross-boundary sweep. Other host faces, tool faces and material interiors
// cannot intersect.
func ValidatePlanarInsetContactSet(ctx context.Context, host, tool brep.Solid, hostFace int, toolFaces []int, o PlanarFaceContactOptions) (PlanarInsetContactSet, error) {
	var zero PlanarInsetContactSet
	if ctx == nil || len(toolFaces) == 0 || o.Cross.MaxPairs < 0 || o.Cross.MaxContacts < 0 || o.MaxPerimeterChecks < 0 {
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
	if hostFace < 0 || hostFace >= len(af) || len(toolFaces) > len(bf) {
		return zero, ErrInvalid
	}
	ae, be := host.Edges(), tool.Edges()
	h := af[hostFace]
	v, err := measurePlanarRegionFace(h, ae, am.Vertices, nil)
	if err != nil {
		return zero, err
	}
	start := func(e []brep.Edge, u brep.EdgeUse) int {
		p := e[u.Edge]
		if u.Reversed {
			return p.End
		}
		return p.Start
	}
	origin := exact(am.Vertices[start(ae, h.Boundary[0])])
	out := PlanarInsetContactSet{HostFace: hostFace, Tools: make([]PlanarInsetContact, len(toolFaces))}
	as, bs := make([]bool, len(af)), make([]bool, len(bf))
	as[hostFace] = true
	index := make(map[int]int, len(toolFaces))
	wires := make([][]point, len(toolFaces))
	seen := make(map[int]bool)
	totalArea := new(big.Rat)
	for i, id := range toolFaces {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		if id < 0 || id >= len(bf) || bs[id] || len(bf[id].Holes) != 0 {
			return zero, fmt.Errorf("%w: expected distinct whole original tool faces without holes", ErrInvalid)
		}
		bs[id] = true
		index[id] = i
		f := bf[id]
		w, err := measurePlanarRegionFace(f, be, bm.Vertices, nil)
		if err != nil {
			return zero, err
		}
		if v.drop != w.drop || !vertexParallelOpposite(v.normal, w.normal) || v.area.Cmp(w.area) <= 0 {
			return zero, fmt.Errorf("%w: expected strictly smaller oppositely oriented tool faces", ErrInvalid)
		}
		totalArea.Add(totalArea, w.area)
		q := PlanarInsetContact{HostFace: hostFace, ToolFace: id, DropAxis: v.drop, TwiceHostArea: new(big.Rat).Set(v.area), TwiceToolArea: w.area}
		for _, u := range f.Boundary {
			j := start(be, u)
			x := exact(bm.Vertices[j])
			if dot(minus(x, origin), v.normal).Sign() != 0 {
				return zero, fmt.Errorf("%w: tool face is not exactly on the original host support", ErrInvalid)
			}
			if seen[j] {
				return zero, fmt.Errorf("%w: inset disks must have disjoint original perimeters", ErrInvalid)
			}
			seen[j] = true
			wires[i] = append(wires[i], x)
			q.ToolVertices = append(q.ToolVertices, j)
			q.ToolEdges = append(q.ToolEdges, u.Edge)
		}
		out.Tools[i] = q
	}
	if totalArea.Cmp(v.area) >= 0 {
		return zero, fmt.Errorf("%w: inset disks consume the entire host face", ErrInvalid)
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
	area := make([]big.Rat, len(out.Tools))
	report, err := sweepIntersections(ctx, am, bm, false, o.Cross, func(i, j int, k Kind, ps []point) error {
		// Every host outer or inner wire also lies on another original
		// host face. Rejecting all such pairs proves a strictly interior
		// footprint without a distance tolerance.
		if m[i] != hostFace {
			return fmt.Errorf("%w: contact reaches outside the strict host face interior", ErrInvalid)
		}
		if id, selected := index[n[j]]; selected {
			switch k {
			case CoplanarOverlap:
				area[id].Add(&area[id], absoluteProjectedFaceArea2(ps, v.drop))
				return nil
			case Touching:
				return nil
			}
		}
		if k != Touching || len(ps) == 0 || len(ps) > 2 {
			return fmt.Errorf("%w: unexpected contact outside the original inset face", ErrInvalid)
		}
		for _, wire := range wires {
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
		}
		return fmt.Errorf("%w: tool side leaves the original inset perimeter", ErrInvalid)
	})
	if err != nil {
		return zero, err
	}
	if !report.Complete {
		return zero, ErrLimit
	}
	for i, p := range out.Tools {
		if area[i].Cmp(p.TwiceToolArea) != 0 {
			return zero, fmt.Errorf("%w: original tool face %d is not fully covered", ErrInvalid, p.ToolFace)
		}
	}
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
	for i := range out.Tools {
		out.Tools[i].SignedVolume = new(big.Rat).Set(out.SignedVolume)
		out.Tools[i].BroadPairs = out.BroadPairs
	}
	return out, nil
}

func vertexParallelOpposite(a, b point) bool {
	return nonzeroAxis(cross(a, b)) < 0 && dot(a, b).Sign() < 0
}
