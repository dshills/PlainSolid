package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SewFace is one independent face with local vertex and edge indices.
type SewFace struct {
	Vertices []geom.Vec3
	Edges    []parametric.Edge
	Face     Face
}

// SewOptions bounds exact matching work. Zero MaxFaces and MaxUses select
// 128 faces and 4096 edge uses. Tolerance must be positive.
type SewOptions struct {
	Tolerance         float64
	MaxFaces, MaxUses int
	Proof             edgecheck.Options
}

// SewHistory maps local vertices and edges to the shared boundary graph.
// Face order is retained. A reversed local model edge is recorded in
// EdgeReversed even if its use was originally reversed as well.
type SewHistory struct {
	Vertices, Edges [][]int
	EdgeReversed    [][]bool
}

// Sew joins independently indexed faces with exactly equal endpoint
// coordinates and a proved common normalized edge evaluator. This initial
// domain supports exact line edges, spline isocurves, continuously certified
// spline-section edges and selected analytic or spline periodic seams,
// closed edges and analytic poles.
// It never welds near points. Ambiguous edge matches return an error.
func Sew(ctx context.Context, input []SewFace, o SewOptions) (Solid, SewHistory, error) {
	bad := func(reason string) (Solid, SewHistory, error) {
		return Solid{}, SewHistory{}, fmt.Errorf("trimmed: sew: %s", reason)
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 128
	}
	if o.MaxUses == 0 {
		o.MaxUses = 4096
	}
	if ctx == nil || !finite(o.Tolerance) || o.Tolerance <= 0 || o.MaxFaces < 1 || o.MaxUses < 1 {
		return bad("invalid options")
	}
	if len(input) > o.MaxFaces || len(input) > 4096 || o.MaxUses > 1<<20 {
		return Solid{}, SewHistory{}, parametric.ErrBudget
	}
	var v []geom.Vec3
	var edges []parametric.Edge
	var faces []Face
	h := SewHistory{make([][]int, len(input)), make([][]int, len(input)), make([][]bool, len(input))}
	vertices := map[geom.Vec3]int{}
	sources := map[[2]int]parametric.Edge{}
	ids := map[[2]int]int{}
	type closedSource struct {
		id      int
		matched bool
	}
	closed := map[int][]closedSource{}
	affineCharts := affineEdgeCharts{}
	nuses, nvertices := 0, 0
	for i, p := range input {
		if e := ctx.Err(); e != nil {
			return Solid{}, SewHistory{}, e
		}
		if len(p.Edges) > o.MaxUses-nuses || len(p.Vertices) > 2*o.MaxUses-nvertices {
			return Solid{}, SewHistory{}, parametric.ErrBudget
		}
		nvertices += len(p.Vertices)
		if p.Face.Surface == nil {
			return bad("face has no support")
		}
		f := cloneFace(p.Face)
		h.Vertices[i], h.Edges[i], h.EdgeReversed[i] = make([]int, len(p.Vertices)), make([]int, len(p.Edges)), make([]bool, len(p.Edges))
		used := make([]bool, len(p.Edges))
		usedVertex := make([]bool, len(p.Vertices))
		for j, q := range p.Vertices {
			if !q.IsFinite() {
				return bad("nonfinite vertex")
			}
			id, ok := vertices[q]
			if !ok {
				id = len(v)
				v = append(v, q)
				vertices[q] = id
			}
			h.Vertices[i][j] = id
		}
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for j := range loop {
				if e := ctx.Err(); e != nil {
					return Solid{}, SewHistory{}, e
				}
				nuses++
				if nuses > o.MaxUses {
					return Solid{}, SewHistory{}, parametric.ErrBudget
				}
				u := &loop[j]
				if u.Edge < 0 || u.Edge >= len(p.Edges) || used[u.Edge] {
					return bad("invalid or repeated local edge use")
				}
				old := u.Edge
				used[old] = true
				e := p.Edges[old]
				if e.Start < 0 || e.Start >= len(p.Vertices) || e.End < 0 || e.End >= len(p.Vertices) ||
					e.Curve != nil && (!finite(e.Range.Min) || !finite(e.Range.Max) || e.Range.Min >= e.Range.Max) ||
					e.Curve == nil && e.Start != e.End {
					return bad("invalid edge")
				}
				usedVertex[e.Start], usedVertex[e.End] = true, true
				e.Start, e.End = h.Vertices[i][e.Start], h.Vertices[i][e.End]
				if e.Curve == nil {
					distance := analyticPoleSquaredError(f.Surface, *u, v[e.Start])
					tol := ratInset(o.Tolerance)
					if distance == nil || distance.Cmp(tol.Mul(tol, tol)) > 0 {
						return bad("unproved collapsed pole")
					}
					u.Edge, h.Edges[i][old] = len(edges), len(edges)
					edges = append(edges, e)
					continue
				}
				if e.Start == e.End {
					if (!isCanonicalAnalyticLoop(e) && !isCanonicalWrappedLoop(e)) || !sameAnalyticEdgeUse(e, f.Surface, *u) {
						return bad("unproved closed edge")
					}
					matches := closed[e.Start]
					if len(matches) > 64 {
						return Solid{}, SewHistory{}, parametric.ErrBudget
					}
					index, reversed := -1, false
					for j, x := range matches {
						if e := ctx.Err(); e != nil {
							return Solid{}, SewHistory{}, e
						}
						prior := edges[x.id]
						forward, backward := sameAnalyticModelEdges(prior, e, false), sameAnalyticModelEdges(prior, e, true)
						if !forward && !backward && (isCanonicalWrappedLoop(prior) || isCanonicalWrappedLoop(e)) {
							var err error
							forward, err = compareSewSplineEdges(ctx, prior, e, false, o.Proof)
							if err != nil {
								return Solid{}, SewHistory{}, err
							}
							backward, err = compareSewSplineEdges(ctx, prior, e, true, o.Proof)
							if err != nil {
								return Solid{}, SewHistory{}, err
							}
						}
						if !forward && !backward {
							continue
						}
						if x.matched || index >= 0 || forward && backward {
							return bad("ambiguous closed edge match")
						}
						index, reversed = j, backward
					}
					if index < 0 {
						if len(matches) == 64 {
							return Solid{}, SewHistory{}, parametric.ErrBudget
						}
						u.Edge, h.Edges[i][old] = len(edges), len(edges)
						closed[e.Start] = append(matches, closedSource{id: len(edges)})
						edges = append(edges, e)
					} else {
						matches[index].matched = true
						u.Edge, h.Edges[i][old] = matches[index].id, matches[index].id
						u.Reversed = u.Reversed != reversed
						h.EdgeReversed[i][old] = reversed
					}
					continue
				}
				key := [2]int{min(e.Start, e.End), max(e.Start, e.End)}
				id, exists := ids[key]
				if !exists {
					id = len(edges)
					edges = append(edges, e)
					ids[key], sources[key] = id, e
				} else {
					prior, ok := sources[key]
					if !ok {
						return bad("more than two uses share one endpoint pair")
					}
					delete(sources, key)
					rev := prior.Start != e.Start
					h.EdgeReversed[i][old] = rev
					u.Reversed = u.Reversed != rev
					same := sameSewModelEdge(prior, e, rev) ||
						sameAnalyticModelEdges(prior, e, rev) ||
						sameSewSectionEdges(prior, e, rev) ||
						sameSewStraightTrimEdges(prior, e, rev)
					if !same {
						var err error
						same, err = compareSewSplineEdges(ctx, prior, e, rev, o.Proof)
						if err != nil {
							return Solid{}, SewHistory{}, err
						}
						if !same {
							same, err = affineCharts.matchModel(ctx, prior, e, rev, o.Proof)
							if err != nil {
								return Solid{}, SewHistory{}, err
							}
						}
						if !same {
							return bad("unproved continuous edge correspondence")
						}
					}
				}
				u.Edge, h.Edges[i][old] = id, id
			}
		}
		for _, yes := range append(used, usedVertex...) {
			if !yes {
				return bad("unused local topology")
			}
		}
		faces = append(faces, f)
	}
	if len(sources) != 0 {
		return bad("unmatched boundary edges")
	}
	for _, matches := range closed {
		for _, x := range matches {
			if !x.matched {
				return bad("unmatched closed edge")
			}
		}
	}
	s, e := NewSolidContext(ctx, v, edges, faces, o.Tolerance)
	if e != nil {
		return Solid{}, SewHistory{}, e
	}
	proof, e := s.ValidateEdgeAgreement(ctx, o.Proof)
	if e != nil {
		return Solid{}, SewHistory{}, e
	}
	if !proof.Complete {
		return bad("at least one face use has no continuous edge agreement proof")
	}
	return s, h, nil
}

