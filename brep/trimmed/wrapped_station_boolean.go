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

// WrappedStationBooleanBand records the source wall retained on an open
// height interval. Two identical source profiles list both original face
// IDs; otherwise the unused operand's face is -1. ResultWall is a face in
// the resulting single station loft.
type WrappedStationBooleanBand struct {
	LowerZ, UpperZ *big.Rat
	SourceWalls    [2]int
	ResultWall     int
	// BasisPreserved is true when the retained wall uses the original
	// circumferential basis, including exact axial subpatches.
	BasisPreserved bool
}

type WrappedStationBooleanHistory struct {
	Bands, InnerBands []WrappedStationBooleanBand
	SourceCaps        [2][2]int
	ResultCaps        [2]int
	CapSources        [2][2]bool // result cap, original operand
}

func hasWrappedStationLoftShape(s Solid) bool {
	n := len(s.faces) - 2
	if n < 1 || n > 127 || len(s.vertices) != 4*(n+1) || len(s.edges) != 5*n+4 {
		return false
	}
	walls := 0
	for _, f := range s.faces {
		if _, ok := f.Surface.(*geom.WrappedSurfacePatch); ok && !f.Reversed {
			walls++
		}
	}
	return walls == n
}

// CombineCoaxialWrappedStationLofts combines complete circular station lofts
// with the same exact axis and end heights. Union and intersection retain
// the larger or smaller radius, respectively, in each exact height slab.
// A transverse radial crossing inserts a shared rim without faceting the
// support. Difference returns empty when A lies inside B, or a through
// bore when B is strictly radially inside A at every height. Mixed or
// pinched differences, stepped heights and nonrepresentable new station
// coordinates are outside this operation.
func CombineCoaxialWrappedStationLofts(ctx context.Context, a, b Solid, op parametric.BooleanOp,
	o geom.SurfacePatchOptions) (Solid, WrappedStationBooleanHistory, error) {
	return combineCoaxialWrappedStationLofts(ctx, a, b, op, o, true)
}

// CombineCoaxialWrappedRationalStationLofts builds Common, Fuse or strict
// through-bore Cut of two complete lofts sharing an exact normalized
// rational profile, axis and end planes. Each output wall preserves the
// selected original U basis. Crossing vertices must be exactly represented.
func CombineCoaxialWrappedRationalStationLofts(ctx context.Context, a, b Solid, op parametric.BooleanOp,
	o geom.SurfacePatchOptions) (Solid, WrappedStationBooleanHistory, error) {
	return combineCoaxialWrappedStationLofts(ctx, a, b, op, o, false)
}

