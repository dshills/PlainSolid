package trimmed

import (
	"context"
	"fmt"
	"math"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SewPlanarCapPrism joins an outward axial prism or cuts an inward one at
// an upper planar cap. The prism has congruent convex caps, parallel to the
// stock cap. Its volume must lie strictly clear of every other stock face.
// The stock face retains its index and the other tool cap becomes a new face.
func SewPlanarCapPrism(ctx context.Context, stock, tool Solid, stockCap, toolLower, toolUpper int, cut bool) (Solid, int, error) {
	return sewPlanarCapPrism(ctx, stock, tool, stockCap, toolLower, toolUpper, cut, true)
}

func sewPlanarCapPrism(ctx context.Context, stock, tool Solid, stockCap, toolLower, toolUpper int, cut, build bool) (Solid, int, error) {
	fail := func(err error) (Solid, int, error) { return Solid{}, -1, err }
	if ctx == nil || stockCap < 0 || stockCap >= len(stock.faces) || toolLower < 0 || toolLower >= len(tool.faces) ||
		toolUpper < 0 || toolUpper >= len(tool.faces) || toolLower == toolUpper {
		return fail(geom.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if len(stock.Topology().Shells) != 1 || len(tool.Topology().Shells) != 1 {
		return fail(geom.ErrDomain)
	}
	source, lower, upper := stock.faces[stockCap], tool.faces[toolLower], tool.faces[toolUpper]
	support, plane := source.Surface.(geom.PlaneSurface)
	low, lowPlane := lower.Surface.(geom.PlaneSurface)
	high, highPlane := upper.Surface.(geom.PlaneSurface)
	if !plane || !lowPlane || !highPlane || source.Reversed || !lower.Reversed || upper.Reversed ||
		len(lower.Holes) != 0 || len(upper.Holes) != 0 || len(lower.Outer) != len(upper.Outer) {
		return fail(geom.ErrDomain)
	}
	for i, u := range lower.Outer {
		v := upper.Outer[i]
		if u.UVStart != v.UVStart || u.UVEnd != v.UVEnd || !reflect.DeepEqual(u.Curve, v.Curve) {
			return fail(geom.ErrDomain)
		}
	}
	frame := support.Frame()
	if !prismCardinal(frame) {
		return fail(geom.ErrDomain)
	}
	for _, f := range []geom.Frame{low.Frame(), high.Frame()} {
		if f.XAxis() != frame.XAxis() || f.YAxis() != frame.YAxis() || f.ZAxis() != frame.ZAxis() {
			return fail(geom.ErrDomain)
		}
	}
	a, err := prismExactLocal(frame, low.Frame().Origin())
	if err != nil {
		return fail(err)
	}
	b, err := prismExactLocal(frame, high.Frame().Origin())
	if err != nil {
		return fail(err)
	}
	if a.X != b.X || a.Y != b.Y || a.Z >= b.Z || cut && b.Z != 0 || !cut && a.Z != 0 {
		return fail(geom.ErrDomain)
	}
	if err = prismConvexWire(lower.Outer); err != nil {
		return fail(err)
	}
	lo, hi, err := prismWireBounds(lower.Outer)
	if err != nil {
		return fail(err)
	}
	pad := 8 * max(stock.tolerance, tool.tolerance)
	loCap := geom.Vec2{X: math.Nextafter(lo.X+a.X, math.Inf(-1)), Y: math.Nextafter(lo.Y+a.Y, math.Inf(-1))}
	hiCap := geom.Vec2{X: math.Nextafter(hi.X+a.X, math.Inf(1)), Y: math.Nextafter(hi.Y+a.Y, math.Inf(1))}
	var disk *prismDisk
	if c, ok := prismWholeCircle(lower.Outer); ok {
		center := low.Frame().ToWorld(geom.Vec3{X: c.Center.X, Y: c.Center.Y})
		if p, err := prismExactLocal(low.Frame(), center); err == nil && p == (geom.Vec3{X: c.Center.X, Y: c.Center.Y}) {
			disk = &prismDisk{center: center, axis: frame.ZAxis(), radius: c.Cos.X}
		}
	}
	if err = prismCapContains(source, loCap, hiCap, disk.inFrame(frame), pad); err != nil {
		return fail(fmt.Errorf("cap prism footprint: %w", err))
	}
	u, v := parametric.Interval{Min: lo.X, Max: hi.X}, parametric.Interval{Min: lo.Y, Max: hi.Y}
	lowBox, err := parametric.SurfaceBounds(low, u, v)
	if err != nil {
		return fail(err)
	}
	highBox, err := parametric.SurfaceBounds(high, u, v)
	if err != nil {
		return fail(err)
	}
	worldBox, err := lowBox.Union(highBox)
	if err != nil {
		return fail(err)
	}
	corners := make([]geom.Vec3, 0, 8)
	for _, x := range []float64{worldBox.Min.X, worldBox.Max.X} {
		for _, y := range []float64{worldBox.Min.Y, worldBox.Max.Y} {
			for _, z := range []float64{worldBox.Min.Z, worldBox.Max.Z} {
				corners = append(corners, geom.Vec3{X: x, Y: y, Z: z})
			}
		}
	}
	for i, face := range stock.faces {
		if err = ctx.Err(); err != nil {
			return fail(err)
		}
		if i != stockCap && !prismMissesFace(face, corners, worldBox, disk, pad) {
			return fail(fmt.Errorf("cap prism stock face %d (%T): %w", i, face.Surface, geom.ErrDomain))
		}
	}
	for i, face := range tool.faces {
		if i == toolLower || i == toolUpper {
			continue
		}
		if !axialCapPrismWall(tool, face, low.Frame(), b.Z-a.Z) {
			return fail(geom.ErrDomain)
		}
	}

	if !build {
		return Solid{}, -1, nil
	}
	omit, other := toolLower, toolUpper
	if cut {
		omit, other = toolUpper, toolLower
	}
	stock, err = prismRefineCapSeed(ctx, stock, stockCap, loCap, hiCap, disk.inFrame(frame), pad)
	if err != nil {
		return fail(err)
	}
	return stitchCapPrism(ctx, stock, tool, stockCap, omit, other, geom.Vec2{X: a.X, Y: a.Y}, cut)
}

func axialCapPrismWall(tool Solid, face Face, frame geom.Frame, height float64) bool {
	if len(face.Holes) != 0 || len(face.Outer) != 4 {
		return false
	}
	switch surface := face.Surface.(type) {
	case geom.PlaneSurface:
		if prismExactDot(surface.Frame().ZAxis(), frame.ZAxis()).Sign() != 0 {
			return false
		}
	case geom.CylinderSurface:
		f, _ := surface.Parameters()
		if f.ZAxis() != frame.ZAxis() && f.ZAxis() != frame.ZAxis().Scale(-1) {
			return false
		}
	default:
		return false
	}
	low, high := 0, 0
	for _, use := range face.Outer {
		if use.Curve != nil {
			return false
		}
		edge := tool.edges[use.Edge]
		i := edge.Start
		if use.Reversed {
			i = edge.End
		}
		p, err := prismExactLocal(frame, tool.vertices[i])
		if err != nil {
			return false
		}
		switch p.Z {
		case 0:
			low++
		case height:
			high++
		default:
			return false
		}
	}
	return low == 2 && high == 2
}

func stitchCapPrism(ctx context.Context, stock, tool Solid, stockCap, omit, other int, offset geom.Vec2, cut bool) (Solid, int, error) {
	vertices := append(stock.Vertices(), tool.Vertices()...)
	edges, faces := stock.Edges(), stock.Faces()
	edgeOffset, vertexOffset := len(edges), len(stock.vertices)
	for _, edge := range tool.edges {
		edge.Start += vertexOffset
		edge.End += vertexOffset
		edges = append(edges, edge)
	}
	wire, err := prismShiftWire(tool.faces[omit].Outer, offset, edgeOffset)
	if err != nil {
		return Solid{}, -1, err
	}
	hole := make([]Use, len(wire))
	for i, u := range wire {
		u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
		u.Reversed = !u.Reversed
		u.Curve = reversePCurve(u.Curve)
		hole[len(wire)-1-i] = u
	}
	faces[stockCap].Holes = append(faces[stockCap].Holes, hole)
	newCap := -1
	for i, face := range tool.Faces() {
		if i == omit {
			continue
		}
		if i == other {
			newCap = len(faces)
		}
		if cut {
			face.Reversed = !face.Reversed
		}
		for j := range face.Outer {
			face.Outer[j].Edge += edgeOffset
		}
		faces = append(faces, face)
	}
	result, err := NewSolidContext(ctx, vertices, edges, faces, max(stock.tolerance, tool.tolerance))
	return result, newCap, err
}