func sameSewSectionEdges(a, b parametric.Edge, reversed bool) bool {
	x, ok := b.Curve.(surfaceTrimCurve)
	if !ok || b.Range != (parametric.Interval{Max: 1}) {
		return false
	}
	return sameCertifiedSection(a, x.surface, Use{Curve: x.curve, Reversed: reversed, UVStart: x.a, UVEnd: x.b})
}

func sameSewStraightTrimEdges(a, b parametric.Edge, reversed bool) bool {
	x, ok := b.Curve.(surfaceTrimCurve)
	if !ok || b.Range != (parametric.Interval{Max: 1}) {
		return false
	}
	return sameSplineTrim(a, x.surface, Use{Curve: x.curve, Reversed: reversed, UVStart: x.a, UVEnd: x.b})
}

func compareSewSplineEdges(ctx context.Context, a, b parametric.Edge, reversed bool, o edgecheck.Options) (bool, error) {
	x, revX, e := exactEdgeSpline(a)
	if e != nil || x == nil {
		return false, e
	}
	y, revY, e := exactEdgeSpline(b)
	if e != nil || y == nil {
		return false, e
	}
	return compareSplineDescriptors(ctx, x, y, revX != (revY != reversed), o)
}

func sameSewModelEdge(a, b parametric.Edge, reverse bool) bool {
	if x, ok := a.Curve.(geom.Line); ok {
		y, ok := b.Curve.(geom.Line)
		if !ok {
			return false
		}
		p, q := b.Range.Min, b.Range.Max
		if reverse {
			p, q = q, p
		}
		at := func(l geom.Line, t float64) [3]*big.Rat {
			o, v := l.Origin(), l.Direction()
			z := new(big.Rat).SetFloat64(t)
			out := [3]*big.Rat{}
			for i, p := range [3]float64{o.X, o.Y, o.Z} {
				a := [3]float64{v.X, v.Y, v.Z}[i]
				out[i] = new(big.Rat).Add(new(big.Rat).SetFloat64(p), new(big.Rat).Mul(new(big.Rat).SetFloat64(a), z))
			}
			return out
		}
		for _, p := range [][2][3]*big.Rat{{at(x, a.Range.Min), at(y, p)}, {at(x, a.Range.Max), at(y, q)}} {
			for i := range 3 {
				if p[0][i].Cmp(p[1][i]) != 0 {
					return false
				}
			}
		}
		return true
	}
	return false
}
