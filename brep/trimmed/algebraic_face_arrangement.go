//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

type AlgebraicFaceCut struct {
	Face int
	Line [3]*big.Rat
}

type AlgebraicArrangementEdge struct {
	Start, End int
	Boundary   int // subdivision edge, or -1 for a generated cut
	Cut        int // cut index, or -1 for an existing boundary edge
}

type AlgebraicArrangementFace struct {
	Original, Fragment int // Fragment is -1 for an untouched face
	Outer              []AlgebraicSubdivisionUse
	Holes              [][]AlgebraicSubdivisionUse
	Reversed           bool
}

// AlgebraicFaceArrangement preserves one closed indexed boundary while
// partitioning selected source faces. Generated cut edges use exact composed
// model curves with algebraic endpoints. This changes the boundary's face
// arrangement, not its geometry or material, and does not infer embedding.
type AlgebraicFaceArrangement struct {
	subdivision   AlgebraicSubdivision
	edges         []AlgebraicArrangementEdge
	faces         []AlgebraicArrangementFace
	curves        []AlgebraicPartitionCurve
	facesBySource [][]int
}

func (a AlgebraicFaceArrangement) Edges() []AlgebraicArrangementEdge {
	return append([]AlgebraicArrangementEdge(nil), a.edges...)
}

func (a AlgebraicFaceArrangement) Faces() []AlgebraicArrangementFace {
	out := make([]AlgebraicArrangementFace, len(a.faces))
	for i, f := range a.faces {
		out[i] = f
		out[i].Outer = append([]AlgebraicSubdivisionUse(nil), f.Outer...)
		out[i].Holes = make([][]AlgebraicSubdivisionUse, len(f.Holes))
		for j, h := range f.Holes {
			out[i].Holes[j] = append([]AlgebraicSubdivisionUse(nil), h...)
		}
	}
	return out
}

func (a AlgebraicFaceArrangement) Vertices() []AlgebraicSubdivisionVertex {
	return a.subdivision.Vertices()
}

// FacesBySource returns result face IDs for each original face in source order.
func (a AlgebraicFaceArrangement) FacesBySource() [][]int {
	out := make([][]int, len(a.facesBySource))
	for i, row := range a.facesBySource {
		out[i] = append([]int(nil), row...)
	}
	return out
}

func (a AlgebraicFaceArrangement) Topology() AlgebraicSubdivisionTopology {
	r := a.subdivision.Topology()
	r.Edges, r.Faces = len(a.edges), len(a.faces)
	r.EulerCharacteristic = r.Vertices - r.Edges + r.Faces - r.InnerWires + r.CollapsedEdges
	return r
}

