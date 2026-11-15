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
	p, err := readExactCircularLoopDialect(ctx, r, l, false)
	if err != nil {
		return ExactCircularLoopExtrusionProduct{}, err
	}
	return ExactCircularLoopExtrusionProduct{
		Descriptor: ExactCircularLoopExtrusion{Name: p.Descriptor.Name, Frame: p.Descriptor.Frame, Segments: p.Descriptor.Outer, Height: p.Descriptor.Height, LengthUnit: p.Descriptor.LengthUnit},
		Native:     p.Native, Source: p.Source, Body: p.Body, Shell: p.Shell, Caps: p.Caps, Walls: p.Walls, VerticalEdges: p.VerticalEdges, Rims: p.Rims, Vertices: p.Vertices,
	}, nil
}

// ExactDirectedCircularLoopExtrusionProduct binds each original ring to the
// native solid. RingOffsets delimit outer and inner wires in the original rim,
// vertex, wall, and vertical edge arrays.
type ExactDirectedCircularLoopExtrusionProduct struct {
	Descriptor           ExactDirectedCircularLoopExtrusion
	Native               trimmed.Solid
	Source               *File
	Body, Shell          int64
	Caps                 [2]int64
	Walls, VerticalEdges []int64
	Rims, Vertices       [2][]int64
	RingOffsets          []int
}

// ReadExactDirectedCircularLoopExtrusion reads the canonical rooted analytic
// dialect, re-proves all original directed wires, and authenticates every
// shared edge use and support against the reconstructed STEP graph.
func ReadExactDirectedCircularLoopExtrusion(ctx context.Context, r io.Reader, l Limits) (ExactDirectedCircularLoopExtrusionProduct, error) {
	return readExactCircularLoopDialect(ctx, r, l, true)
}

func readExactCircularLoopDialect(ctx context.Context, r io.Reader, l Limits, directed bool) (ExactDirectedCircularLoopExtrusionProduct, error) {
	var zero ExactDirectedCircularLoopExtrusionProduct
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
	out := ExactDirectedCircularLoopExtrusionProduct{Source: f, Body: bodies[0], Shell: body.Args[1].Ref, Caps: [2]int64{ids[0], ids[1]}, Walls: append([]int64(nil), ids[2:]...)}
	out.Descriptor = ExactDirectedCircularLoopExtrusion{Name: name, Frame: frame, Height: height, LengthUnit: unit}
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
		rings, err := exactLoopRimSourceUses(f, ids[j], n, directed)
		if err != nil {
			return zero, err
		}
		if j == 0 {
			out.RingOffsets = []int{0}
			out.Descriptor.Holes = make([][]trimmed.ExactCircularLoopSegment, len(rings)-1)
		} else if len(rings)+1 != len(out.RingOffsets) {
			return zero, geom.ErrDomain
		}
		for ri, uses := range rings {
			if j == 1 && len(uses) != out.RingOffsets[ri+1]-out.RingOffsets[ri] {
				return zero, geom.ErrDomain
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
				if ri == 0 {
					out.Descriptor.Outer = append(out.Descriptor.Outer, segment)
				} else {
					out.Descriptor.Holes[ri-1] = append(out.Descriptor.Holes[ri-1], segment)
				}
			}
			if j == 0 {
				out.RingOffsets = append(out.RingOffsets, len(out.Rims[0]))
			}
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
	if e = writeExactCircularLoopDialect(ctx, &expected, out.Descriptor, directed, nil); e != nil {
		return zero, e
	}
	canonical, e := Parse(&expected, l)
	if e != nil {
		return zero, e
	}
	if e = sideDiskMatchWholeGraph(f, canonical); e != nil {
		return zero, e
	}
	if directed {
		out.Native, e = trimmed.ExtrudeExactDirectedCircularLoops(ctx, out.Descriptor.Outer, out.Descriptor.Holes, lo.Z, hi.Z, trimmed.ExactCircularLoopOptions{})
	} else {
		out.Native, e = trimmed.ExtrudeExactCircularLoop(ctx, out.Descriptor.Outer, height, trimmed.ExactCircularLoopOptions{})
	}
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
func exactLoopSourceUses(f *File, fid int64, n int) ([]int64, error) {
	loops, err := exactLoopRimSourceUses(f, fid, n, false)
	if err != nil {
		return nil, err
	}
	return loops[0], nil
}

func exactLoopRimSourceUses(f *File, fid int64, n int, directed bool) ([][]int64, error) {
	face, err := simple(f, fid, "ADVANCED_FACE", 4)
	if err != nil {
		return nil, err
	}
	ids, err := listRefs(face.Args[1], 1)
	if err != nil || len(ids) > 21 || (!directed && len(ids) != 1) {
		return nil, geom.ErrDomain
	}
	loops := make([][]int64, len(ids))
	total := 0
	maximum := n
	if maximum == 0 && directed {
		maximum = 64
	}
	for i, id := range ids {
		kind := "FACE_OUTER_BOUND"
		if i > 0 {
			kind = "FACE_BOUND"
		}
		b, err := simple(f, id, kind, 3)
		if err != nil {
			return nil, err
		}
		loop, err := simple(f, b.Args[1].Ref, "EDGE_LOOP", 2)
		if err != nil {
			return nil, err
		}
		uses, err := listRefs(loop.Args[1], 3)
		if err != nil || len(uses) > maximum-total {
			return nil, geom.ErrDomain
		}
		total += len(uses)
		for _, k := range uses {
			u, err := simple(f, k, "ORIENTED_EDGE", 5)
			if err != nil {
				return nil, err
			}
			loops[i] = append(loops[i], u.Args[3].Ref)
		}
	}
	if n != 0 && total != n {
		return nil, geom.ErrDomain
	}
	return loops, nil
}
