package iges

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// NodalCubicIGESOptions controls exact coedge reproof and decimal
// representation of derived model and pcurve poles.
type NodalCubicIGESOptions = ClippedPolynomialFuseIGESOptions

// WriteNodalCubicCommon transfers the original Common as one closed type
// 186 solid with exact node rays, paired coedges and the original roof seam.
func WriteNodalCubicCommon(ctx context.Context, w io.Writer, common *trimmed.NodalCubicCommon, unit float64, l Limits, o NodalCubicIGESOptions) error {
	if common == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGES(ctx, w, common.Boundary(), unit, l, o)
}

// VerifyNodalCubicCommonIGES authenticates the topology, original owners,
// coedges, units and settings against its native Common proof.
func VerifyNodalCubicCommonIGES(ctx context.Context, input io.Reader, common *trimmed.NodalCubicCommon, unit float64, l Limits, o NodalCubicIGESOptions) error {
	if common == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, input, l, "nodal cubic Common", func(w io.Writer) error {
		return WriteNodalCubicCommon(ctx, w, common, unit, l, o)
	})
}

// WriteNodalCubicFuse transfers the exact original Fuse as one closed type
// 186 solid and retains the original support and coedge for each roof sector.
func WriteNodalCubicFuse(ctx context.Context, w io.Writer, fuse *trimmed.NodalCubicFuse, unit float64, l Limits, o NodalCubicIGESOptions) error {
	if fuse == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGES(ctx, w, fuse.Boundary(), unit, l, o)
}

// VerifyNodalCubicFuseIGES authenticates the topology, original owners,
// coedges, units and settings against its native Fuse proof.
func VerifyNodalCubicFuseIGES(ctx context.Context, input io.Reader, fuse *trimmed.NodalCubicFuse, unit float64, l Limits, o NodalCubicIGESOptions) error {
	if fuse == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, input, l, "nodal cubic Fuse", func(w io.Writer) error {
		return WriteNodalCubicFuse(ctx, w, fuse, unit, l, o)
	})
}

// WriteNodalCubicCut transfers the separate original loop and arm solids
// with one type 186 entity and one vertex table for each component.
func WriteNodalCubicCut(ctx context.Context, w io.Writer, cut *trimmed.NodalCubicCut, unit float64, l Limits, o NodalCubicIGESOptions) error {
	parts := cut.Components()
	if len(parts) != 2 {
		return geom.ErrInvalid
	}
	p := cut.Contact()
	contacts := []trimmed.PolynomialMixedCutContact{{TwoPolynomialNodeComponentContact: trimmed.TwoPolynomialNodeComponentContact{
		Point: p.Point, Components: [2]int{0, 1}, Vertices: p.Vertices,
	}}}
	return writeClippedCutIGESBodies(ctx, w, parts, contacts, unit, l, o)
}

// VerifyNodalCubicCutIGES authenticates the grouped topology and exact
// component-local contact occurrences against the native Cut proof.
func VerifyNodalCubicCutIGES(ctx context.Context, input io.Reader, cut *trimmed.NodalCubicCut, unit float64, l Limits, o NodalCubicIGESOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, input, l, func(w io.Writer) error {
		return WriteNodalCubicCut(ctx, w, cut, unit, l, o)
	})
}
