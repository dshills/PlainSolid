package document

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// FeatureEdge names one authored box edge by its two incident face roles.
// Role order is immaterial. Parallel or identical faces do not name an edge.
type FeatureEdge struct {
	Feature string
	Roles   [2]string
}

// PlanarEdgeHistory addresses a semantic source edge at a regenerated target.
// Target must descend through planar Boolean or affine transform features.
type PlanarEdgeHistory struct {
	Source FeatureEdge
	Target string
}

// ResolvedPlanarEdgeInterval maps one positive-length portion of an output
// edge to the authored source. Output parameters increase along the result
// Edge.Start to Edge.End. Original parameters use the source edge's orientation
// and may decrease. All parameters are exact rationals owned by the result.
type ResolvedPlanarEdgeInterval struct {
	Index                          int
	OutputFrom, OutputTo, From, To *big.Rat
}

// ResolvedPlanarEdgeHistory can contain multiple portions of a result edge.
// Exact positive overlap is retained, including overlap by a coincident
// non-owning operand. Generated portions without source overlap are excluded.
type ResolvedPlanarEdgeHistory struct {
	Intervals []ResolvedPlanarEdgeInterval
}

func (s State) sourceBoxEdge(ref FeatureEdge) (int, error) {
	f, ok := s.Features[ref.Feature]
	if !ok || f.Kind != BoxFeature {
		return 0, geom.ErrUnrepresentable
	}
	a, err := s.ResolveBoxFace(FeatureFace{Feature: ref.Feature, Role: ref.Roles[0]})
	if err != nil {
		return 0, err
	}
	b, err := s.ResolveBoxFace(FeatureFace{Feature: ref.Feature, Role: ref.Roles[1]})
	if err != nil {
		return 0, err
	}
	if a == b {
		return 0, geom.ErrInvalid
	}
	solid, err := brep.Box(f.Min, f.Max)
	if err != nil {
		return 0, err
	}
	faces := solid.Faces()
	for _, x := range faces[a].Boundary {
		for _, y := range faces[b].Boundary {
			if x.Edge == y.Edge {
				return x.Edge, nil
			}
		}
	}
	return 0, geom.ErrInvalid
}

type namedEdgeInterval struct{ output, original [2]*big.Rat }

const namedEdgeCompositionLimit = 250000

func namedEdgeAffine(a [2]*big.Rat, b [2]*big.Rat, at *big.Rat) *big.Rat {
	t := new(big.Rat).Sub(at, a[0])
	t.Quo(t, new(big.Rat).Sub(a[1], a[0]))
	t.Mul(t, new(big.Rat).Sub(b[1], b[0]))
	return t.Add(t, b[0])
}

func bindNamedEdgeOverlap(p namedEdgeInterval, hit poly.EdgeOverlap) (namedEdgeInterval, bool) {
	lo := maxNamedEdgeRat(p.output[0], minNamedEdgeRat(hit.InputFrom, hit.InputTo))
	hi := minNamedEdgeRat(p.output[1], maxNamedEdgeRat(hit.InputFrom, hit.InputTo))
	if lo.Cmp(hi) >= 0 {
		return namedEdgeInterval{}, false
	}
	if hit.InputFrom.Cmp(hit.InputTo) > 0 {
		lo, hi = hi, lo
	}
	var out namedEdgeInterval
	for i, station := range [2]*big.Rat{lo, hi} {
		out.output[i] = namedEdgeAffine([2]*big.Rat{hit.InputFrom, hit.InputTo}, [2]*big.Rat{hit.OutputFrom, hit.OutputTo}, station)
		out.original[i] = namedEdgeAffine(p.output, p.original, station)
	}
	return out, true
}

func minNamedEdgeRat(a, b *big.Rat) *big.Rat {
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}

func maxNamedEdgeRat(a, b *big.Rat) *big.Rat {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}

func namedEdgeIntervalOrder(a, b namedEdgeInterval) int {
	for _, pair := range [][2][2]*big.Rat{{a.output, b.output}, {a.original, b.original}} {
		for i := range pair[0] {
			if cmp := pair[0][i].Cmp(pair[1][i]); cmp != 0 {
				return cmp
			}
		}
	}
	return 0
}

