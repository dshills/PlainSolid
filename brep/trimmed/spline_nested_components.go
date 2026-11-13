package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineShellMaterialProof describes one embedded boundary component.
// Parent is its smallest strict enclosing shell, or -1. Depth controls
// material polarity: even depths are outward and odd depths are cavities.
type SplineShellMaterialProof struct {
	SplineComponentProof
	Parent, Depth int
}

type certifiedSplineMaterialShell struct {
	SplineShellMaterialProof
	original Shell
	solid    Solid // always outward for geometric comparisons
	inward   bool
}

// ValidateNestedSplineComponents proves disjoint or strictly nested material
// boundaries, including cavities and islands inside cavities. Supported
// shell families are certified spline rectangles and canonical wrapped
// toroids/frusta. Overlapping conservative boxes require a further proof:
// strict containment of compatible coaxial profiles, strict box enclosure
// inside a circular frustum, or complete separation and witness
// classification between two rectangular spline shells.
func (s Solid) ValidateNestedSplineComponents(ctx context.Context, o SplineShellBooleanOptions) ([]SplineShellMaterialProof, error) {
	shells, e := gatherCertifiedSplineMaterial(ctx, s, o)
	if e != nil {
		return nil, e
	}
	m, e := splineMaterialContainment(ctx, shells, o)
	if e == nil {
		e = assignSplineMaterialDepths(shells, m)
	}
	if e != nil {
		return nil, e
	}
	out := make([]SplineShellMaterialProof, len(shells))
	for i, s := range shells {
		out[i] = s.SplineShellMaterialProof
	}
	return out, nil
}

func gatherCertifiedSplineMaterial(ctx context.Context, s Solid, o SplineShellBooleanOptions) ([]certifiedSplineMaterialShell, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	maxFaces := o.Embedding.MaxFaces
	if maxFaces == 0 {
		maxFaces = 64
	}
	if maxFaces < 1 || maxFaces > 128 {
		return nil, geom.ErrInvalid
	}
	if len(s.faces) > maxFaces {
		return nil, parametric.ErrBudget
	}
	var out []certifiedSplineMaterialShell
	for i, original := range s.Topology().Shells {
		part, e := s.ShellContext(ctx, i)
		if e != nil {
			return nil, e
		}
		proof, e := part.validateSplineComponents(ctx, o.Embedding, false)
		inward := false
		if e != nil && ctx.Err() == nil {
			flipped, flipErr := reverseMaterialShell(ctx, part)
			if flipErr != nil {
				return nil, flipErr
			}
			var reverseErr error
			proof, reverseErr = flipped.validateSplineComponents(ctx, o.Embedding, false)
			if reverseErr != nil {
				return nil, fmt.Errorf("trimmed: material shell %d: %w", i, e)
			}
			part, inward, e = flipped, true, nil
		}
		if e != nil {
			return nil, e
		}
		p := proof[0]
		p.Shell = i
		out = append(out, certifiedSplineMaterialShell{SplineShellMaterialProof{SplineComponentProof: p, Parent: -1}, original, part, inward})
	}
	return out, nil
}

func reverseMaterialShell(ctx context.Context, s Solid) (Solid, error) {
	f := s.Faces()
	for i := range f {
		f[i].Reversed = !f[i].Reversed
	}
	return NewSolidContext(ctx, s.vertices, s.edges, f, s.tolerance)
}

func splineMaterialContainment(ctx context.Context, shells []certifiedSplineMaterialShell, o SplineShellBooleanOptions) ([][]bool, error) {
	m := make([][]bool, len(shells))
	for i := range m {
		m[i] = make([]bool, len(shells))
	}
	for i, a := range shells {
		for j := i + 1; j < len(shells); j++ {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			b := shells[j]
			if _, overlaps := a.Bounds.Intersect(b.Bounds); !overlaps {
				continue
			}
			contains, e := exactCoaxialShellContainment(ctx, a, b, o.Embedding.Patches)
			if e != nil {
				return nil, e
			}
			if contains > 0 || toroidStrictlyContains(a.Toroid, b.Toroid) || frustumStrictlyContainsBox(a.SplineComponentProof, b.Bounds) {
				m[i][j] = true
			} else if contains < 0 || toroidStrictlyContains(b.Toroid, a.Toroid) || frustumStrictlyContainsBox(b.SplineComponentProof, a.Bounds) {
				m[j][i] = true
			} else if a.Rectangle != nil && b.Rectangle != nil {
				// The independently certified ray result identifies which
				// side of the disjoint boundary contains each connected shell.
				_, h, e := CombineSeparatedSplineShells(ctx, a.solid, b.solid, parametric.Intersection, o)
				if e != nil {
					return nil, fmt.Errorf("trimmed: material shell pair %d,%d: %w", i, j, e)
				}
				m[i][j], m[j][i] = h.BInsideA, h.AInsideB
			} else {
				return nil, fmt.Errorf("trimmed: material shell pair %d,%d is not proved separated: %w", i, j, geom.ErrConvergence)
			}
		}
	}
	return m, nil
}

