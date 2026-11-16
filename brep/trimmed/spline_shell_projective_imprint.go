package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ProjectiveShellImprintOptions bounds native face splitting and exact
// propagation of each split to the neighboring original coedges.
type ProjectiveShellImprintOptions struct {
	Faces ProjectiveSplineImprintOptions
	Sew   ExactSplineSewOptions
}

// ProjectiveShellImprintEdge maps one interval of an original model edge
// to the sewn graph. Lower and Upper refer to its original normalized
// model clock; Reversed expresses that clock's direction in the result.
type ProjectiveShellImprintEdge struct {
	OriginalEdge, Edge int
	Lower, Upper       *big.Rat
	Reversed           bool
}

// ProjectiveShellImprintOwner preserves original face and edge ancestry.
// Each SourceFaces entry records all emitted faces for that original face.
type ProjectiveShellImprintOwner struct {
	Solid                           Solid
	SourceFaces                     [][]int
	Edges                           []ProjectiveShellImprintEdge
	SectionEdge, InsertedBoundaries int
}

// ProjectiveShellImprint contains each original solid with one explicit
// section edge inserted. No material Boolean is implied.
type ProjectiveShellImprint struct {
	Owners   [2]ProjectiveShellImprintOwner
	Section  geom.ProjectiveContactSpline
	Complete bool
}

type projectiveShellRawEdge struct {
	source int
	clock  [2]*big.Rat
	seam   bool
}

// ImprintProjectiveSplineShellFaces splits the selected face of each shell
// and propagates exact source border subdivisions to each adjacent face.
// The original shells must pass exact spline sewing, including full curve
// equality, original UV trim validity and final vertex link checks.
func (a Solid) ImprintProjectiveSplineShellFaces(ctx context.Context, faceA int, b Solid, faceB int, axes [2]int,
	o ProjectiveShellImprintOptions) (r ProjectiveShellImprint, err error) {
	if ctx == nil || faceA < 0 || faceA >= len(a.faces) || faceB < 0 || faceB >= len(b.faces) || o.Sew.Sew.RepairOrientation {
		return r, geom.ErrInvalid
	}
	split, err := ImprintProjectiveSplineFacePair(ctx, a.faces[faceA], b.faces[faceB], axes, o.Faces)
	if err != nil {
		return r, err
	}
	var owners [2]ProjectiveShellImprintOwner
	for i, source := range [2]Solid{a, b} {
		face := [2]int{faceA, faceB}[i]
		limits := o.Sew
		if limits.Sew.DisplayTolerance == 0 {
			limits.Sew.DisplayTolerance = source.tolerance
		}
		owners[i], err = propagateProjectiveImprint(ctx, source, face, split.Fragments[i], split.Sources[i], limits)
		if err != nil {
			return r, err
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Owners, r.Section, r.Complete = owners, split.Section, true
	return r, nil
}

func propagateProjectiveImprint(ctx context.Context, source Solid, selected int, fragments [2]SewFace,
	fragmentSources [2][]ProjectiveSplineImprintUse, o ExactSplineSewOptions) (r ProjectiveShellImprintOwner, err error) {
	var vertices []geom.Vec3
	var edges []parametric.Edge
	var faces []Face
	var history []projectiveShellRawEdge
	zero, one := new(big.Rat), big.NewRat(1, 1)
	whole := [2]*big.Rat{zero, one}
	index := make([][]int, len(source.faces))
	for i, face := range source.faces {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if i == selected {
			for j, part := range fragments {
				index[i] = append(index[i], len(faces))
				firstV, firstE := len(vertices), len(edges)
				vertices = append(vertices, part.Vertices...)
				for k, e := range part.Edges {
					e.Start, e.End = firstV+e.Start, firstV+e.End
					edges = append(edges, e)
					ref := fragmentSources[j][k]
					row := projectiveShellRawEdge{source: ref.OriginalEdge, clock: ref.OriginalRange, seam: ref.Section}
					if ref.Section {
						row.clock = whole
					} else if face.Outer[ref.OriginalUse].Reversed {
						row.clock = [2]*big.Rat{new(big.Rat).Sub(one, row.clock[0]), new(big.Rat).Sub(one, row.clock[1])}
					}
					history = append(history, row)
				}
				f := cloneFace(part.Face)
				for j := range f.Outer {
					f.Outer[j].Edge += firstE
				}
				faces = append(faces, f)
			}
			continue
		}
		index[i] = []int{len(faces)}
		f := cloneFace(face)
		perFace := make(map[int]int)
		for _, wire := range append([][]Use{f.Outer}, f.Holes...) {
			for j, use := range wire {
				e := source.edges[use.Edge]
				bind := func(v int) int {
					if id, ok := perFace[v]; ok {
						return id
					}
					id := len(vertices)
					perFace[v] = id
					vertices = append(vertices, source.vertices[v])
					return id
				}
				e.Start, e.End = bind(e.Start), bind(e.End)
				wire[j].Edge = len(edges)
				edges = append(edges, e)
				history = append(history, projectiveShellRawEdge{source: use.Edge, clock: whole})
			}
		}
		faces = append(faces, f)
	}
	solid, report, err := SewExactSplineSubdivisions(ctx, vertices, edges, faces, o)
	if err != nil {
		return r, err
	}
	if len(report.FlippedFaces) != 0 || len(report.Segments) != len(history) {
		return r, geom.ErrConvergence
	}
	section := -1
	sectionOccurrences := 0
	var lineage []ProjectiveShellImprintEdge
	for i, row := range report.Segments {
		original := history[i]
		if original.seam {
			if len(row) != 1 || section >= 0 && section != row[0].Edge {
				return r, geom.ErrConvergence
			}
			section = row[0].Edge
			sectionOccurrences++
			continue
		}
		clock := original.clock
		for _, part := range row {
			width := new(big.Rat).Sub(clock[1], clock[0])
			station := func(q *big.Rat) *big.Rat {
				return new(big.Rat).Add(clock[0], new(big.Rat).Mul(width, q))
			}
			lo, hi := station(part.Lower), station(part.Upper)
			reversed := part.Reversed
			if width.Sign() < 0 {
				lo, hi, reversed = hi, lo, !reversed
			}
			lineage = append(lineage, ProjectiveShellImprintEdge{original.source, part.Edge, lo, hi, reversed})
		}
	}
	if section < 0 || sectionOccurrences != 2 {
		return r, geom.ErrConvergence
	}
	r.Solid, r.SourceFaces, r.Edges, r.SectionEdge, r.InsertedBoundaries = solid, index, lineage, section, report.InsertedVertices
	return r, nil
}