func combineCoaxialWrappedStationLofts(ctx context.Context, a, b Solid, op parametric.BooleanOp,
	o geom.SurfacePatchOptions, circular bool) (Solid, WrappedStationBooleanHistory, error) {
	var h WrappedStationBooleanHistory
	if ctx == nil || op > parametric.Difference {
		return Solid{}, h, geom.ErrInvalid
	}
	x, xb, e := a.validateWrappedProfileStationLoft(ctx, o, circular)
	if e != nil {
		return Solid{}, h, e
	}
	y, yb, e := b.validateWrappedProfileStationLoft(ctx, o, circular)
	if e != nil {
		return Solid{}, h, e
	}
	if x.Center[0].Cmp(y.Center[0]) != 0 || x.Center[1].Cmp(y.Center[1]) != 0 ||
		x.Heights[0].Cmp(y.Heights[0]) != 0 || x.Heights[len(x.Heights)-1].Cmp(y.Heights[len(y.Heights)-1]) != 0 {
		return Solid{}, h, geom.ErrConvergence
	}
	if !circular {
		// Each original station proof guarantees one shared profile
		// within its operand. Compare the first normalized rims in full.
		equal, e := sameNormalizedWrappedProfile(ctx, [2]*geom.WrappedSurfacePatch{xb[0], yb[0]},
			x.Center, [2]*big.Rat{x.Radii[0], y.Radii[0]}, o)
		if e != nil {
			return Solid{}, h, e
		}
		if !equal {
			return Solid{}, h, geom.ErrUnrepresentable
		}
	}
	levels := append(append([]*big.Rat(nil), x.Heights...), y.Heights...)
	sort.Slice(levels, func(i, j int) bool { return levels[i].Cmp(levels[j]) < 0 })
	unique := levels[:0]
	for _, z := range levels {
		if len(unique) == 0 || z.Cmp(unique[len(unique)-1]) != 0 {
			unique = append(unique, z)
		}
	}
	if len(unique) > 128 {
		return Solid{}, h, parametric.ErrBudget
	}
	eval := func(p WrappedStationLoftProof, i int, z *big.Rat) *big.Rat {
		span := new(big.Rat).Sub(p.Heights[i+1], p.Heights[i])
		t := new(big.Rat).Quo(new(big.Rat).Sub(z, p.Heights[i]), span)
		r := new(big.Rat).Sub(p.Radii[i+1], p.Radii[i])
		return r.Add(r.Mul(r, t), p.Radii[i])
	}
	part := [2]int{}
	var stations []WrappedLoftStation
	var outputSupports []*geom.WrappedSurfacePatch
	appendStation := func(z, r *big.Rat) error {
		h, e := geom.RepresentedExactScalar(z)
		if e != nil {
			return e
		}
		radius, e := geom.RepresentedExactScalar(r)
		if e != nil {
			return e
		}
		if len(stations) == 128 {
			return parametric.ErrBudget
		}
		stations = append(stations, WrappedLoftStation{Z: h, Radius: radius})
		return nil
	}
	contained := true
	strictBore := true
	for i, lo := range unique[:len(unique)-1] {
		if e := ctx.Err(); e != nil {
			return Solid{}, WrappedStationBooleanHistory{}, e
		}
		hi := unique[i+1]
		for k, p := range [2]WrappedStationLoftProof{x, y} {
			for p.Heights[part[k]+1].Cmp(lo) <= 0 {
				part[k]++
			}
		}
		a0, a1, b0, b1 := eval(x, part[0], lo), eval(x, part[0], hi), eval(y, part[1], lo), eval(y, part[1], hi)
		d0, d1 := new(big.Rat).Sub(a0, b0), new(big.Rat).Sub(a1, b1)
		contained = contained && d0.Sign() <= 0 && d1.Sign() <= 0
		strictBore = strictBore && d0.Sign() > 0 && d1.Sign() > 0
		if op == parametric.Difference {
			continue
		}
		appendBand := func(z0, z1, r0, r1 *big.Rat, direction int) error {
			if len(stations) == 0 {
				if e := appendStation(z0, r0); e != nil {
					return e
				}
			}
			if e := appendStation(z1, r1); e != nil {
				return e
			}
			source := [2]int{x.History.WallFaces[part[0]], y.History.WallFaces[part[1]]}
			if direction > 0 {
				source[1] = -1
			} else if direction < 0 {
				source[0] = -1
			}
			selected := 0
			if direction < 0 {
				selected = 1
			}
			p := [2]WrappedStationLoftProof{x, y}[selected]
			w := [2][]*geom.WrappedSurfacePatch{xb, yb}[selected][part[selected]]
			j := part[selected]
			// Keep exact original U and use its affine axial subpatch
			// when possible. Bands are later joined only if the two
			// parameterized rims agree throughout.
			if z0.Cmp(p.Heights[j]) > 0 {
				_, w, e = geom.SplitWrappedFrustumAtZ(ctx, w, stations[len(stations)-2].Z, o)
				if e != nil {
					return e
				}
			}
			if z1.Cmp(p.Heights[j+1]) < 0 {
				w, _, e = geom.SplitWrappedFrustumAtZ(ctx, w, stations[len(stations)-1].Z, o)
				if e != nil {
					return e
				}
			}
			outputSupports = append(outputSupports, w)
			h.Bands = append(h.Bands, WrappedStationBooleanBand{LowerZ: new(big.Rat).Set(z0), UpperZ: new(big.Rat).Set(z1),
				SourceWalls: source, ResultWall: len(h.Bands), BasisPreserved: true})
			return nil
		}
		choose := func(c int) int {
			if op == parametric.Intersection {
				return -c
			}
			return c
		}
		selectRadius := func(c int, a, b *big.Rat) *big.Rat {
			if c >= 0 {
				return a
			}
			return b
		}
		if d0.Sign()*d1.Sign() == -1 {
			f := new(big.Rat).Quo(d0, new(big.Rat).Sub(d0, d1))
			z := new(big.Rat).Add(lo, new(big.Rat).Mul(new(big.Rat).Sub(hi, lo), f))
			r := eval(x, part[0], z)
			c0, c1 := choose(d0.Sign()), choose(d1.Sign())
			if e := appendBand(lo, z, selectRadius(c0, a0, b0), r, c0); e != nil {
				return Solid{}, WrappedStationBooleanHistory{}, e
			}
			if e := appendBand(z, hi, r, selectRadius(c1, a1, b1), c1); e != nil {
				return Solid{}, WrappedStationBooleanHistory{}, e
			}
			continue
		}
		c := choose(d0.Sign() + d1.Sign())
		if e := appendBand(lo, hi, selectRadius(c, a0, b0), selectRadius(c, a1, b1), c); e != nil {
			return Solid{}, WrappedStationBooleanHistory{}, e
		}
	}
	if op == parametric.Difference {
		if contained {
			return Solid{}, h, nil
		}
		if strictBore {
			return joinWrappedStationLoftBore(ctx, a, b, x, y, o)
		}
		if !circular {
			return Solid{}, h, geom.ErrUnrepresentable
		}
		return Solid{}, h, geom.ErrConvergence
	}
	cx, e := geom.RepresentedExactScalar(x.Center[0])
	if e != nil {
		return Solid{}, WrappedStationBooleanHistory{}, e
	}
	cy, e := geom.RepresentedExactScalar(x.Center[1])
	if e != nil {
		return Solid{}, WrappedStationBooleanHistory{}, e
	}
	build := NewWrappedRationalStationLoft
	if circular {
		build = NewWrappedCircularStationLoft
	}
	s, p, e := build(ctx, outputSupports, max(a.tolerance, b.tolerance), o)
	if circular && (e == geom.ErrConvergence || e == geom.ErrUnrepresentable) {
		// Both sources were independently proved circular. Different
		// parameterizations can be rebuilt in one exact common basis.
		s, p, e = LoftWrappedCircularStations(ctx, geom.Vec2{X: cx, Y: cy}, stations, max(a.tolerance, b.tolerance), o)
		for i := range h.Bands {
			h.Bands[i].BasisPreserved = false
		}
	}
	if e != nil {
		return Solid{}, WrappedStationBooleanHistory{}, e
	}
	for i, id := range p.WallFaces {
		h.Bands[i].ResultWall = id
	}
	h.SourceCaps, h.ResultCaps = [2][2]int{x.History.CapFaces, y.History.CapFaces}, p.CapFaces
	for i, j := range []int{0, len(x.Radii) - 1} {
		k := 0
		if i == 1 {
			k = len(y.Radii) - 1
		}
		sign := x.Radii[j].Cmp(y.Radii[k])
		if op == parametric.Intersection {
			sign = -sign
		}
		h.CapSources[i] = [2]bool{sign >= 0, sign <= 0}
	}
	return s, h, nil
}

