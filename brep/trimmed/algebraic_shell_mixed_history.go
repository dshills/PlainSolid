package trimmed

import (
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MixedBooleanFaceHistory maps an original face to every retained result
// fragment. An empty ResultFaces slice denotes an omitted original face.
// ResultFaces indexes the exact mixed material boundary, before tessellation.
type MixedBooleanFaceHistory struct {
	Operand, OriginalFace int
	ResultFaces           []int
}

// MixedBooleanEdgeHistory describes one retained result edge. An original
// edge has Operand and OriginalEdge >= 0 and retains exact normalized source
// limits (nil roots denote 0 and 1). A generated edge has both fields -1 and
// names the shared section Branch, Piece, and the two original face owners.
// Reversed is its traversal relative to the section piece's canonical tube.
type MixedBooleanEdgeHistory struct {
	ResultEdge, Operand, OriginalEdge int
	Lower, Upper                      *geom.ExactAlgebraicRoot
	Branch, Piece                     int
	Faces                             [2]int
	Reversed                          bool
}

// MixedBooleanHistory is computed from the immutable exact selection and
// subdivision. It carries no mutable geometry or display mesh identifiers.
type MixedBooleanHistory struct {
	Operation parametric.BooleanOp
	Faces     []MixedBooleanFaceHistory
	Edges     []MixedBooleanEdgeHistory
}

// History provides stable, source-relative Boolean ancestry without inferring
// correspondence from distance or sampling. It is rebuilt after native reload.
func (s *MixedAlgebraicShellBoolean) History() (MixedBooleanHistory, error) {
	var zero MixedBooleanHistory
	if s == nil || s.section == nil || s.boundary.source == nil {
		return zero, geom.ErrInvalid
	}
	b := &s.boundary
	a, z := len(b.source.a.faces), len(b.source.b.faces)
	out := MixedBooleanHistory{Operation: b.op, Faces: make([]MixedBooleanFaceHistory, 0, a+z), Edges: make([]MixedBooleanEdgeHistory, 0, len(b.index))}
	for operand, count := range [2]int{a, z} {
		for id := 0; id < count; id++ {
			out.Faces = append(out.Faces, MixedBooleanFaceHistory{Operand: operand, OriginalFace: id})
		}
	}
	for i, f := range b.faces {
		if f.Operand < 0 || f.Operand > 1 || f.Original < 0 || f.Original >= [2]int{a, z}[f.Operand] {
			return zero, geom.ErrInvalid
		}
		x := f.Original + f.Operand*a
		out.Faces[x].ResultFaces = append(out.Faces[x].ResultFaces, i)
	}
	for i, e := range b.index {
		x := MixedBooleanEdgeHistory{ResultEdge: i, Operand: -1, OriginalEdge: -1, Branch: -1, Piece: -1, Faces: [2]int{-1, -1}}
		if e.SectionPiece >= 0 {
			if e.OriginalSubdivisionEdge >= 0 || e.SectionPiece >= len(b.parts[0].Section) {
				return zero, geom.ErrInvalid
			}
			p := b.parts[0].Section[e.SectionPiece]
			if p.Branch < 0 || p.Branch >= len(b.source.branches) {
				return zero, geom.ErrInvalid
			}
			x.Branch, x.Piece, x.Reversed = p.Branch, p.Index, p.Reversed
			x.Faces = b.source.branches[p.Branch].Faces
		} else {
			if e.Operand < 0 || e.Operand > 1 || e.OriginalSubdivisionEdge < 0 || e.OriginalSubdivisionEdge >= len(b.parts[e.Operand].Subdivision.edges) {
				return zero, geom.ErrInvalid
			}
			q := b.parts[e.Operand].Subdivision.edges[e.OriginalSubdivisionEdge]
			x.Operand, x.OriginalEdge = e.Operand, q.Original
			x.Lower, x.Upper = q.Lower, q.Upper
		}
		out.Edges = append(out.Edges, x)
	}
	return out, nil
}