// ResolvePlanarEdgeHistory re-evaluates the feature chain and composes the
// kernel's exact maps for every surviving edge interval. It returns no result
// on cancellation, unsupported provenance, or arithmetic/work exhaustion.
func (s State) ResolvePlanarEdgeHistory(ctx context.Context, ref PlanarEdgeHistory) (ResolvedPlanarEdgeHistory, error) {
	var zero ResolvedPlanarEdgeHistory
	if ctx == nil || ref.Source.Feature == "" || ref.Target == "" {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	original, err := s.sourceBoxEdge(ref.Source)
	if err != nil {
		return zero, err
	}
	if _, ok := s.Features[ref.Target]; !ok {
		return zero, fmt.Errorf("document: unknown target feature %q", ref.Target)
	}
	type lineage struct {
		related bool
		edges   map[int][]namedEdgeInterval
		err     error
	}
	unit := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	cache := map[string]lineage{
		ref.Source.Feature: {related: true, edges: map[int][]namedEdgeInterval{original: {{output: unit, original: unit}}}},
	}
	work := 0
	observer := featureFaceObserver{
		needsEdges: func(f Feature) bool {
			a, b := cache[f.Input], cache[f.Tool]
			return (a.related || b.related) && a.err == nil && b.err == nil && (len(a.edges) > 0 || len(b.edges) > 0)
		},
		record: func(id string, f Feature, geometry State, history poly.TopologyHistory) error {
			if id == ref.Source.Feature {
				return nil
			}
			a, b := cache[f.Input], cache[f.Tool]
			if !a.related && !b.related {
				return nil
			}
			next := lineage{related: true, err: errors.Join(a.err, b.err)}
			defer func() { cache[id] = next }()
			if next.err != nil {
				return nil
			}
			switch f.Kind {
			case TransformFeature:
				in, iok := geometry.Planar[f.Input]
				out, ook := geometry.Planar[id]
				if iok && ook && len(in.Edges()) == len(out.Edges()) {
					// Planar transforms preserve the endpoint IDs and the
					// original Start-to-End orientation under reflection.
					next.edges = a.edges
				} else {
					next.err = geom.ErrUnrepresentable
				}
			case BooleanFeature, PlanarSeamJoinFeature, PlanarInsetJoinFeature:
				if len(a.edges) == 0 && len(b.edges) == 0 {
					return nil
				}
				if len(history.Edges) != len(geometry.Planar[id].Edges()) {
					next.err = geom.ErrUnrepresentable
					break
				}
				prior := [2]lineage{a, b}
				next.edges = make(map[int][]namedEdgeInterval)
				for index, edge := range history.Edges {
					if err := ctx.Err(); err != nil {
						return err
					}
					var rows []namedEdgeInterval
					for _, hit := range edge.Overlaps {
						if hit.Operand > poly.OperandB {
							return geom.ErrUnrepresentable
						}
						for _, ancestor := range prior[hit.Operand].edges[hit.Edge] {
							work++
							if work > namedEdgeCompositionLimit {
								return geom.ErrConvergence
							}
							row, ok := bindNamedEdgeOverlap(ancestor, hit)
							if !ok {
								continue
							}
							for _, value := range [4]*big.Rat{row.output[0], row.output[1], row.original[0], row.original[1]} {
								if value.Num().BitLen() > 4096 || value.Denom().BitLen() > 4096 {
									return geom.ErrConvergence
								}
							}
							rows = append(rows, row)
						}
					}
					if len(rows) > 0 {
						sort.Slice(rows, func(i, j int) bool { return namedEdgeIntervalOrder(rows[i], rows[j]) < 0 })
						unique := rows[:1]
						for _, row := range rows[1:] {
							if namedEdgeIntervalOrder(unique[len(unique)-1], row) != 0 {
								unique = append(unique, row)
							}
						}
						next.edges[index] = unique
					}
				}
			default:
				next.err = fmt.Errorf("document: no planar edge provenance through feature %q: %w", id, geom.ErrUnrepresentable)
			}
			return nil
		},
	}
	geometry, err := s.evaluateFeatures(ctx, &observer)
	if err != nil {
		return zero, err
	}
	next := cache[ref.Target]
	if next.err != nil {
		return zero, next.err
	}
	if !next.related {
		return zero, ErrUnrelatedFeature
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	var out ResolvedPlanarEdgeHistory
	for index := range geometry.Planar[ref.Target].Edges() {
		for _, row := range next.edges[index] {
			out.Intervals = append(out.Intervals, ResolvedPlanarEdgeInterval{Index: index,
				OutputFrom: new(big.Rat).Set(row.output[0]), OutputTo: new(big.Rat).Set(row.output[1]),
				From: new(big.Rat).Set(row.original[0]), To: new(big.Rat).Set(row.original[1]),
			})
		}
	}
	return out, nil
}
