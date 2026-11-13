package feature

import (
	"context"
	"errors"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

var ErrConflict = errors.New("feature: sketch tools overlap or touch")

// BatchOptions bounds profile count (default 8, maximum 64), per-profile
// validation, pairwise exact tool distance and the final Boolean independently.
type BatchOptions struct {
	MaxProfiles int
	Feature     Options
	Distance    collision.DistanceOptions
}

// ProfileMap resolves each original profile's tool caps and wall faces.
type ProfileMap struct {
	Mouth, Terminal int
	SketchWalls     []int
}

// BatchResult maps Boolean operand B to the disjoint compound Tool. ToolFaces
// and ToolEdges give the profile index owning each compound entity. Profiles
// preserve caller order. Operand A remains the original base.
type BatchResult struct {
	Solid, Tool          brep.Solid
	History              poly.TopologyHistory
	ToolFaces, ToolEdges []int
	Profiles             []ProfileMap
}

// ExtrudeProfiles applies one common offset and kind to disjoint interior
// profiles on the same base face. Exact positive boundary separation is required
// between tools. Since every tool starts on the same plane, nested or overlapping
// footprints also have intersecting caps and are rejected. No partial result is
// returned on any invalid profile, conflict, cancellation or exhausted budget.
func ExtrudeProfiles(ctx context.Context, base brep.Solid, face int, profiles [][]geom.Vec3, offset geom.Vec3, kind Kind, o BatchOptions) (BatchResult, error) {
	empty := BatchResult{}
	if ctx == nil || o.MaxProfiles < 0 || o.MaxProfiles > 64 || len(profiles) == 0 {
		return empty, ErrInvalid
	}
	if o.MaxProfiles == 0 {
		o.MaxProfiles = 8
	}
	if len(profiles) > o.MaxProfiles {
		return empty, ErrLimit
	}
	var tools []Result
	for _, p := range profiles {
		r, e := extrude(ctx, base, face, -1, p, offset, kind, o.Feature, true)
		if e != nil {
			return empty, e
		}
		tools = append(tools, r)
	}
	for i := range tools {
		for j := 0; j < i; j++ {
			r, e := collision.BoundaryDistance(ctx, tools[i].Tool, tools[j].Tool, o.Distance)
			if e != nil {
				return empty, e
			}
			if r.SquaredDistance.Sign() == 0 {
				return empty, ErrConflict
			}
		}
	}
	var vertices []geom.Vec3
	var edges []brep.Edge
	var faces []brep.Face
	result := BatchResult{}
	for i, t := range tools {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		vo, eo, fo := len(vertices), len(edges), len(faces)
		vertices = append(vertices, t.Tool.Vertices()...)
		for _, e := range t.Tool.Edges() {
			edges = append(edges, brep.Edge{Start: e.Start + vo, End: e.End + vo})
			result.ToolEdges = append(result.ToolEdges, i)
		}
		for _, f := range t.Tool.Faces() {
			for j := range f.Boundary {
				f.Boundary[j].Edge += eo
			}
			faces = append(faces, f)
			result.ToolFaces = append(result.ToolFaces, i)
		}
		m := ProfileMap{Mouth: fo + t.Mouth, Terminal: fo + t.Terminal}
		for _, wall := range t.SketchWalls {
			m.SketchWalls = append(m.SketchWalls, fo+wall)
		}
		result.Profiles = append(result.Profiles, m)
	}
	tool, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return empty, e
	}
	op := poly.Union
	if kind == Pocket {
		op = poly.Difference
	}
	solid, history, e := poly.CombineWithTopologyHistory(ctx, base, tool, op, o.Feature.Boolean)
	if e != nil {
		return empty, e
	}
	result.Solid, result.Tool, result.History = solid, tool, history
	return result, nil
}