func joinWrappedStationLoftBore(ctx context.Context, a, b Solid, x, y WrappedStationLoftProof,
	o geom.SurfacePatchOptions) (Solid, WrappedStationBooleanHistory, error) {
	return joinProvedWrappedStationLoftBore(ctx, a, b, x.History, y.History, x.Heights, y.Heights, o)
}

// The caller proves strict whole-height nesting; this core only transfers
// original supports/coedges and independently validates the annular topology.
func joinProvedWrappedStationLoftBore(ctx context.Context, a, b Solid, x, y WrappedLoftHistory, xz, yz []*big.Rat, o geom.SurfacePatchOptions) (Solid, WrappedStationBooleanHistory, error) {
	var history WrappedStationBooleanHistory
	history.SourceCaps = [2][2]int{x.CapFaces, y.CapFaces}
	history.ResultCaps = x.CapFaces
	history.CapSources = [2][2]bool{{true, true}, {true, true}}
	v := append(a.Vertices(), b.vertices...)
	edges := a.Edges()
	offset := len(edges)
	for _, e := range b.edges {
		e.Start += len(a.vertices)
		e.End += len(a.vertices)
		edges = append(edges, e)
	}
	faces := a.Faces()
	for i, id := range x.WallFaces {
		history.Bands = append(history.Bands, WrappedStationBooleanBand{
			LowerZ: new(big.Rat).Set(xz[i]), UpperZ: new(big.Rat).Set(xz[i+1]),
			SourceWalls: [2]int{id, -1}, ResultWall: id, BasisPreserved: true,
		})
	}
	for i, id := range y.WallFaces {
		wall := cloneFace(b.faces[id])
		wall.Reversed = !wall.Reversed
		for j := range wall.Outer {
			wall.Outer[j].Edge += offset
		}
		history.InnerBands = append(history.InnerBands, WrappedStationBooleanBand{
			LowerZ: new(big.Rat).Set(yz[i]), UpperZ: new(big.Rat).Set(yz[i+1]),
			SourceWalls: [2]int{-1, id}, ResultWall: len(faces), BasisPreserved: true,
		})
		faces = append(faces, wall)
	}
	for i, id := range x.CapFaces {
		inner := b.faces[y.CapFaces[i]].Outer
		hole := make([]Use, len(inner))
		for j, u := range inner {
			hole[len(inner)-1-j] = Use{Edge: u.Edge + offset, Reversed: !u.Reversed,
				UVStart: u.UVEnd, UVEnd: u.UVStart, Curve: reversePCurve(u.Curve)}
		}
		faces[id].Holes = [][]Use{hole}
		p, e := ValidateFaceTrimLoops(ctx, faces[id], trimvalidate.Options{})
		if e != nil {
			return Solid{}, WrappedStationBooleanHistory{}, e
		}
		if p.Status != trimvalidate.Certified {
			return Solid{}, WrappedStationBooleanHistory{}, geom.ErrConvergence
		}
	}
	s, e := NewSolidContext(ctx, v, edges, faces, max(a.tolerance, b.tolerance))
	if e != nil {
		return Solid{}, WrappedStationBooleanHistory{}, e
	}
	top := s.Topology()
	if len(top.Shells) != 1 || top.InnerWires != 2 || top.EulerCharacteristic != 0 {
		return Solid{}, WrappedStationBooleanHistory{}, geom.ErrDegenerate
	}
	agreement, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return Solid{}, WrappedStationBooleanHistory{}, e
	}
	if !agreement.Complete {
		return Solid{}, WrappedStationBooleanHistory{}, geom.ErrConvergence
	}
	for _, u := range agreement.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return Solid{}, WrappedStationBooleanHistory{}, geom.ErrConvergence
		}
	}
	return s, history, nil
}