func toroidStrictlyContains(a, b *geom.WrappedToroidEmbedding) bool {
	if a == nil || b == nil || !a.CircularProfiles || !b.CircularProfiles || a.MajorRadius.Cmp(b.MajorRadius) != 0 ||
		a.RadialMinorRadius.Cmp(b.RadialMinorRadius) <= 0 || a.AxialMinorRadius.Cmp(b.AxialMinorRadius) <= 0 {
		return false
	}
	for i, p := range a.Center {
		if p.Cmp(b.Center[i]) != 0 {
			return false
		}
	}
	// Each azimuth has two concentric elliptical meridian disks with
	// common principal axes, hence strict inclusion at every azimuth.
	return true
}

func exactCoaxialShellContainment(ctx context.Context, a, b certifiedSplineMaterialShell, o geom.SurfacePatchOptions) (int, error) {
	if a.Cylinder == nil || b.Cylinder == nil || !sameCylinderAxis(*a.Cylinder, *b.Cylinder) {
		return 0, nil
	}
	x, y := a.Cylinder, b.Cylinder
	inside := func(a, b *geom.WrappedCylinderEmbedding) bool {
		return a.LowerZ.Cmp(b.LowerZ) < 0 && a.UpperZ.Cmp(b.UpperZ) > 0 && a.Radius.Cmp(b.Radius) > 0
	}
	sign := 0
	if inside(x, y) {
		sign = 1
	} else if inside(y, x) {
		sign = -1
	} else {
		return 0, nil
	}
	support := func(s Solid) *geom.WrappedSurfacePatch {
		for _, f := range s.faces {
			if w, ok := f.Surface.(*geom.WrappedSurfacePatch); ok {
				return w
			}
		}
		return nil
	}
	equal, e := sameNormalizedCylinderProfile(ctx, support(a.solid), support(b.solid), *x, *y, o)
	if e != nil || !equal {
		return 0, e
	}
	return sign, nil
}

func frustumStrictlyContainsBox(p SplineComponentProof, b geom.Box) bool {
	var cx, cy, low, high, r0, r1 *big.Rat
	if p.Cylinder != nil && p.Cylinder.Circular {
		x := p.Cylinder
		cx, cy, low, high, r0, r1 = x.CenterX, x.CenterY, x.LowerZ, x.UpperZ, x.Radius, x.Radius
	} else if p.Frustum != nil && p.Frustum.Circular {
		x := p.Frustum
		cx, cy, low, high, r0, r1 = x.CenterX, x.CenterY, x.LowerZ, x.UpperZ, x.LowerRadius, x.UpperRadius
	} else {
		return false
	}
	l, h := ratInset(b.Min.Z), ratInset(b.Max.Z)
	if l.Cmp(low) <= 0 || h.Cmp(high) >= 0 {
		return false
	}
	// A circular frustum is convex: strict inclusion of all corners
	// proves strict inclusion of the complete enclosing box.
	for _, z := range []*big.Rat{l, h} {
		t := new(big.Rat).Quo(new(big.Rat).Sub(z, low), new(big.Rat).Sub(high, low))
		r := new(big.Rat).Add(r0, new(big.Rat).Mul(t, new(big.Rat).Sub(r1, r0)))
		r.Mul(r, r)
		for _, x := range []float64{b.Min.X, b.Max.X} {
			for _, y := range []float64{b.Min.Y, b.Max.Y} {
				u, v := new(big.Rat).Sub(ratInset(x), cx), new(big.Rat).Sub(ratInset(y), cy)
				q := new(big.Rat).Add(u.Mul(u, u), v.Mul(v, v))
				if q.Cmp(r) >= 0 {
					return false
				}
			}
		}
	}
	return true
}

