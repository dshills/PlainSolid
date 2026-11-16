package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineFaceCoincidenceOptions bounds the two independent trim validations,
// per-use extraction and one cumulative support/curve identity ledger.
type SplineFaceCoincidenceOptions struct {
	// MatchHolePermutation admits uniquely shared exact original hole vertices.
	// Candidate matching never replaces the subsequent complete wire proof.
	MatchHolePermutation bool
	// ProjectiveAxes uses a proved graph correspondence along these two
	// model axes. This subset requires paired whole uses and matching
	// hole order; exact original coedge clocks must agree under the map.
	ProjectiveAxes         *[2]int
	Patches                geom.SurfacePatchOptions
	Trims                  trimvalidate.Options
	MaxUses, MaxOperations int
}

// CoincidentSplineUse identifies original topological uses. Reversed compares
// their UV traversal, independently of each model edge's stored direction.
type CoincidentSplineUse struct {
	// RangeA and RangeB are owned normalized original coedge traversal ranges.
	RangeA, RangeB                         [2]*big.Rat
	LoopA, UseA, EdgeA, LoopB, UseB, EdgeB int
	Reversed                               bool
}

// SplineFaceCoincidence reports equality of complete original trimmed images,
// not chart injectivity, shell embedding or Boolean admission. Hole order must
// agree unless MatchHolePermutation proves a unique original-vertex matching;
// each wire may start at a different use. B may subdivide complete A
// uses when exact rational cut recovery and full subcurve identity succeed.
// Partial overlap and incompatible refinements remain unsupported. Slices are owned.
type SplineFaceCoincidence struct {
	Faces [2]int
	Map   intersect.SplineChartMap
	// ProjectiveMap is populated instead of Map in the projective subset.
	ProjectiveMap *geom.ProjectiveSurfaceGraphOverlap `json:",omitempty"`
	Uses          []CoincidentSplineUse
	// OppositeOrientation compares oriented chart normals at regular points;
	// it does not prove regularity.
	OppositeOrientation bool
	Complete            bool
	ExactOperations     int
}

