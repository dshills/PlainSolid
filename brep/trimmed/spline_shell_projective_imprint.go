//go:build research

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
	SectionEdges                    []int // Ordered by the input face pairs.
	SectionEdge, InsertedBoundaries int
}

// ProjectiveShellImprint contains each original solid with one explicit
// section edge inserted. No material Boolean is implied.
type ProjectiveShellImprint struct {
	Owners   [2]ProjectiveShellImprintOwner
	Section  geom.ProjectiveContactSpline
	Complete bool
}

// ProjectiveShellFacePair selects two distinct original face slots and
// the physical projection axes for that one pair.
type ProjectiveShellFacePair struct {
	Faces, Axes [2]int
}

// ProjectiveMultiShellImprint carries a complete closed imprint and one
// shared analytic section for every selected original pair, in input order.
type ProjectiveMultiShellImprint struct {
	Owners   [2]ProjectiveShellImprintOwner
	Sections []geom.ProjectiveContactSpline
	Complete bool
}

type projectiveShellRawEdge struct {
	source int
	clock  [2]*big.Rat
	seam   bool
	pair   int
}

type projectiveShellReplacement struct {
	fragments [2]SewFace
	sources   [2][]ProjectiveSplineImprintUse
	pair      int
}

// ImprintProjectiveSplineShellFaces splits the selected face of each shell
// and propagates exact source border subdivisions to each adjacent face.
// The original shells must pass exact spline sewing, including full curve
// equality, original UV trim validity and final vertex link checks.
func (a Solid) ImprintProjectiveSplineShellFaces(ctx context.Context, faceA int, b Solid, faceB int, axes [2]int,
	o ProjectiveShellImprintOptions) (r ProjectiveShellImprint, err error) {
	pair := ProjectiveShellFacePair{Faces: [2]int{faceA, faceB}, Axes: axes}
	multiple, err := a.ImprintProjectiveSplineShellFacePairs(ctx, b, []ProjectiveShellFacePair{pair}, o)
	if err != nil {
		return r, err
	}
	return ProjectiveShellImprint{Owners: multiple.Owners, Section: multiple.Sections[0], Complete: true}, nil
}

// ImprintProjectiveSplineShellFacePairs processes at most 64 distinct face
// pairs, with no original face selected twice for either owner. Every pair
// must have a complete supported section. Both closed shells are rebuilt
// only after all local proofs have passed; unresolved pairs publish nothing.
func (a Solid) ImprintProjectiveSplineShellFacePairs(ctx context.Context, b Solid, pairs []ProjectiveShellFacePair,
	o ProjectiveShellImprintOptions) (r ProjectiveMultiShellImprint, err error) {
	if ctx == nil || len(pairs) == 0 || len(pairs) > 64 || o.Sew.Sew.RepairOrientation {
		return r, geom.ErrInvalid
	}
	sources := [2]Solid{a, b}
	var seen [2]map[int]bool
	for owner, source := range sources {
		seen[owner] = make(map[int]bool, len(pairs))
		for _, pair := range pairs {
			face := pair.Faces[owner]
			if face < 0 || face >= len(source.faces) || seen[owner][face] {
				return r, geom.ErrInvalid
			}
			seen[owner][face] = true
		}
	}
	replacements := [2]map[int]projectiveShellReplacement{make(map[int]projectiveShellReplacement, len(pairs)), make(map[int]projectiveShellReplacement, len(pairs))}
	sections := make([]geom.ProjectiveContactSpline, len(pairs))
	for i, pair := range pairs {
		split, err := ImprintProjectiveSplineFacePair(ctx, a.faces[pair.Faces[0]], b.faces[pair.Faces[1]], pair.Axes, o.Faces)
		if err != nil {
			return r, err
		}
		for owner := range sources {
			replacements[owner][pair.Faces[owner]] = projectiveShellReplacement{split.Fragments[owner], split.Sources[owner], i}
		}
		sections[i] = split.Section
	}
	var owners [2]ProjectiveShellImprintOwner
	for i, source := range sources {
		limits := o.Sew
		if limits.Sew.DisplayTolerance == 0 {
			limits.Sew.DisplayTolerance = source.tolerance
		}
		owners[i], err = propagateProjectiveImprint(ctx, source, replacements[i], len(pairs), limits)
		if err != nil {
			return r, err
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Owners, r.Sections, r.Complete = owners, sections, true
	return r, nil
}

func propagateProjectiveImprint(ctx context.Context, source Solid, replacements map[int]projectiveShellReplacement, count int,
	o ExactSplineSewOptions) (r ProjectiveShellImprintOwner, err error) {
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
		if replacement, ok := replacements[i]; ok {
			for j, part := range replacement.fragments {
				index[i] = append(index[i], len(faces))
				firstV, firstE := len(vertices), len(edges)
				vertices = append(vertices, part.Vertices...)
				for k, e := range part.Edges {
					e.Start, e.End = firstV+e.Start, firstV+e.End
					edges = append(edges, e)
					ref := replacement.sources[j][k]
					row := projectiveShellRawEdge{source: ref.OriginalEdge, clock: ref.OriginalRange, seam: ref.Section, pair: replacement.pair}
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
	sections, occurrences := make([]int, count), make([]int, count)
	for i := range sections {
		sections[i] = -1
	}
	var lineage []ProjectiveShellImprintEdge
	for i, row := range report.Segments {
		original := history[i]
		if original.seam {
			prior := sections[original.pair]
			if len(row) != 1 || prior >= 0 && prior != row[0].Edge {
				return r, geom.ErrConvergence
			}
			sections[original.pair] = row[0].Edge
			occurrences[original.pair]++
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
	sectionSet := make(map[int]bool, count)
	for i, edge := range sections {
		if edge < 0 || occurrences[i] != 2 || sectionSet[edge] {
			return r, geom.ErrConvergence
		}
		sectionSet[edge] = true
	}
	r.Solid, r.SourceFaces, r.Edges, r.SectionEdges, r.InsertedBoundaries = solid, index, lineage, sections, report.InsertedVertices
	r.SectionEdge = -1
	if count == 1 {
		r.SectionEdge = sections[0]
	}
	return r, nil
}