func assignSplineMaterialDepths(shells []certifiedSplineMaterialShell, m [][]bool) error {
	for i := range shells {
		parent := -1
		for j := range shells {
			if !m[j][i] {
				continue
			}
			shells[i].Depth++
			if parent < 0 || m[parent][j] {
				parent = j
			} else if !m[j][parent] {
				return geom.ErrConvergence
			}
		}
		shells[i].Parent = parent
		if shells[i].inward != (shells[i].Depth%2 == 1) {
			return fmt.Errorf("trimmed: material shell %d has incorrect nesting orientation: %w", i, geom.ErrDegenerate)
		}
	}
	return nil
}

// CombineNestedSplineComponents combines any number of supported components
// once all input boundary pairs have been proved strictly separated or nested.
// Each shell is selected by Boolean occupancy on its two sides, and reversed
// when necessary. Faces, edges, and surface objects are preserved exactly.
func CombineNestedSplineComponents(ctx context.Context, a, b Solid, op parametric.BooleanOp, o SplineShellBooleanOptions) (Solid, SplineShellCombineHistory, error) {
	var h SplineShellCombineHistory
	if op > parametric.Difference {
		return Solid{}, h, geom.ErrInvalid
	}
	as, e := gatherCertifiedSplineMaterial(ctx, a, o)
	if e != nil {
		return Solid{}, h, e
	}
	bs, e := gatherCertifiedSplineMaterial(ctx, b, o)
	if e != nil {
		return Solid{}, h, e
	}
	shells := append(append([]certifiedSplineMaterialShell(nil), as...), bs...)
	m, e := splineMaterialContainment(ctx, shells, o)
	if e != nil {
		return Solid{}, h, e
	}
	for _, span := range [][2]int{{0, len(as)}, {len(as), len(shells)}} {
		matrix := make([][]bool, span[1]-span[0])
		for i := range matrix {
			matrix[i] = m[span[0]+i][span[0]:span[1]]
		}
		if e = assignSplineMaterialDepths(shells[span[0]:span[1]], matrix); e != nil {
			return Solid{}, h, e
		}
	}
	h = SplineShellCombineHistory{Method: "nested-components",
		FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)),
		EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(b.edges))}
	inside := func(v [2]bool) bool {
		switch op {
		case parametric.Union:
			return v[0] || v[1]
		case parametric.Intersection:
			return v[0] && v[1]
		default:
			return v[0] && !v[1]
		}
	}
	var parts []PlacedSolid
	nf, ne := 0, 0
	for i, s := range shells {
		operand := 0
		if i >= len(as) {
			operand = 1
		}
		var outside [2]bool
		for j := range shells {
			if m[j][i] {
				k := 0
				if j >= len(as) {
					k = 1
				}
				outside[k] = !outside[k]
			}
		}
		interior := outside
		interior[operand] = !interior[operand]
		x, y := inside(outside), inside(interior)
		if x == y {
			continue
		}
		part := s.solid
		if x {
			part, e = reverseMaterialShell(ctx, part)
			if e != nil {
				return Solid{}, SplineShellCombineHistory{}, e
			}
		}
		parts = append(parts, PlacedSolid{part, geom.Identity()})
		faces, edges := [2][][]int{h.FacesA, h.FacesB}[operand], [2][][]int{h.EdgesA, h.EdgesB}[operand]
		for j, old := range s.original.Faces {
			faces[old] = []int{nf + j}
		}
		for j, old := range s.original.Edges {
			edges[old] = []int{ne + j}
		}
		nf += len(part.faces)
		ne += len(part.edges)
	}
	switch len(parts) {
	case 0:
		return Solid{}, h, nil
	case 1:
		return parts[0].Solid, h, nil
	default:
		s, e := AssembleShells(ctx, parts, nf)
		if e != nil {
			return Solid{}, SplineShellCombineHistory{}, e
		}
		return s, h, nil
	}
}