// ArrangeFacesByAlgebraicUVLines applies one independently proved cut per
// selected face. Adjacent faces may be partitioned at shared vertices. Each
// generated edge belongs to exactly two fragments of its own source chart.
// Existing edge and vertex IDs are retained. Invalid input produces no result.
func (s AlgebraicSubdivision) ArrangeFacesByAlgebraicUVLines(ctx context.Context, cuts []AlgebraicFaceCut,
	o AlgebraicSeamOptions) (AlgebraicFaceArrangement, error) {
	var empty AlgebraicFaceArrangement
	if ctx == nil || len(cuts) == 0 || len(cuts) > 32 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if e := s.checkIncidence(ctx); e != nil {
		return empty, e
	}
	r := AlgebraicFaceArrangement{subdivision: s, facesBySource: make([][]int, len(s.faces))}
	for i, e := range s.edges {
		r.edges = append(r.edges, AlgebraicArrangementEdge{e.Start, e.End, i, -1})
	}
	byFace := make(map[int]int, len(cuts))
	for i, x := range cuts {
		if _, ok := byFace[x.Face]; ok {
			return empty, geom.ErrInvalid
		}
		q, e := s.BuildAlgebraicPartitionCurve(ctx, x.Face, x.Line, o)
		if e != nil {
			return empty, e
		}
		byFace[x.Face] = i
		r.curves = append(r.curves, q)
		r.edges = append(r.edges, AlgebraicArrangementEdge{q.Partition.Vertices[0], q.Partition.Vertices[1], -1, i})
	}
	for face, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if cut, ok := byFace[face]; ok {
			uses := make(map[int]int, len(f.Outer))
			for _, u := range f.Outer {
				uses[u.Edge] = u.OriginalUse
			}
			for k, loop := range r.curves[cut].Partition.Fragments {
				q := AlgebraicArrangementFace{Original: face, Fragment: k, Reversed: f.Reversed, Holes: r.curves[cut].Partition.HoleFragments[k]}
				for _, u := range loop {
					if u.BoundaryEdge < 0 {
						q.Outer = append(q.Outer, AlgebraicSubdivisionUse{Edge: len(s.edges) + cut, OriginalUse: -1, Reversed: u.Reversed})
					} else {
						j, ok := uses[u.BoundaryEdge]
						if !ok {
							return empty, geom.ErrConvergence
						}
						q.Outer = append(q.Outer, AlgebraicSubdivisionUse{Edge: u.BoundaryEdge, OriginalUse: j, Reversed: u.Reversed})
					}
				}
				r.facesBySource[face] = append(r.facesBySource[face], len(r.faces))
				r.faces = append(r.faces, q)
			}
		} else {
			r.facesBySource[face] = []int{len(r.faces)}
			r.faces = append(r.faces, AlgebraicArrangementFace{face, -1, f.Outer, f.Holes, f.Reversed})
		}
	}
	if e := r.checkIncidence(ctx); e != nil {
		return empty, e
	}
	if r.Topology().EulerCharacteristic != s.Topology().EulerCharacteristic {
		return empty, geom.ErrConvergence
	}
	return r, nil
}

func (a AlgebraicFaceArrangement) checkIncidence(ctx context.Context) error {
	counts := make([][2]int, len(a.edges))
	for _, f := range a.faces {
		if e := ctx.Err(); e != nil {
			return e
		}
		for _, loop := range append([][]AlgebraicSubdivisionUse{f.Outer}, f.Holes...) {
			first, last := -1, -1
			for _, u := range loop {
				if u.Edge < 0 || u.Edge >= len(a.edges) {
					return geom.ErrInvalid
				}
				e := a.edges[u.Edge]
				from, to := e.Start, e.End
				if u.Reversed {
					from, to = to, from
				}
				if first < 0 {
					first = from
				} else if from != last {
					return geom.ErrConvergence
				}
				last = to
				orientation := 0
				if u.Reversed != f.Reversed {
					orientation = 1
				}
				counts[u.Edge][orientation]++
			}
			if first < 0 || first != last {
				return geom.ErrConvergence
			}
		}
	}
	for i, n := range counts {
		b := a.edges[i].Boundary
		pole := b >= 0 && a.subdivision.source.edges[a.subdivision.edges[b].Original].Curve == nil
		if pole {
			if n[0]+n[1] != 1 {
				return geom.ErrConvergence
			}
		} else if n != ([2]int{1, 1}) {
			return geom.ErrConvergence
		}
	}
	return nil
}

// ModelCurveCells returns an independently proved original-edge subdomain
// or the exact source-chart composition for a generated cut. Descriptors
// are immutable; the returned slices and rational enclosures are owned.
func (a AlgebraicFaceArrangement) ModelCurveCells(ctx context.Context, edge int, o AlgebraicSeamOptions) (geom.SurfaceCurveCompositionCells, error) {
	var empty geom.SurfaceCurveCompositionCells
	if ctx == nil || edge < 0 || edge >= len(a.edges) {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	x := a.edges[edge]
	if x.Boundary >= 0 {
		return a.subdivision.ModelCurveCells(ctx, x.Boundary, o)
	}
	r := a.curves[x.Cut].Model
	r.Cells = append([]*geom.ExactComposedCurveCell(nil), r.Cells...)
	r.Events = append([]geom.ExactParameterInterval(nil), r.Events...)
	for i, p := range r.Events {
		r.Events[i] = geom.ExactParameterInterval{Lower: new(big.Rat).Set(p.Lower), Upper: new(big.Rat).Set(p.Upper)}
	}
	return r, nil
}
