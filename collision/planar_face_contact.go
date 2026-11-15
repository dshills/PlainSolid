package collision

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/geom"
)

// PlanarFaceContactOptions bounds local embedding, cross-boundary tests,
// and checks against the declared original perimeter.
type PlanarFaceContactOptions struct {
	Validation LocationOptions
	Cross      Options
	// Zero MaxPerimeterChecks selects one million comparisons.
	MaxPerimeterChecks int
}

// PlanarFaceContact is an exact full-face incidence proof between two
// separate materials. The edge and vertex pairs follow the first face's
// outer boundary. Each pair's entries are original IDs in the two bodies.
type PlanarFaceContact struct {
	FaceA, FaceB       int
	Vertices, Edges    [][2]int
	DropAxis           int
	TwiceProjectedArea *big.Rat
	SignedVolume       *big.Rat
	BroadPairs         int
}

// ValidatePlanarFaceContact proves that the only contact between two
// outward single-shell parts is precisely the specified matching original
// face. Concave faces are supported. The two face cycles must have opposite
// orientation, complete identical boundaries and no inner wires.
func ValidatePlanarFaceContact(ctx context.Context, a, b brep.Solid, faceA, faceB int, o PlanarFaceContactOptions) (PlanarFaceContact, error) {
	var zero PlanarFaceContact
	if ctx == nil || o.Cross.MaxContacts < 0 || o.Cross.MaxPairs < 0 || o.MaxPerimeterChecks < 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	options, err := locationDefaults(o.Validation)
	if err != nil {
		return zero, err
	}
	af, bf := a.Faces(), b.Faces()
	if faceA < 0 || faceA >= len(af) || faceB < 0 || faceB >= len(bf) {
		return zero, ErrInvalid
	}
	fa, fb := af[faceA], bf[faceB]
	if len(fa.Holes) != 0 || len(fb.Holes) != 0 || len(fa.Boundary) < 3 || len(fa.Boundary) != len(fb.Boundary) {
		return zero, fmt.Errorf("%w: expected matching complete perimeter cycles without inner wires", ErrInvalid)
	}
	av, bv := a.Vertices(), b.Vertices()
	ae, be := a.Edges(), b.Edges()
	start := func(u brep.EdgeUse, edges []brep.Edge) int {
		e := edges[u.Edge]
		if u.Reversed {
			return e.End
		}
		return e.Start
	}
	first := av[start(fa.Boundary[0], ae)]
	at := -1
	for i, u := range fb.Boundary {
		if bv[start(u, be)] == first {
			at = i
			break
		}
	}
	if at < 0 {
		return zero, fmt.Errorf("%w: original perimeters differ", ErrInvalid)
	}
	n := len(fa.Boundary)
	perimeter := make([]point, n)
	vertices, edges := make([][2]int, n), make([][2]int, n)
	skipA, skipB := make(map[int]bool, n), make(map[int]bool, n)
	for i, u := range fa.Boundary {
		j := (at + n - i) % n
		va, vb := start(u, ae), start(fb.Boundary[j], be)
		if av[va] != bv[vb] {
			return zero, fmt.Errorf("%w: perimeters must match with opposite directions", ErrInvalid)
		}
		vertices[i], edges[i] = [2]int{va, vb}, [2]int{u.Edge, fb.Boundary[(j+n-1)%n].Edge}
		perimeter[i] = exact(av[va])
		skipA[va], skipB[vb] = true, true
	}
	normal := point{new(big.Rat), new(big.Rat), new(big.Rat)}
	for i := 1; i+1 < len(perimeter); i++ {
		normal = cross(minus(perimeter[i], perimeter[0]), minus(perimeter[i+1], perimeter[0]))
		if nonzeroAxis(normal) >= 0 {
			break
		}
	}
	drop := nonzeroAxis(normal)
	if drop < 0 {
		return zero, fmt.Errorf("%w: original face has no area", ErrInvalid)
	}
	for _, p := range perimeter {
		if dot(minus(p, perimeter[0]), normal).Sign() != 0 {
			return zero, fmt.Errorf("%w: original common face is not exactly planar", ErrInvalid)
		}
	}
	wantArea := absoluteProjectedFaceArea2(perimeter, drop)
	if wantArea.Sign() == 0 {
		return zero, fmt.Errorf("%w: original common face has zero projected area", ErrInvalid)
	}
	ma, mb := a.Tessellate(), b.Tessellate()
	if len(ma.Triangles) > options.MaxTriangles-len(mb.Triangles) {
		return zero, ErrLimit
	}
	am, bm := a.TriangleFaces(), b.TriangleFaces()
	la, err := NewMeshPointLocator(ctx, ma, am, options)
	if err != nil {
		return zero, err
	}
	lb, err := NewMeshPointLocator(ctx, mb, bm, options)
	if err != nil {
		return zero, err
	}
	if len(la.shells) != 1 || len(lb.shells) != 1 {
		return zero, fmt.Errorf("%w: original face contact requires two single-shell parts", ErrInvalid)
	}
	remaining := o.MaxPerimeterChecks
	if remaining == 0 {
		remaining = DefaultMaxPairs
	}
	haveArea := new(big.Rat)
	crossing, err := sweepIntersections(ctx, ma, mb, false, o.Cross, func(i, j int, k Kind, ps []point) error {
		onFaces := am[i] == faceA && bm[j] == faceB
		if k == CoplanarOverlap && onFaces {
			haveArea.Add(haveArea, absoluteProjectedFaceArea2(ps, drop))
			return nil
		}
		if k != Touching || len(ps) == 0 || len(ps) > 2 {
			return fmt.Errorf("%w: intersection outside the declared common face", ErrInvalid)
		}
		if onFaces {
			return nil
		}
		for k, p := range perimeter {
			if remaining == 0 {
				return ErrLimit
			}
			remaining--
			q := perimeter[(k+1)%n]
			if onSegment(ps[0], p, q) && (len(ps) == 1 || onSegment(ps[1], p, q)) {
				return nil
			}
		}
		return fmt.Errorf("%w: side contact leaves the original common perimeter", ErrInvalid)
	})
	if err != nil {
		return zero, err
	}
	if !crossing.Complete {
		return zero, ErrLimit
	}
	if haveArea.Cmp(wantArea) != 0 {
		return zero, fmt.Errorf("%w: triangle overlaps do not cover the complete original face", ErrInvalid)
	}
	boxA, err := geom.Bounds(av)
	if err != nil {
		return zero, err
	}
	boxB, err := geom.Bounds(bv)
	if err != nil {
		return zero, err
	}
	ca, cb := vertexContactComponent{ma, boxA, la}, vertexContactComponent{mb, boxB, lb}
	// The complement of a complete embedded closed disk in a connected
	// boundary stays connected, so one noncontact witness per part suffices.
	if err := validateComponentOutside(ctx, ca, cb, skipA); err != nil {
		return zero, err
	}
	if err := validateComponentOutside(ctx, cb, ca, skipB); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return PlanarFaceContact{
		FaceA: faceA, FaceB: faceB, Vertices: vertices, Edges: edges, DropAxis: drop,
		TwiceProjectedArea: wantArea, SignedVolume: add(la.shells[0].SignedVolume, lb.shells[0].SignedVolume),
		BroadPairs: crossing.BroadPhasePairs,
	}, nil
}

func absoluteProjectedFaceArea2(p []point, drop int) *big.Rat {
	area := new(big.Rat)
	for i := 1; i+1 < len(p); i++ {
		area.Add(area, orient(p[0], p[i], p[i+1], drop))
	}
	return area.Abs(area)
}
