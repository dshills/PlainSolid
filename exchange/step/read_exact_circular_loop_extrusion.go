package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ExactCircularLoopExtrusionProduct binds original role IDs to the reconstructed
// native topology. Caps are low/high along the local axis. Rim and vertex slices
// follow the original profile order; walls and verticals share that same order.
// Native is product-local; Descriptor.Frame carries its exact proper placement.
type ExactCircularLoopExtrusionProduct struct {
	Descriptor           ExactCircularLoopExtrusion
	Native               trimmed.Solid
	Source               *File
	Body, Shell          int64
	Caps                 [2]int64
	Walls, VerticalEdges []int64
	Rims, Vertices       [2][]int64
}

// ReadExactCircularLoopExtrusion accepts the rooted model-only writer dialect.
// It recovers signed height from original wall and cap placements, proves the
// original loop through exact native construction, then authenticates every
// record and shared reference against the reconstructed analytic graph.
// Entity numbering/order and exact equivalent decimal spelling may differ;
// alternate topology, rounded values, pcurves and metadata are not inferred.
func ReadExactCircularLoopExtrusion(ctx context.Context, r io.Reader, l Limits) (ExactCircularLoopExtrusionProduct, error) {
	var zero ExactCircularLoopExtrusionProduct
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, e
	}
	f, _, unit, bodies, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil {
		return zero, e
	}
	if len(bodies) != 1 {
		return zero, geom.ErrDomain
	}
	body, e := simple(f, bodies[0], "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return zero, e
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return zero, e
	}
	shell, e := simple(f, body.Args[1].Ref, "CLOSED_SHELL", 2)
	if e != nil {
		return zero, e
	}
	ids, e := listRefs(shell.Args[1], 5)
	if e != nil || len(ids) > 66 {
		return zero, geom.ErrDomain
	}
	n := len(ids) - 2
	support := func(fid int64) (Record, error) {
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return Record{}, e
		}
		en := f.Entities[face.Args[2].Ref]
		if len(en.Records) != 1 {
			return Record{}, geom.ErrInvalid
		}
		return en.Records[0], nil
	}
	var caps [2]geom.Frame
	for j := 0; j < 2; j++ {
		s, e := support(ids[j])
		if e != nil || s.Type != "PLANE" || len(s.Args) != 2 {
			return zero, geom.ErrDomain
		}
		caps[j], e = sideDiskReadFrame(f, s.Args[1].Ref)
		if e != nil {
			return zero, e
		}
	}
	if caps[0].XAxis() != caps[1].XAxis() || caps[0].ZAxis() != caps[1].ZAxis() {
		return zero, geom.ErrDomain
	}
	// Every wall support is authored at extrusion station zero. Its axial
	// projection identifies the signed entry cap without guessing from names.
	s, e := support(ids[2])
	if e != nil || (s.Type != "PLANE" && s.Type != "CYLINDRICAL_SURFACE") || len(s.Args) < 2 {
		return zero, geom.ErrDomain
	}
	placement, e := simple(f, s.Args[1].Ref, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return zero, e
	}
	wallOrigin, e := splitTriple(f, placement.Args[1].Ref, "CARTESIAN_POINT")
	if e != nil {
		return zero, e
	}
	low := caps[0].Origin()
	axis := caps[0].ZAxis()
	a := []float64{axis.X, axis.Y, axis.Z}
	p := []float64{low.X, low.Y, low.Z}
	v := []float64{wallOrigin.X, wallOrigin.Y, wallOrigin.Z}
	offset := new(big.Rat)
	for j := 0; j < 3; j++ {
		offset.Add(offset, new(big.Rat).Mul(sideRat(a[j]), new(big.Rat).Sub(sideRat(v[j]), sideRat(p[j]))))
	}
	var origin [3]float64
	for j := range origin {
		q := new(big.Rat).Add(sideRat(p[j]), new(big.Rat).Mul(sideRat(a[j]), offset))
		x, ok := q.Float64()
		if !ok {
			return zero, geom.ErrUnrepresentable
		}
		origin[j] = x
	}
	frame, e := geom.NewFrame(geom.Vec3{X: origin[0], Y: origin[1], Z: origin[2]}, caps[0].XAxis(), axis)
	if e != nil {
		return zero, e
	}
	lo, e := sideDiskLocal(frame, caps[0].Origin())
	if e != nil {
		return zero, e
	}
	hi, e := sideDiskLocal(frame, caps[1].Origin())
	if e != nil {
		return zero, e
	}
	if lo.X != 0 || lo.Y != 0 || hi.X != 0 || hi.Y != 0 || lo.Z >= hi.Z {
		return zero, geom.ErrDomain
	}
	height := hi.Z
	if lo.Z != 0 {
		if hi.Z != 0 {
			return zero, geom.ErrDomain
		}
		height = lo.Z
	}
	out := ExactCircularLoopExtrusionProduct{Source: f, Body: bodies[0], Shell: body.Args[1].Ref, Caps: [2]int64{ids[0], ids[1]}, Walls: append([]int64(nil), ids[2:]...)}
	out.Descriptor = ExactCircularLoopExtrusion{Name: name, Frame: frame, Height: height, LengthUnit: unit}
	point := func(id int64) (geom.Vec3, error) {
		v, e := simple(f, id, "VERTEX_POINT", 2)
		if e != nil {
			return geom.Vec3{}, e
		}
		p, e := splitTriple(f, v.Args[1].Ref, "CARTESIAN_POINT")
		if e != nil {
			return geom.Vec3{}, e
		}
		return sideDiskLocal(frame, p)
	}
	for j := 0; j < 2; j++ {
		uses, e := exactLoopSourceUses(f, ids[j], n)
		if e != nil {
			return zero, e
		}
		for _, eid := range uses {
			edge, e := simple(f, eid, "EDGE_CURVE", 5)
			if e != nil {
				return zero, e
			}
			out.Rims[j] = append(out.Rims[j], eid)
			out.Vertices[j] = append(out.Vertices[j], edge.Args[1].Ref)
			if j != 0 {
				continue
			}
			start, e := point(edge.Args[1].Ref)
			if e != nil {
				return zero, e
			}
			end, e := point(edge.Args[2].Ref)
			if e != nil {
				return zero, e
			}
			if start.Z != lo.Z || end.Z != lo.Z {
				return zero, geom.ErrDomain
			}
			segment := trimmed.ExactCircularLoopSegment{Start: geom.Vec2{X: start.X, Y: start.Y}, End: geom.Vec2{X: end.X, Y: end.Y}}
			curve := f.Entities[edge.Args[3].Ref]
			if len(curve.Records) != 1 {
				return zero, geom.ErrDomain
			}
			c := curve.Records[0]
			switch c.Type {
			case "LINE":
				if len(c.Args) != 3 {
					return zero, geom.ErrDomain
				}
			case "CIRCLE":
				if len(c.Args) != 3 {
					return zero, geom.ErrDomain
				}
				cf, e := sideDiskReadFrame(f, c.Args[1].Ref)
				if e != nil {
					return zero, e
				}
				center, e := sideDiskLocal(frame, cf.Origin())
				if e != nil || center.Z != lo.Z {
					return zero, geom.ErrDomain
				}
				segment.Center = geom.Vec2{X: center.X, Y: center.Y}
				segment.Radius, e = cylinderSupportNumber(c.Args[2])
				if e != nil || segment.Radius <= 0 {
					return zero, geom.ErrDomain
				}
			default:
				return zero, geom.ErrDomain
			}
			out.Descriptor.Segments = append(out.Descriptor.Segments, segment)
		}
	}
	for _, fid := range ids[2:] {
		uses, e := exactLoopSourceUses(f, fid, 4)
		if e != nil {
			return zero, e
		}
		out.VerticalEdges = append(out.VerticalEdges, uses[3])
	}
	var expected bytes.Buffer
	if e = WriteExactCircularLoopExtrusion(ctx, &expected, out.Descriptor); e != nil {
		return zero, e
	}
	canonical, e := Parse(&expected, l)
	if e != nil {
		return zero, e
	}
	if e = sideDiskMatchWholeGraph(f, canonical); e != nil {
		return zero, e
	}
	out.Native, e = trimmed.ExtrudeExactCircularLoop(ctx, out.Descriptor.Segments, height, trimmed.ExactCircularLoopOptions{})
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
func exactLoopSourceUses(f *File, fid int64, n int) ([]int64, error) {
	face, e := simple(f, fid, "ADVANCED_FACE", 4)
	if e != nil {
		return nil, e
	}
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) != 1 {
		return nil, geom.ErrDomain
	}
	b, e := simple(f, bounds[0], "FACE_OUTER_BOUND", 3)
	if e != nil {
		return nil, e
	}
	loop, e := simple(f, b.Args[1].Ref, "EDGE_LOOP", 2)
	if e != nil {
		return nil, e
	}
	uses, e := listRefs(loop.Args[1], n)
	if e != nil || len(uses) != n {
		return nil, geom.ErrDomain
	}
	var ids []int64
	for _, id := range uses {
		u, e := simple(f, id, "ORIENTED_EDGE", 5)
		if e != nil {
			return nil, e
		}
		ids = append(ids, u.Args[3].Ref)
	}
	return ids, nil
}
