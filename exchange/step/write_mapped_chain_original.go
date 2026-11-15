package step

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteMappedChainRectangularOriginalShell exports the captured STEP graph of
// a mixed-basis shell with a supported original-edge partition, including two
// asynchronous cuts in one original model Bézier. Both
// the captured source and the staged canonical graph pass
// the complete proof. The authored model poles, weights, knot values, and
// directions survive, as do the original pcurve bases and parameter limits.
// A source-only native shell supplies normalized station evidence. This
// writer needs the captured STEP model for the authored 3D parameter scale.
func WriteMappedChainRectangularOriginalShell(ctx context.Context, w io.Writer, model MappedChainSTEPModel, l Limits, o MappedChainShellSTEPOptions) error {
	if ctx == nil || w == nil || model.original == "" || !o.allowsUnsplitMixedOwnerBases() {
		return geom.ErrInvalid
	}
	source, err := readMappedChainRectangularShell(ctx, strings.NewReader(model.original), l, o, model.foreign)
	if err != nil {
		return err
	}
	if !utf8.ValidString(source.Name) || !validLengthUnit(source.LengthUnit) {
		return geom.ErrInvalid
	}
	independent := source.Shell.IndependentOwnerBases()
	crossings := source.Crossings
	recordedPartition := len(crossings) >= 3 && len(crossings) <= 6 &&
		(allMappedChainCrossingsOnOneEdge(crossings) || mappedChainCrossingLocalCellCount(crossings) != 0 ||
			mappedChainSeparatedSupportCells(source.Chain, crossings))
	if independent[0] == independent[1] || !recordedPartition && !mappedChainSameCellAsynchronousCrossings(crossings) {
		return fmt.Errorf("original mixed STEP export requires a supported source-edge support-knot partition: %w", geom.ErrConvergence)
	}
	file, err := Parse(strings.NewReader(source.original), l)
	if err != nil {
		return err
	}
	original, err := originalMappedChainSTEPGraph(ctx, file, source)
	if err != nil {
		return err
	}
	raw, err := encodeMappedChainRectangularShell(ctx, source.Native, source.Name, source.LengthUnit, independent, original, o.Geometry)
	if err != nil {
		return err
	}
	return writeReprovedMappedChainSTEP(ctx, w, raw, l, o)
}

type mappedChainOriginalSTEPEdge struct {
	model      mappedChainStepSpline
	owners     [2]trimmed.SplineUV
	start, end int
	reversed   bool
}

type mappedChainOriginalSTEPGraph struct {
	// The first native child represents its complete authored source edge.
	edges           map[int]mappedChainOriginalSTEPEdge
	omittedEdges    map[int]bool
	omittedVertices map[int]bool
}

func originalMappedChainSTEPGraph(ctx context.Context, f *File, m MappedChainSTEPModel) (mappedChainOriginalSTEPGraph, error) {
	var zero mappedChainOriginalSTEPGraph
	g := mappedChainOriginalSTEPGraph{
		edges:           make(map[int]mappedChainOriginalSTEPEdge, len(m.Chain)),
		omittedEdges:    make(map[int]bool, len(m.Crossings)),
		omittedVertices: make(map[int]bool, len(m.Crossings)),
	}
	edges, vertices := m.Native.Solid.Edges(), m.Native.Solid.Vertices()
	next := 0
	for _, cell := range m.Chain {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		count := 1
		for _, crossing := range m.Crossings {
			if crossing.EdgeID == cell.EdgeID {
				count++
			}
		}
		if next+count > len(m.Native.ChainEdges) {
			return zero, geom.ErrConvergence
		}
		children := m.Native.ChainEdges[next : next+count]
		next += count
		start, end := edges[children[0]].Start, edges[children[len(children)-1]].End
		for i, child := range children[1:] {
			joint := edges[children[i]].End
			if joint != edges[child].Start {
				return zero, geom.ErrConvergence
			}
			g.omittedVertices[joint], g.omittedEdges[child] = true, true
		}
		original, err := originalMappedChainSTEPEdge(f, cell, m.foreign, start, end, vertices)
		if err != nil {
			return zero, fmt.Errorf("original mapped STEP edge #%d: %w", cell.EdgeID, err)
		}
		g.edges[children[0]] = original
	}
	if next != len(m.Native.ChainEdges) {
		return zero, geom.ErrConvergence
	}
	return g, nil
}

func originalMappedChainSTEPEdge(f *File, cell MappedChainSTEPCell, foreign bool, start, end int, vertices []geom.Vec3) (mappedChainOriginalSTEPEdge, error) {
	var zero mappedChainOriginalSTEPEdge
	first, last, _, err := analyticEdge(f, cell.EdgeID)
	if err != nil {
		return zero, err
	}
	var points [2]geom.Vec3
	for i, id := range []int64{first, last} {
		p, err := mappedChainStepVertex(f, id)
		if err != nil {
			return zero, err
		}
		points[i], err = geom.RepresentedExactPoint(p)
		if err != nil {
			return zero, err
		}
	}
	reversed := points == [2]geom.Vec3{vertices[end], vertices[start]}
	if reversed {
		start, end = end, start
	} else if points != [2]geom.Vec3{vertices[start], vertices[end]} {
		return zero, geom.ErrConvergence
	}
	model, err := readMappedChainStepSpline(f, cell.ModelCurveID, 3, 16, 33, foreign)
	if err != nil {
		return zero, err
	}
	out := mappedChainOriginalSTEPEdge{model: model, start: start, end: end, reversed: reversed}
	for i, owner := range cell.Owners {
		out.owners[i], err = originalMappedChainStepOwner(f, owner, foreign)
		if err != nil {
			return zero, err
		}
	}
	return out, nil
}

func originalMappedChainStepOwner(f *File, owner MappedChainSTEPCoedge, foreign bool) (trimmed.SplineUV, error) {
	var zero trimmed.SplineUV
	spline, err := readMappedChainStepSpline(f, owner.BasisID, 2, 4, 64, foreign)
	if err != nil {
		return zero, err
	}
	basis, err := spline.ownerBasis()
	if err != nil {
		return zero, err
	}
	lo, hi := basis.Domain()
	out := trimmed.SplineUV{Curve: basis, Range: trimmed.UVInterval{Min: lo, Max: hi}}
	if owner.TrimmedCurveID != 0 {
		id, raw, reverse, err := mappedChainStepRawUse(f, owner.TrimmedCurveID)
		if err != nil || id != owner.BasisID {
			return zero, geom.ErrInvalid
		}
		out.Range, out.Reversed = raw, reverse
	}
	return out, nil
}
