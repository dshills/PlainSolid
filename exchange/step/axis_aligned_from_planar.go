package step

import (
	"context"
	"errors"
	"io"
	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// AxisAlignedSplineShellsFromPlanar constructs exact degree-one spline charts
// from native axis-aligned orthogonal planar solids, including through-holes.
// It preserves explicit vertices/edges and offsets their IDs across bodies;
// no positional welding, fitting or curve approximation is performed. Each
// body must be one connected outward embedded shell with at most 64 total
// faces across the model. General planar oblique faces are not supported.
func AxisAlignedSplineShellsFromPlanar(ctx context.Context, m Model) (AxisAlignedSplineShellModel, error) {
	fail := func() (AxisAlignedSplineShellModel, error) {
		return AxisAlignedSplineShellModel{}, errors.New("step: unsupported planar-to-spline shell input")
	}
	if ctx == nil || len(m.Bodies) == 0 {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return AxisAlignedSplineShellModel{}, e
	}
	out := AxisAlignedSplineShellModel{LengthUnit: m.LengthUnit}
	vertexOffset, edgeOffset, faceOffset := int64(0), int64(0), int64(0)
	coord := func(p geom.Vec3, axis int) float64 {
		switch axis {
		case 0:
			return p.X
		case 1:
			return p.Y
		default:
			return p.Z
		}
	}
	for _, input := range m.Bodies {
		vertices, edges, faces := input.Solid.Vertices(), input.Solid.Edges(), input.Solid.Faces()
		if len(faces) < 4 || int64(len(faces))+faceOffset > MaxOrthogonalShellFaces {
			return fail()
		}
		if e := poly.ValidateEmbeddedBoundary(ctx, input.Solid, poly.Options{RequireExactConversion: true}); e != nil {
			return AxisAlignedSplineShellModel{}, e
		}
		canonical := make([]brep.Edge, len(edges))
		for i, edge := range edges {
			axis := -1
			for k := 0; k < 3; k++ {
				if coord(vertices[edge.Start], k) != coord(vertices[edge.End], k) {
					if axis != -1 {
						return fail()
					}
					axis = k
				}
			}
			if axis < 0 {
				return fail()
			}
			canonical[i] = edge
			if coord(vertices[edge.Start], axis) > coord(vertices[edge.End], axis) {
				canonical[i] = brep.Edge{Start: edge.End, End: edge.Start}
			}
		}
		body := AxisAlignedSplineShellBody{Name: input.Name, Solid: input.Solid, FaceHoles: make([][][]geom.Vec2, len(faces))}
		for fi, face := range faces {
			loops := append([][]brep.EdgeUse{face.Boundary}, face.Holes...)
			indices := make([][]int, len(loops))
			for li, loop := range loops {
				for _, use := range loop {
					edge := edges[use.Edge]
					start := edge.Start
					if use.Reversed {
						start = edge.End
					}
					indices[li] = append(indices[li], start)
				}
			}
			fixed := -1
			for k := 0; k < 3; k++ {
				same := true
				v := coord(vertices[indices[0][0]], k)
				for _, loop := range indices {
					for _, id := range loop {
						same = same && coord(vertices[id], k) == v
					}
				}
				if same {
					if fixed != -1 {
						return fail()
					}
					fixed = k
				}
			}
			if fixed < 0 {
				return fail()
			}
			u, v := (fixed+1)%3, (fixed+2)%3
			project := func(loop []int) []geom.Vec2 {
				p := make([]geom.Vec2, len(loop))
				for i, id := range loop {
					p[i] = geom.Vec2{X: coord(vertices[id], u), Y: coord(vertices[id], v)}
				}
				return p
			}
			boundary := project(indices[0])
			domain, e := orthogonalPolygon(boundary)
			if e != nil {
				u, v = v, u
				boundary = project(indices[0])
				domain, e = orthogonalPolygon(boundary)
				if e != nil {
					return fail()
				}
			}
			constant := coord(vertices[indices[0][0]], fixed)
			poles := make([]geom.Vec3, 4)
			for a := 0; a < 2; a++ {
				for b := 0; b < 2; b++ {
					p := [3]float64{}
					p[fixed] = constant
					p[u] = domain.UMin
					if a == 1 {
						p[u] = domain.UMax
					}
					p[v] = domain.VMin
					if b == 1 {
						p[v] = domain.VMax
					}
					poles[2*a+b] = geom.Vec3{X: p[0], Y: p[1], Z: p[2]}
				}
			}
			support, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{domain.UMin, domain.UMin, domain.UMax, domain.UMax}, []float64{domain.VMin, domain.VMin, domain.VMax, domain.VMax}, nil)
			if e != nil {
				return AxisAlignedSplineShellModel{}, e
			}
			body.Faces = append(body.Faces, OrthogonalSplineFace{Geometry: support, Boundary: boundary})
			body.FaceIDs = append(body.FaceIDs, faceOffset+int64(fi)+1)
			for li, loop := range loops {
				if li > 0 {
					physical := project(indices[li])
					positive := make([]geom.Vec2, len(physical))
					positive[0] = physical[0]
					for j := 1; j < len(physical); j++ {
						positive[j] = physical[len(physical)-j]
					}
					body.FaceHoles[fi] = append(body.FaceHoles[fi], positive)
				}
				for j, use := range loop {
					edge := canonical[use.Edge]
					body.Uses = append(body.Uses, OrthogonalShellUse{Face: fi, Loop: li, EdgeID: edgeOffset + int64(use.Edge) + 1, StartVertexID: vertexOffset + int64(edge.Start) + 1, EndVertexID: vertexOffset + int64(edge.End) + 1, Forward: indices[li][j] == edge.Start})
				}
			}
		}
		out.Bodies = append(out.Bodies, body)
		vertexOffset += int64(len(vertices))
		edgeOffset += int64(len(edges))
		faceOffset += int64(len(faces))
	}
	if e := ctx.Err(); e != nil {
		return AxisAlignedSplineShellModel{}, e
	}
	if e := WriteAxisAlignedSplineShells(io.Discard, out); e != nil {
		return AxisAlignedSplineShellModel{}, e
	}
	if e := ctx.Err(); e != nil {
		return AxisAlignedSplineShellModel{}, e
	}
	return out, nil
}
