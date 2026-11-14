package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

// WrappedAnisotropicBoreBoundarySpan is a closed original cap or wall span.
// Faces and Edges own its relative interior; endpoints retain all original
// cap/rim/crease/seam incidences, including finite clipping endpoints.
type WrappedAnisotropicBoreBoundarySpan struct {
	Start, End   WrappedAnisotropicLineHit
	Faces, Edges []int
}

// WrappedAnisotropicBoreLine separates open material-volume intervals from
// closed boundary spans and boundary events. Hits include boundary
// clip endpoints. All IDs index the original bore. Parameter remains the
// caller's unnormalized t. Nonexact brackets are certified isolated roots,
// never approximate identities or rounded parameter values.
type WrappedAnisotropicBoreLine struct {
	Hits            []WrappedAnisotropicLineHit
	Interior        []WrappedRationalLineInterior
	Boundary        []WrappedAnisotropicBoreBoundarySpan
	ExactOperations int
}

// ClipWrappedAnisotropicStationBoreLineRational accepts positive-length finite
// rational lines through authenticated anisotropic through material. It supports
// horizontal lines at any height, annular cap coincidence, and nonhorizontal
// constant-normalized-profile wall generators. Other coincident polynomials or
// inseparable root brackets fail atomically. Strict source nesting ensures that
// every lateral root is exposed, but cap-disk events in the opening are removed.
// One query ledger covers both source root solves, exact points and arithmetic;
// source authentication retains its separately bounded original passes.
func (s Solid) ClipWrappedAnisotropicStationBoreLineRational(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o WrappedAnisotropicMultiBandLineOptions) (WrappedAnisotropicBoreLine, error) {
	var empty WrappedAnisotropicBoreLine
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 512
	}
	bits := o.Geometry.Surface.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if ctx == nil || o.MaxOperations < 1 || o.MaxOperations > 8000000 || o.MaxEvents < 1 || o.MaxEvents > 512 || bits < 64 || bits > 65536 || !finite(o.Geometry.ParameterTolerance) || o.Geometry.ParameterTolerance <= 0 || o.Geometry.MaxSubdivisions < 0 || o.Geometry.MaxSubdivisions > 65536 {
		return empty, geom.ErrInvalid
	}
	b := &stationMassBudget{ctx: ctx, limit: o.MaxOperations, bits: bits}
	for _, q := range append(append([]*big.Rat{minT, maxT}, origin[:]...), direction[:]...) {
		if q == nil {
			return empty, geom.ErrInvalid
		}
		b.check(q)
	}
	if b.err != nil {
		return empty, b.err
	}
	if minT.Cmp(maxT) >= 0 || (direction[0].Sign() == 0 && direction[1].Sign() == 0 && direction[2].Sign() == 0) {
		return empty, geom.ErrInvalid
	}
	source, solids, e := s.validateAnisotropicStationBore(ctx, o.Geometry.Surface)
	if e != nil {
		return empty, e
	}
	proofs := [2]WrappedAnisotropicStationProof{source.Outer, source.Inner}
	ids := [2][]int{source.OuterWalls, source.InnerWalls}
	var bands [2][]*geom.WrappedSurfacePatch
	var faceMap, edgeMap, vertexMap [2][]int
	for side, p := range proofs {
		for _, id := range ids[side] {
			bands[side] = append(bands[side], s.faces[id].Surface.(*geom.WrappedSurfacePatch))
		}
		faceMap[side] = make([]int, len(solids[side].faces))
		edgeMap[side] = make([]int, len(solids[side].edges))
		vertexMap[side] = make([]int, len(solids[side].vertices))
		for _, m := range [][]int{faceMap[side], edgeMap[side], vertexMap[side]} {
			for i := range m {
				m[i] = -1
			}
		}
		for i, f := range p.History.WallFaces {
			faceMap[side][f] = ids[side][i]
			for _, u := range solids[side].faces[f].Outer {
				found := -1
				var use Use
				for _, v := range s.faces[ids[side][i]].Outer {
					if u.UVStart == v.UVStart && u.UVEnd == v.UVEnd {
						if found >= 0 {
							return empty, geom.ErrConvergence
						}
						found = v.Edge
						use = v
					}
				}
				if found < 0 {
					return empty, geom.ErrConvergence
				}
				if edgeMap[side][u.Edge] >= 0 && edgeMap[side][u.Edge] != found {
					return empty, geom.ErrConvergence
				}
				edgeMap[side][u.Edge] = found
				a, c := solids[side].edges[u.Edge], s.edges[found]
				if u.Reversed != use.Reversed {
					c.Start, c.End = c.End, c.Start
				}
				for _, pair := range [][2]int{{a.Start, c.Start}, {a.End, c.End}} {
					old := vertexMap[side][pair[0]]
					if old >= 0 && old != pair[1] {
						return empty, geom.ErrConvergence
					}
					vertexMap[side][pair[0]] = pair[1]
				}
			}
		}
		for i, f := range p.History.CapFaces {
			faceMap[side][f] = source.Caps[i]
		}
	}
	union := func(a, c []int) []int {
		for _, v := range c {
			a = appendUniqueFace(a, v)
		}
		sort.Ints(a)
		return a
	}
	mapped := func(values, m []int) ([]int, error) {
		var out []int
		for _, id := range values {
			if id < 0 || id >= len(m) || m[id] < 0 {
				return nil, geom.ErrConvergence
			}
			out = appendUniqueFace(out, m[id])
		}
		sort.Ints(out)
		return out, nil
	}
	exact := func(t *big.Rat) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(t), Max: new(big.Rat).Set(t), Multiplicity: 1}
	}
	clone := func(t geom.ToroidLineRoot) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: new(big.Rat).Set(t.Min), Max: new(big.Rat).Set(t.Max), Multiplicity: t.Multiplicity}
	}
	parts := func(faces []int) geom.CylinderBoundaryPart {
		var out geom.CylinderBoundaryPart
		for _, id := range faces {
			if id == source.Caps[0] {
				out |= geom.CylinderLowerCap
			} else if id == source.Caps[1] {
				out |= geom.CylinderUpperCap
			} else {
				out |= geom.CylinderLateral
			}
		}
		return out
	}
	sourcePoint := func(side int, t *big.Rat) (WrappedAnisotropicStationPoint, error) {
		var q [3]*big.Rat
		for axis := range q {
			q[axis] = b.add(origin[axis], b.mul(direction[axis], t))
		}
		if b.err != nil {
			return WrappedAnisotropicStationPoint{}, b.err
		}
		remaining := b.limit - b.used
		if remaining < 1 {
			return WrappedAnisotropicStationPoint{}, geom.ErrConvergence
		}
		p, e := solids[side].locatePreparedAnisotropicStationRational(ctx, q, WrappedRationalStationPointOptions{Source: o.Geometry.Surface, MaxPointOperations: min(remaining, 1000000), MaxCoefficientBits: bits}, &proofs[side], bands[side])
		if e != nil {
			return p, e
		}
		b.used += p.ExactOperations
		if b.used > b.limit {
			return WrappedAnisotropicStationPoint{}, geom.ErrConvergence
		}
		return p, nil
	}
	point := func(t *big.Rat) (parametric.Location, WrappedAnisotropicLineHit, error) {
		var raw [2]WrappedAnisotropicStationPoint
		for side := range raw {
			q, e := sourcePoint(side, t)
			if e != nil {
				return parametric.Outside, WrappedAnisotropicLineHit{}, e
			}
			raw[side] = q
		}
		out := WrappedAnisotropicLineHit{Parameter: exact(t)}
		if raw[0].Location == parametric.Outside || raw[1].Location == parametric.Inside {
			return parametric.Outside, out, nil
		}
		innerWall := false
		for _, id := range raw[1].Faces {
			for _, wall := range proofs[1].History.WallFaces {
				if id == wall {
					innerWall = true
				}
			}
		}
		if raw[1].Location == parametric.Boundary && !innerWall {
			return parametric.Outside, out, nil
		}
		if raw[0].Location != parametric.Boundary && !innerWall {
			return parametric.Inside, out, nil
		}
		for side, q := range raw {
			if q.Location != parametric.Boundary {
				continue
			}
			for _, entry := range []struct {
				src, m []int
				dst    *[]int
			}{{q.Faces, faceMap[side], &out.Faces}, {q.Edges, edgeMap[side], &out.Edges}, {q.RimEdges, edgeMap[side], &out.RimEdges}, {q.Vertices, vertexMap[side], &out.Vertices}} {
				v, e := mapped(entry.src, entry.m)
				if e != nil {
					return parametric.Outside, out, e
				}
				*entry.dst = union(*entry.dst, v)
			}
		}
		out.Parts = parts(out.Faces)
		return parametric.Boundary, out, nil
	}
	var hits []WrappedAnisotropicLineHit
	appendHit := func(h WrappedAnisotropicLineHit) error {
		if len(hits) >= o.MaxEvents {
			return geom.ErrConvergence
		}
		hits = append(hits, h)
		return nil
	}
	addPoint := func(t *big.Rat) error {
		location, h, e := point(t)
		if e != nil {
			return e
		}
		if location == parametric.Boundary {
			return appendHit(h)
		}
		return nil
	}
	type generator struct {
		lo, hi *big.Rat
		face   int
	}
	var generators []generator
	horizontal := direction[2].Sign() == 0
	for side, p := range proofs {
		for i, w := range bands[side] {
			lo, hi := minT, maxT
			if horizontal {
				if origin[2].Cmp(p.Heights[i]) < 0 || origin[2].Cmp(p.Heights[i+1]) > 0 {
					continue
				}
				// Choose the lower band at a common station; all incident owners are
				// restored below. Never duplicate uncertain algebraic root brackets.
				if i > 0 && origin[2].Cmp(p.Heights[i]) == 0 {
					continue
				}
			} else {
				lo = b.div(b.sub(p.Heights[i], origin[2]), direction[2])
				hi = b.div(b.sub(p.Heights[i+1], origin[2]), direction[2])
				if lo.Cmp(hi) > 0 {
					lo, hi = hi, lo
				}
				if lo.Cmp(minT) < 0 {
					lo = minT
				}
				if hi.Cmp(maxT) > 0 {
					hi = maxT
				}
				if lo.Cmp(hi) >= 0 {
					continue
				}
				constant := true
				for axis := 0; axis < 2; axis++ {
					slope := b.div(b.sub(p.Scales[i+1][axis], p.Scales[i][axis]), b.sub(p.Heights[i+1], p.Heights[i]))
					base := b.add(p.Scales[i][axis], b.mul(slope, b.sub(origin[2], p.Heights[i])))
					rate := b.mul(slope, direction[2])
					det := b.sub(b.mul(direction[axis], base), b.mul(b.sub(origin[axis], p.Center[axis]), rate))
					if det.Sign() != 0 {
						constant = false
					}
				}
				if b.err != nil {
					return empty, b.err
				}
				if constant {
					midpoint := b.div(b.add(lo, hi), big.NewRat(2, 1))
					q, e := sourcePoint(side, midpoint)
					if e != nil {
						return empty, e
					}
					if q.Location == parametric.Boundary {
						generators = append(generators, generator{lo, hi, ids[side][i]})
						if e = addPoint(lo); e != nil {
							return empty, e
						}
						if e = addPoint(hi); e != nil {
							return empty, e
						}
					}
					// Constant normalized coordinates also prove absence of
					// any lateral root for an interior/exterior profile point.
					// An unused conic branch may vanish identically here.
					continue
				}
			}
			if b.err != nil {
				return empty, b.err
			}
			remaining := b.limit - b.used
			if remaining < 1 {
				return empty, geom.ErrConvergence
			}
			goOptions := o.Geometry
			goOptions.Surface.MaxOperations = remaining
			goOptions.Surface.MaxCoefficientBits = bits
			q, e := geom.IntersectWrappedAnisotropicClosedBandLineRational(ctx, w, origin, direction, lo, hi, goOptions)
			if e != nil {
				return empty, e
			}
			b.used += q.ExactOperations
			if b.used > b.limit || len(q.Intersections.Overlaps) > 0 {
				return empty, geom.ErrConvergence
			}
			for _, hit := range q.Intersections.Hits {
				root := hit.Parameter
				if root.Min.Cmp(root.Max) == 0 {
					location, h, e := point(root.Min)
					if e != nil {
						return empty, e
					}
					if location != parametric.Boundary {
						return empty, geom.ErrConvergence
					}
					h.Parameter.Multiplicity = root.Multiplicity
					if e = appendHit(h); e != nil {
						return empty, e
					}
					continue
				}
				h := WrappedAnisotropicLineHit{Parameter: clone(root), Parts: geom.CylinderLateral, Faces: []int{ids[side][i]}}
				// Entire root-bracket affine signs establish original quarter ownership.
				// Uncertain cardinal/seam incidence is rejected instead of dropped.
				var signs [2]int
				for axis := 0; axis < 2; axis++ {
					a := b.sub(b.add(origin[axis], b.mul(direction[axis], root.Min)), p.Center[axis])
					c := b.sub(b.add(origin[axis], b.mul(direction[axis], root.Max)), p.Center[axis])
					if a.Sign() == 0 || a.Sign() != c.Sign() {
						return empty, geom.ErrConvergence
					}
					signs[axis] = a.Sign()
				}
				if horizontal {
					quadrant := 0
					if signs[0] < 0 {
						if signs[1] > 0 {
							quadrant = 1
						} else {
							quadrant = 2
						}
					} else if signs[1] < 0 {
						quadrant = 3
					}
					for station, z := range p.Heights {
						if origin[2].Cmp(z) != 0 {
							continue
						}
						edge := edgeMap[side][p.History.StationRims[station][quadrant]]
						h.Edges = []int{edge}
						h.RimEdges = []int{edge}
						if station == 0 {
							h.Faces = union(h.Faces, []int{source.Caps[0]})
						} else if station == len(p.Heights)-1 {
							h.Faces = union(h.Faces, []int{source.Caps[1]})
						} else {
							h.Faces = union(h.Faces, []int{ids[side][station]})
						}
					}
				} else if root.Min.Cmp(lo) <= 0 || root.Max.Cmp(hi) >= 0 {
					return empty, geom.ErrConvergence
				}
				h.Parts = parts(h.Faces)
				if b.err != nil {
					return empty, b.err
				}
				if e = appendHit(h); e != nil {
					return empty, e
				}
			}
		}
		if !horizontal {
			for station, z := range p.Heights {
				t := b.div(b.sub(z, origin[2]), direction[2])
				if b.err != nil {
					return empty, b.err
				}
				if t.Cmp(minT) >= 0 && t.Cmp(maxT) <= 0 {
					local, e := sourcePoint(side, t)
					if e != nil {
						return empty, e
					}
					terminal := station == 0 || station == len(p.Heights)-1
					if len(local.RimEdges) > 0 || (terminal && local.Location == parametric.Boundary) {
						if e = addPoint(t); e != nil {
							return empty, e
						}
					}
				}
			}
		}
	}
	for _, t := range []*big.Rat{minT, maxT} {
		if e = addPoint(t); e != nil {
			return empty, e
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Parameter.Min.Cmp(hits[j].Parameter.Min) < 0 })
	var merged []WrappedAnisotropicLineHit
	for _, h := range hits {
		if len(merged) > 0 {
			last := &merged[len(merged)-1]
			a, c := last.Parameter, h.Parameter
			if a.Max.Cmp(c.Min) >= 0 {
				if a.Min.Cmp(a.Max) != 0 || c.Min.Cmp(c.Max) != 0 || a.Min.Cmp(c.Min) != 0 {
					return empty, geom.ErrConvergence
				}
				last.Faces = union(last.Faces, h.Faces)
				last.Edges = union(last.Edges, h.Edges)
				last.RimEdges = union(last.RimEdges, h.RimEdges)
				last.Vertices = union(last.Vertices, h.Vertices)
				last.Parts |= h.Parts
				last.Parameter.Multiplicity = max(a.Multiplicity, c.Multiplicity)
				continue
			}
		}
		merged = append(merged, h)
	}
	ends := []WrappedAnisotropicLineHit{{Parameter: exact(minT)}}
	ends = append(ends, merged...)
	ends = append(ends, WrappedAnisotropicLineHit{Parameter: exact(maxT)})
	out := WrappedAnisotropicBoreLine{Hits: merged}
	for i := 1; i < len(ends); i++ {
		a, c := ends[i-1], ends[i]
		if a.Parameter.Max.Cmp(c.Parameter.Min) >= 0 {
			if a.Parameter.Min.Cmp(a.Parameter.Max) == 0 && c.Parameter.Min.Cmp(c.Parameter.Max) == 0 && a.Parameter.Min.Cmp(c.Parameter.Min) == 0 {
				continue
			}
			return empty, geom.ErrConvergence
		}
		t := b.div(b.add(a.Parameter.Max, c.Parameter.Min), big.NewRat(2, 1))
		location, owner, e := point(t)
		if e != nil {
			return empty, e
		}
		switch location {
		case parametric.Inside:
			out.Interior = append(out.Interior, WrappedRationalLineInterior{clone(a.Parameter), clone(c.Parameter)})
		case parametric.Outside:
		case parametric.Boundary:
			proved := horizontal && (origin[2].Cmp(proofs[0].Heights[0]) == 0 || origin[2].Cmp(proofs[0].Heights[len(proofs[0].Heights)-1]) == 0)
			for _, g := range generators {
				if a.Parameter.Min.Cmp(g.lo) >= 0 && c.Parameter.Max.Cmp(g.hi) <= 0 {
					for _, id := range owner.Faces {
						if id == g.face {
							proved = true
						}
					}
				}
			}
			if !proved {
				return empty, geom.ErrConvergence
			}
			out.Boundary = append(out.Boundary, WrappedAnisotropicBoreBoundarySpan{Start: a, End: c, Faces: append([]int(nil), owner.Faces...), Edges: append([]int(nil), owner.Edges...)})
		default:
			return empty, geom.ErrConvergence
		}
		if len(out.Hits)+len(out.Interior)+len(out.Boundary) > o.MaxEvents {
			return empty, geom.ErrConvergence
		}
	}
	if b.err != nil {
		return empty, b.err
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	out.ExactOperations = b.used
	return out, nil
}
