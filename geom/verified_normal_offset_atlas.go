package geom

import (
	"context"
	"math/big"
)

// VerifiedNormalOffsetFace supplies one verified rectangular offset patch
// and its orientation within a connected atlas.
type VerifiedNormalOffsetFace struct {
	Patch          *VerifiedNormalOffsetPatch
	ReverseWinding bool
}

// VerifiedNormalOffsetJoin declares an original full-edge match of two
// different faces. ReverseB and BStationRatio have the same meaning as
// on SurfaceNormalOffsetSeamOptions.
type VerifiedNormalOffsetJoin struct {
	Faces         [2]int
	Edges         [2]SurfaceEdge
	ReverseB      bool
	BStationRatio *big.Rat
}

// VerifiedNormalOffsetAtlasOptions bounds the combined exact work.
// Zero allows 24 faces and eight million operations. Pair controls apply
// to each nonadjacent pair, subject to the remaining shared work.
type VerifiedNormalOffsetAtlasOptions struct {
	MaxFaces, MaxOperations int
	Pair                    VerifiedNormalOffsetSeparationOptions
}

// VerifiedNormalOffsetAtlas reports the certified connected atlas. Closed
// means each original edge is paired. It does not select an absolute
// outward orientation or assign material to a closed atlas.
type VerifiedNormalOffsetAtlas struct {
	Complete, Closed            bool
	BoundaryEdges               int
	ProvenJoins, SeparatedPairs int
	UnresolvedPairs             [][2]int
	ExactOperations             int
}

// CertifyVerifiedNormalOffsetAtlas verifies each declared join, consistent
// orientation, and separation of every remaining face pair. An undeclared
// contact, including one at a corner alone, cannot pass. This requires
// each supported pair to meet along at most one complete original edge.
func CertifyVerifiedNormalOffsetAtlas(ctx context.Context, faces []VerifiedNormalOffsetFace, joins []VerifiedNormalOffsetJoin,
	o VerifiedNormalOffsetAtlasOptions) (r VerifiedNormalOffsetAtlas, err error) {
	if ctx == nil || len(faces) == 0 || o.MaxFaces < 0 || o.MaxFaces > 128 || o.MaxOperations < 0 {
		return r, ErrInvalid
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 24
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8_000_000
	}
	if len(faces) > o.MaxFaces {
		return r, ErrConvergence
	}
	if len(joins) > 2*len(faces) {
		return r, ErrInvalid
	}
	for _, face := range faces {
		if face.Patch == nil || !face.Patch.Domain().Valid() {
			return r, ErrInvalid
		}
	}
	type faceEdge struct {
		face int
		edge SurfaceEdge
	}
	matched := make(map[faceEdge]bool, 2*len(joins))
	paired := make(map[[2]int]VerifiedNormalOffsetJoin, len(joins))
	graph := make([][]int, len(faces))
	for _, join := range joins {
		a, b := join.Faces[0], join.Faces[1]
		ea, eb := join.Edges[0], join.Edges[1]
		if a < 0 || a >= len(faces) || b < 0 || b >= len(faces) || a == b ||
			ea > SurfaceEdgeVMax || eb > SurfaceEdgeVMax || (join.BStationRatio != nil && join.BStationRatio.Sign() <= 0) {
			return r, ErrInvalid
		}
		ids := [2]int{min(a, b), max(a, b)}
		if _, exists := paired[ids]; exists || matched[faceEdge{a, ea}] || matched[faceEdge{b, eb}] {
			return r, ErrInvalid
		}
		x := naturalOffsetBoundarySign(ea, faces[a].ReverseWinding)
		y := naturalOffsetBoundarySign(eb, faces[b].ReverseWinding)
		if join.ReverseB {
			y = -y
		}
		if x == y {
			return r, ErrInvalid
		}
		paired[ids] = join
		matched[faceEdge{a, ea}], matched[faceEdge{b, eb}] = true, true
		graph[a], graph[b] = append(graph[a], b), append(graph[b], a)
	}
	seen := make([]bool, len(faces))
	seen[0] = true
	queue := []int{0}
	for i := 0; i < len(queue); i++ {
		for _, next := range graph[queue[i]] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	if len(queue) != len(faces) {
		return r, ErrInvalid
	}
	r.BoundaryEdges = 4*len(faces) - 2*len(joins)
	pending := make([][2]int, 0, len(faces)*(len(faces)-1)/2)
	// Test joins first. A disallowed extra pair or an unresolved
	// declared seam retains its original pair as part of the report.
	for _, wantJoin := range []bool{true, false} {
		for a := range faces {
			for b := a + 1; b < len(faces); b++ {
				ids := [2]int{a, b}
				if _, isJoin := paired[ids]; isJoin == wantJoin {
					pending = append(pending, ids)
				}
			}
		}
	}
	defer func() { r.UnresolvedPairs = append(r.UnresolvedPairs, pending...) }()
	if err := ctx.Err(); err != nil {
		return r, err
	}
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		left := o.MaxOperations - r.ExactOperations
		if left < 1 {
			return r, ErrConvergence
		}
		ids := pending[0]
		if join, ok := paired[ids]; ok {
			a, b := faces[join.Faces[0]], faces[join.Faces[1]]
			got, err := CertifyVerifiedNormalOffsetAdjacency(ctx, a.Patch, b.Patch, join.Edges[0], join.Edges[1],
				SurfaceNormalOffsetSeamOptions{ReverseB: join.ReverseB, FlipNormalA: a.ReverseWinding,
					FlipNormalB: b.ReverseWinding, BStationRatio: join.BStationRatio,
					MaxOperations: left, MaxCoefficientBits: o.Pair.Patch.MaxCoefficientBits})
			r.ExactOperations += min(left, got.ExactOperations)
			if err != nil {
				return r, err
			}
			if !got.Complete {
				return r, ErrConvergence
			}
			r.ProvenJoins++
		} else {
			pair := o.Pair
			if pair.Patch.MaxOperations == 0 || pair.Patch.MaxOperations > left {
				pair.Patch.MaxOperations = left
			}
			got, err := CertifyVerifiedNormalOffsetSeparation(ctx, faces[ids[0]].Patch, faces[ids[1]].Patch, pair)
			r.ExactOperations += min(left, got.ExactOperations)
			if err != nil {
				return r, err
			}
			if !got.Complete {
				return r, ErrConvergence
			}
			r.SeparatedPairs++
		}
		pending = pending[1:]
	}
	r.Complete, r.Closed = true, r.BoundaryEdges == 0
	return r, nil
}

func naturalOffsetBoundarySign(edge SurfaceEdge, reverse bool) int {
	s := 1
	if edge == SurfaceEdgeUMin || edge == SurfaceEdgeVMax {
		s = -1
	}
	if reverse {
		s = -s
	}
	return s
}
