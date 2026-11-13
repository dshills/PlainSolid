package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

type axialWall struct {
	operand, lo, hi int
	reversed        bool
	face, edge      int
}
type axialRim struct{ operand, height int }
type axialCap struct {
	height, lo, hi int // lo=-1 denotes a complete disk.
	reversed       bool
}

// combineSteppedWrappedCylinders applies a radial/axial material arrangement
// to two homothetic coaxial operands. Each cell is the product of a height
// interval and either an inner disk or its surrounding annulus. Only
// interfaces across different material states are reconstructed. A rim
// supporting more than one wall on either side is a nonmanifold contact.
func combineSteppedWrappedCylinders(ctx context.Context, a, b Solid, supports [2]*geom.WrappedSurfacePatch,
	proof [2]geom.WrappedCylinderEmbedding, op parametric.BooleanOp, o geom.SurfacePatchOptions) (Solid, SplineShellCombineHistory, error) {
	var history SplineShellCombineHistory
	same, e := sameNormalizedCylinderProfile(ctx, supports[0], supports[1], proof[0], proof[1], o)
	if e != nil || !same {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, history, e
	}
	heights := []*big.Rat{proof[0].LowerZ, proof[0].UpperZ, proof[1].LowerZ, proof[1].UpperZ}
	sort.Slice(heights, func(i, j int) bool { return heights[i].Cmp(heights[j]) < 0 })
	cuts := heights[:0]
	for _, z := range heights {
		if len(cuts) == 0 || cuts[len(cuts)-1].Cmp(z) != 0 {
			cuts = append(cuts, z)
		}
	}
	small, large := 0, 1
	if proof[0].Radius.Cmp(proof[1].Radius) > 0 {
		small, large = 1, 0
	}
	region := func(layer, ring int) bool {
		if layer < 0 || layer+1 >= len(cuts) {
			return false
		}
		active := [2]bool{}
		for i, p := range proof {
			active[i] = (ring == 0 || i == large) && p.LowerZ.Cmp(cuts[layer]) <= 0 && p.UpperZ.Cmp(cuts[layer+1]) >= 0
		}
		switch op {
		case parametric.Union:
			return active[0] || active[1]
		case parametric.Intersection:
			return active[0] && active[1]
		default:
			return active[0] && !active[1]
		}
	}
	var walls []axialWall
	for _, side := range []int{small, large} {
		active := func(i int) (int, bool) {
			left, right := region(i, 0), region(i, 1)
			if side == large {
				left, right = right, false
			}
			if left == right {
				return 0, false
			}
			if left {
				return 1, true
			}
			return -1, true
		}
		for i := 0; i+1 < len(cuts); {
			sign, ok := active(i)
			if !ok {
				i++
				continue
			}
			j := i + 1
			for j+1 < len(cuts) {
				if next, _ := active(j); next != sign {
					break
				}
				j++
			}
			walls = append(walls, axialWall{operand: side, lo: i, hi: j, reversed: sign < 0})
			i = j
		}
	}
	var caps []axialCap
	for i := range cuts {
		changes := [2]int{}
		for r := range 2 {
			below, above := region(i-1, r), region(i, r)
			if below && !above {
				changes[r] = 1
			} else if above && !below {
				changes[r] = -1
			}
		}
		if changes[0] != 0 && changes[1] != 0 && changes[0] != changes[1] {
			return Solid{}, history, geom.ErrDegenerate
		}
		if changes[0] != 0 && changes[0] == changes[1] {
			caps = append(caps, axialCap{i, -1, large, changes[0] < 0})
		} else if changes[0] != 0 {
			caps = append(caps, axialCap{i, -1, small, changes[0] < 0})
		} else if changes[1] != 0 {
			caps = append(caps, axialCap{i, small, large, changes[1] < 0})
		}
	}
	tol := max(a.tolerance, b.tolerance)
	var vertices []geom.Vec3
	var edges []parametric.Edge
	var faces []Face
	rims := map[axialRim][]Use{}
	for i, w := range walls {
		support, e := geom.ReheightWrappedCylinder(ctx, supports[w.operand], cuts[w.lo], cuts[w.hi], o)
		if e != nil {
			return Solid{}, history, e
		}
		s, _, e := NewWrappedCylinder(ctx, support, tol, o)
		if e != nil {
			return Solid{}, history, e
		}
		nv, ne := len(vertices), len(edges)
		vertices = append(vertices, s.vertices...)
		for _, edge := range s.edges {
			edge.Start += nv
			edge.End += nv
			edges = append(edges, edge)
		}
		w.face, w.edge = len(faces), ne+8
		walls[i] = w
		f := cloneFace(s.faces[0])
		f.Reversed = w.reversed
		for j := range f.Outer {
			f.Outer[j].Edge += ne
		}
		faces = append(faces, f)
		for j, height := range []int{w.lo, w.hi} {
			key := axialRim{w.operand, height}
			if _, duplicate := rims[key]; duplicate {
				return Solid{}, history, geom.ErrDegenerate
			}
			u := append([]Use(nil), s.faces[j+1].Outer...)
			for k := range u {
				u[k].Edge += ne
			}
			rims[key] = u
		}
	}
	capFaces := make([]int, len(caps))
	consumed := map[axialRim]bool{}
	for i, c := range caps {
		take := func(operand int) ([]Use, bool) {
			key := axialRim{operand, c.height}
			r, ok := rims[key]
			if !ok || consumed[key] {
				return nil, false
			}
			consumed[key] = true
			return r, true
		}
		outer, ok := take(c.hi)
		if !ok {
			return Solid{}, history, geom.ErrConvergence
		}
		z, e := geom.RepresentedExactScalar(cuts[c.height])
		if e != nil {
			return Solid{}, history, e
		}
		frame, e := geom.NewFrame(geom.Vec3{Z: z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		if e != nil {
			return Solid{}, history, e
		}
		plane, e := geom.NewPlaneSurface(frame)
		if e != nil {
			return Solid{}, history, e
		}
		f := Face{Surface: plane, Reversed: c.reversed, Outer: outer}
		if c.lo >= 0 {
			hole, ok := take(c.lo)
			if !ok {
				return Solid{}, history, geom.ErrConvergence
			}
			rev := make([]Use, len(hole))
			for j, u := range hole {
				rev[len(hole)-1-j] = Use{Edge: u.Edge, Reversed: !u.Reversed, UVStart: u.UVEnd, UVEnd: u.UVStart, Curve: reversePCurve(u.Curve)}
			}
			f.Holes = [][]Use{rev}
		}
		p, e := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
		if e != nil || p.Status != trimvalidate.Certified {
			if e == nil {
				e = geom.ErrConvergence
			}
			return Solid{}, history, e
		}
		capFaces[i] = len(faces)
		faces = append(faces, f)
	}
	if len(consumed) != len(rims) {
		return Solid{}, history, geom.ErrConvergence
	}
	out, e := NewSolidContext(ctx, vertices, edges, faces, tol)
	if e != nil {
		return Solid{}, history, e
	}
	p, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil || !p.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, history, e
	}
	history = SplineShellCombineHistory{Method: "stepped-wrapped-cylinder",
		FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)),
		EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(b.edges))}
	for oi, input := range [2]Solid{a, b} {
		fr, er := [2][][]int{history.FacesA, history.FacesB}[oi], [2][][]int{history.EdgesA, history.EdgesB}[oi]
		for i, f := range input.faces {
			if f.Surface == supports[oi] {
				for _, wall := range walls {
					if wall.operand == oi {
						fr[i] = append(fr[i], wall.face)
					}
				}
				continue
			}
			z := ratInset(f.Surface.(geom.PlaneSurface).Frame().Origin().Z)
			for j, cap := range caps {
				// The cap has positive-area overlap with an original
				// disk exactly when it includes the inner ring or the
				// original disk is the larger radius.
				if cuts[cap.height].Cmp(z) == 0 && (cap.lo < 0 || oi == large) {
					fr[i] = append(fr[i], capFaces[j])
				}
			}
		}
		for i, edge := range input.edges {
			p, q := input.vertices[edge.Start], input.vertices[edge.End]
			if p.Z != q.Z {
				for _, wall := range walls {
					if wall.operand == oi {
						er[i] = append(er[i], wall.edge)
					}
				}
				continue
			}
			for j, candidate := range edges {
				x, y := vertices[candidate.Start], vertices[candidate.End]
				if p == x && q == y || p == y && q == x {
					er[i] = append(er[i], j)
				}
			}
		}
	}
	return out, history, nil
}