func (a Solid) CertifySplineFaceCoincidence(ctx context.Context, faceA int, b Solid, faceB int, o SplineFaceCoincidenceOptions) (SplineFaceCoincidence, error) {
	fail := func(e error) (SplineFaceCoincidence, error) { return SplineFaceCoincidence{}, e }
	if ctx == nil || faceA < 0 || faceA >= len(a.faces) || faceB < 0 || faceB >= len(b.faces) {
		return fail(geom.ErrInvalid)
	}
	if o.MaxUses == 0 {
		o.MaxUses = 64
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxUses < 1 || o.MaxUses > 256 || o.MaxOperations < 1 || o.MaxOperations > 64000000 {
		return fail(geom.ErrInvalid)
	}
	fa, fb := a.faces[faceA], b.faces[faceB]
	loopsA := append([][]Use{fa.Outer}, fa.Holes...)
	loopsB := append([][]Use{fb.Outer}, fb.Holes...)
	if len(loopsA) != len(loopsB) {
		return fail(geom.ErrConvergence)
	}
	count := 0
	for _, loops := range [][][]Use{loopsA, loopsB} {
		for _, loop := range loops {
			count += len(loop)
			if count > o.MaxUses {
				return fail(parametric.ErrBudget)
			}
		}
	}
	for _, face := range []Face{fa, fb} {
		proof, e := ValidateFaceTrimLoops(ctx, face, o.Trims)
		if e != nil {
			return fail(e)
		}
		if proof.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
	}
	patch := func(face Face) (*geom.SurfacePatch, error) {
		base, pose := parametric.SurfaceMapping(face.Surface)
		p, e := exactSplineSupportPatch(base, o.Patches)
		if e != nil {
			return nil, e
		}
		return p.Transformed(pose)
	}
	pa, e := patch(fa)
	if e != nil {
		return fail(e)
	}
	pb, e := patch(fb)
	if e != nil {
		return fail(e)
	}
	if o.ProjectiveAxes != nil {
		return certifyProjectiveSplineFaceWires(ctx, [2]int{faceA, faceB}, pa, pb, [2][][]Use{loopsA, loopsB}, fa.Reversed != fb.Reversed, o)
	}
	for i, face := range []Face{fa, fb} {
		bounds, err := faceUVBounds(face)
		if err != nil {
			return fail(err)
		}
		domain := pa.Domain()
		if i == 1 {
			domain = pb.Domain()
		}
		if bounds.UMin < domain.UMin || bounds.UMax > domain.UMax || bounds.VMin < domain.VMin || bounds.VMax > domain.VMax {
			return fail(geom.ErrDomain)
		}
	}
	support, e := intersect.FindExactSplineCoincidence(ctx, pa, pb, intersect.SplineCoincidenceOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
	if e != nil {
		return fail(e)
	}
	if !support.Found {
		return fail(geom.ErrConvergence)
	}
	r := SplineFaceCoincidence{Faces: [2]int{faceA, faceB}, Map: support.Map, ExactOperations: support.ExactOperations}
	r.OppositeOrientation = (support.Map.Orientation() < 0) != (fa.Reversed != fb.Reversed)
	matrix := splineCoincidenceUVMatrix(support.Map)
	loopIDs := make([]int, len(loopsB))
	for i := range loopIDs {
		loopIDs[i] = i
	}
	if o.MatchHolePermutation {
		var err error
		loopIDs, err = coincidentHoleOrder(ctx, loopsA, loopsB, matrix, o.Patches)
		if err != nil {
			return fail(err)
		}
	}
	for li, la := range loopsA {
		lb := loopsB[loopIDs[li]]
		if len(la) != len(lb) {
			matches, work, err := refinedCoincidentWire(ctx, la, lb, li, matrix, support.Map.Orientation(), o, o.MaxOperations-r.ExactOperations)
			r.ExactOperations += work
			if err != nil {
				return fail(err)
			}
			for i := range matches {
				matches[i].LoopB = loopIDs[li]
			}
			r.Uses = append(r.Uses, matches...)
			continue
		}
		type curve struct {
			c       *geom.ExactSplineCurve
			reverse bool
		}
		aa, bb := make([]curve, len(la)), make([]curve, len(lb))
		for j := range la {
			aa[j].c, aa[j].reverse, e = coincidentUseCurve(ctx, la[j], o.Patches)
			if e != nil {
				return fail(e)
			}
			aa[j].c, e = aa[j].c.TransformRational(ctx, matrix, o.Patches)
			if e != nil {
				return fail(e)
			}
			bb[j].c, bb[j].reverse, e = coincidentUseCurve(ctx, lb[j], o.Patches)
			if e != nil {
				return fail(e)
			}
		}
		direction := 1
		if support.Map.Orientation() < 0 {
			direction = -1
		}
		found := false
		for start := range lb {
			matches := []CoincidentSplineUse{}
			for j := range la {
				k := (start + direction*j + len(lb)) % len(lb)
				remaining := o.MaxOperations - r.ExactOperations
				if remaining < 1 {
					return fail(parametric.ErrBudget)
				}
				reverse := aa[j].reverse != bb[k].reverse
				reverse = reverse != (direction < 0)
				comparison, err := geom.CompareExactCurvesContext(ctx, aa[j].c, bb[k].c, geom.ExactCurveComparisonOptions{NormalizeDomains: true, ReverseB: reverse, MaxOperations: remaining, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
				r.ExactOperations += comparison.ExactOperations
				if err != nil {
					return fail(err)
				}
				if !comparison.Complete || !comparison.Equal {
					break
				}
				matches = append(matches, CoincidentSplineUse{LoopA: li, UseA: j, EdgeA: la[j].Edge, LoopB: li, UseB: k, EdgeB: lb[k].Edge, Reversed: direction < 0, RangeA: [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, RangeB: coincidentTraversalRange(direction)})
			}
			if len(matches) == len(la) {
				for i := range matches {
					matches[i].LoopB = loopIDs[li]
				}
				r.Uses = append(r.Uses, matches...)
				found = true
				break
			}
		}
		if !found {
			return fail(geom.ErrConvergence)
		}
	}
	r.Complete = true
	return r, nil
}

func coincidentUseCurve(ctx context.Context, u Use, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, bool, error) {
	switch q := u.Curve.(type) {
	case nil:
		h := [][4]*big.Rat{{ratInset(u.UVStart.X), ratInset(u.UVStart.Y), new(big.Rat), big.NewRat(1, 1)}, {ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y), new(big.Rat), big.NewRat(1, 1)}}
		c, e := geom.NewExactHomogeneousBezier(ctx, h, o)
		return c, false, e
	case ExactBezierUV:
		c, e := q.descriptor(ctx, o)
		return c, false, e
	case ExactQuadraticUV:
		h := make([][4]*big.Rat, 3)
		for i, p := range q.control {
			h[i] = [4]*big.Rat{p[0], p[1], new(big.Rat), p[2]}
		}
		c, e := geom.NewExactHomogeneousBezier(ctx, h, o)
		return c, false, e
	case SplineUV:
		c, e := geom.NewExactSplineCurve(q.Curve, o)
		if e == nil {
			c, e = c.Restrict(q.Range.Min, q.Range.Max, o)
		}
		return c, q.Reversed, e
	default:
		return nil, false, geom.ErrInvalid
	}
}
func splineCoincidenceUVMatrix(m intersect.SplineChartMap) (r [3][4]*big.Rat) {
	for i := range r {
		for j := range r[i] {
			r[i][j] = new(big.Rat)
		}
	}
	r[2][2].SetInt64(1)
	amin := [2]float64{m.DomainA.UMin, m.DomainA.VMin}
	amax := [2]float64{m.DomainA.UMax, m.DomainA.VMax}
	bmin := [2]float64{m.DomainB.UMin, m.DomainB.VMin}
	bmax := [2]float64{m.DomainB.UMax, m.DomainB.VMax}
	for i := 0; i < 2; i++ {
		j := i
		if m.Transposed {
			j = 1 - i
		}
		scale := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(bmax[i]), ratInset(bmin[i])), new(big.Rat).Sub(ratInset(amax[j]), ratInset(amin[j])))
		origin := ratInset(bmin[i])
		if (i == 0 && m.ReverseU) || (i == 1 && m.ReverseV) {
			scale.Neg(scale)
			origin = ratInset(bmax[i])
		}
		r[i][j] = scale
		r[i][3] = new(big.Rat).Sub(origin, new(big.Rat).Mul(scale, ratInset(amin[j])))
	}
	return
}
