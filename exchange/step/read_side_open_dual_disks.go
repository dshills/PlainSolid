package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// SideOpenDualDiskProduct contains a completely reproved native body and the
// recovered source descriptor. The local origin is normalized to the stock's
// lower corner; original frame/rectangle origin offsets are not uniquely encoded.
// Tolerance is the caller's native validation policy, not source STEP metadata.
// Source face/edge IDs are distinct from the reconstructed native topology IDs.
type SideOpenDualDiskProduct struct {
	Descriptor   SideOpenDualDiskDescriptor
	Native       trimmed.SideOpenDualDiskCut
	Body, Shell  int64
	Faces, Edges []int64
	// ReversedModelLines lists source EDGE_CURVE IDs normalized by the opt-in
	// exact paired line-direction/sense proof. Empty under the canonical reader.
	ReversedModelLines []int64
	// RephasedModelCircles lists source EDGE_CURVE IDs with a proved equivalent
	// cardinal circle reference axis normalized by the opt-in reader.
	RephasedModelCircles []int64
	// OppositeNormalModelCircles lists source circle EDGE_CURVE IDs whose
	// paired normal/sense reversal was exactly proved and normalized.
	OppositeNormalModelCircles []int64
	// OppositeNormalPlanes lists source ADVANCED_FACE IDs whose exact paired
	// plane-normal/face-sense reversal was verified and normalized.
	OppositeNormalPlanes []int64
}

// ReadSideOpenDualDiskProduct accepts only the rooted model-only dialect emitted
// by WriteSideOpenDualDiskProduct. It recovers exact geometry, reconstructs the
// native material, and compares the entire entity graph, including sharing,
// senses and product metadata. Equivalent decimal spellings and entity numbering
// are accepted; alternate face partitions, pcurves and sense encodings are not.
func ReadSideOpenDualDiskProduct(ctx context.Context, r io.Reader, l Limits, tolerance float64) (SideOpenDualDiskProduct, error) {
	return ReadSideOpenDualDiskProductWithOptions(ctx, r, l, tolerance, SideOpenDualDiskReadOptions{})
}

// ReadSideOpenDualDiskProductWithOptions adds only the explicitly selected,
// independently verified equivalent encodings to the canonical reader.
func ReadSideOpenDualDiskProductWithOptions(ctx context.Context, r io.Reader, l Limits, tolerance float64, options SideOpenDualDiskReadOptions) (SideOpenDualDiskProduct, error) {
	result, err := readSideOpenDualDiskProduct(ctx, r, l, tolerance, options, sideDiskEqual)
	return result.SideOpenDualDiskProduct, err
}

type sideDiskReadResult struct {
	SideOpenDualDiskProduct
	unequalNative   trimmed.UnequalSideOpenDualDiskCut
	generalNative   trimmed.GeneralSideOpenDualDiskCut
	exactWallNative trimmed.GeneralSideOpenDualDiskCut
	offsetNative    trimmed.OffsetSideOpenDualDiskCut
}

func readSideOpenDualDiskProduct(ctx context.Context, r io.Reader, l Limits, tolerance float64, options SideOpenDualDiskReadOptions, kind sideDiskConstruction) (sideDiskReadResult, error) {
	fail := func() (sideDiskReadResult, error) {
		return sideDiskReadResult{}, fmt.Errorf("step: unsupported exact side-open dual-disk product")
	}
	if ctx == nil || r == nil || math.IsNaN(tolerance) || math.IsInf(tolerance, 0) || tolerance <= 0 {
		return fail()
	}
	if err := ctx.Err(); err != nil {
		return sideDiskReadResult{}, err
	}
	// Bound resource use even when the caller supplies permissive parser limits.
	raw, err := io.ReadAll(io.LimitReader(r, (1<<20)+1))
	if err != nil {
		return sideDiskReadResult{}, err
	}
	if len(raw) > 1<<20 {
		return fail()
	}
	f, _, unit, bodies, err := parseCanonicalCircularExtrusions(bytes.NewReader(raw), l)
	if err != nil {
		return sideDiskReadResult{}, err
	}
	if len(bodies) != 1 || len(f.Entities) > 2048 {
		return fail()
	}
	b, err := simple(f, bodies[0], "MANIFOLD_SOLID_BREP", 2)
	if err != nil {
		return fail()
	}
	name, err := textValue(b.Args[0])
	if err != nil {
		return fail()
	}
	shellID, err := ref(b.Args[1])
	if err != nil {
		return fail()
	}
	shell, err := simple(f, shellID, "CLOSED_SHELL", 2)
	if err != nil {
		return fail()
	}
	faces, err := listRefs(shell.Args[1], 1)
	if err != nil || (len(faces) != 9 && len(faces) != 16) {
		return fail()
	}
	var frames []geom.Frame
	var radii []float64
	for _, id := range faces {
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		en, ok := f.Entities[sid]
		if !ok || len(en.Records) != 1 {
			return fail()
		}
		switch en.Records[0].Type {
		case "PLANE":
		case "CYLINDRICAL_SURFACE":
			rec, e := simple(f, sid, "CYLINDRICAL_SURFACE", 3)
			if e != nil {
				return fail()
			}
			pid, e := ref(rec.Args[1])
			if e != nil {
				return fail()
			}
			frame, e := sideDiskReadFrame(f, pid)
			if e != nil {
				return fail()
			}
			rad, e := cylinderSupportNumber(rec.Args[2])
			if e != nil || rad <= 0 {
				return fail()
			}
			frames = append(frames, frame)
			radii = append(radii, rad)
		default:
			return fail()
		}
	}
	if len(frames) != 2 || frames[0].XAxis() != frames[1].XAxis() || frames[0].ZAxis() != frames[1].ZAxis() || (radii[0] != radii[1]) != (kind != sideDiskEqual) {
		return fail()
	}
	// Use one cylinder's exact signed frame for projection. Only exact rational
	// differences that remain representable in the native descriptor are admitted.
	base := frames[0]
	lo, hi := geom.Vec3{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}, geom.Vec3{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	var edgeIDs []int64
	vertices := 0
	depth := 0.
	circleCount := 0
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		rec := en.Records[0]
		switch rec.Type {
		case "VERTEX_POINT":
			rec, e := simple(f, id, "VERTEX_POINT", 2)
			if e != nil {
				return fail()
			}
			pid, e := ref(rec.Args[1])
			if e != nil {
				return fail()
			}
			p, e := splitTriple(f, pid, "CARTESIAN_POINT")
			if e != nil {
				return fail()
			}
			p, e = sideDiskLocal(base, p)
			if e != nil {
				return fail()
			}
			lo.X = math.Min(lo.X, p.X)
			lo.Y = math.Min(lo.Y, p.Y)
			lo.Z = math.Min(lo.Z, p.Z)
			hi.X = math.Max(hi.X, p.X)
			hi.Y = math.Max(hi.Y, p.Y)
			hi.Z = math.Max(hi.Z, p.Z)
			vertices++
		case "EDGE_CURVE":
			edgeIDs = append(edgeIDs, id)
		case "CIRCLE":
			rec, e := simple(f, id, "CIRCLE", 3)
			if e != nil {
				return fail()
			}
			pid, e := ref(rec.Args[1])
			if e != nil {
				return fail()
			}
			frame, e := sideDiskReadFrame(f, pid)
			if e != nil {
				return fail()
			}
			p, e := sideDiskLocal(base, frame.Origin())
			if e != nil || p.Z < 0 {
				return fail()
			}
			depth = math.Max(depth, p.Z)
			circleCount++
		}
	}
	if vertices < 8 || vertices > 128 || len(edgeIDs) > 128 || circleCount != 4 || lo.Z != 0 || depth <= 0 {
		return fail()
	}
	origin, err := sideDiskWorld(base, lo)
	if err != nil {
		return fail()
	}
	frame, err := geom.NewFrame(origin, base.XAxis(), base.ZAxis())
	if err != nil {
		return fail()
	}
	corner, err := sideDiskWorld(base, hi)
	if err != nil {
		return fail()
	}
	dims, err := sideDiskLocal(frame, corner)
	if err != nil {
		return fail()
	}
	d := SideOpenDualDiskDescriptor{Name: name, Frame: frame, Max: geom.Vec2{X: dims.X, Y: dims.Y}, Height: dims.Z, Depth: depth, LengthUnit: unit, Tolerance: tolerance}
	for i, fr := range frames {
		p, e := sideDiskLocal(frame, fr.Origin())
		if e != nil || p.Z != 0 {
			return fail()
		}
		d.Discs[i] = trimmed.ExtrusionCircle{Center: geom.Vec2{X: p.X, Y: p.Y}, Radius: radii[i]}
	}
	if d.Discs[0].Center.X > d.Discs[1].Center.X {
		d.Discs[0], d.Discs[1] = d.Discs[1], d.Discs[0]
	}
	var expected bytes.Buffer
	if kind == sideDiskOffset {
		err = WriteOffsetSideOpenDualDiskProduct(ctx, &expected, OffsetSideOpenDualDiskDescriptor(d))
	} else if kind == sideDiskExactWalls {
		err = WriteExactWallSideOpenDualDiskProduct(ctx, &expected, ExactWallSideOpenDualDiskDescriptor(d))
	} else if kind == sideDiskGeneral {
		err = WriteGeneralSideOpenDualDiskProduct(ctx, &expected, GeneralSideOpenDualDiskDescriptor(d))
	} else if kind == sideDiskUnequal {
		err = WriteUnequalSideOpenDualDiskProduct(ctx, &expected, UnequalSideOpenDualDiskDescriptor(d))
	} else {
		err = WriteSideOpenDualDiskProduct(ctx, &expected, d)
	}
	if err != nil {
		return sideDiskReadResult{}, err
	}
	ef, err := Parse(&expected, Limits{})
	if err != nil {
		return fail()
	}
	var reversedLines []int64
	if options.AllowReversedModelLines {
		reversedLines, err = sideDiskNormalizeReversedLines(f)
		if err != nil {
			return sideDiskReadResult{}, err
		}
	}
	var rephasedCircles, oppositeCircles []int64
	if options.AllowRephasedModelCircles || options.AllowOppositeModelCircleNormals {
		rephasedCircles, oppositeCircles, err = sideDiskNormalizeEquivalentCircles(f, d.Frame, options)
		if err != nil {
			return sideDiskReadResult{}, err
		}
	}
	var oppositePlanes []int64
	if options.AllowOppositePlaneNormals {
		oppositePlanes, err = sideDiskNormalizeOppositePlanes(f, ef, faces)
		if err != nil {
			return sideDiskReadResult{}, err
		}
	}
	if err = sideDiskMatchWholeGraph(f, ef); err != nil {
		return sideDiskReadResult{}, err
	}
	var native trimmed.SideOpenDualDiskCut
	var unequalNative trimmed.UnequalSideOpenDualDiskCut
	var generalNative trimmed.GeneralSideOpenDualDiskCut
	var exactWallNative trimmed.GeneralSideOpenDualDiskCut
	var offsetNative trimmed.OffsetSideOpenDualDiskCut
	if kind == sideDiskOffset {
		offsetNative, err = trimmed.CutOffsetSideOpenDualDisks(ctx, d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
	} else if kind == sideDiskExactWalls {
		exactWallNative, err = trimmed.CutGeneralSideOpenDualDisksWithExactWalls(ctx, d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
	} else if kind == sideDiskGeneral {
		generalNative, err = trimmed.CutGeneralSideOpenDualDisks(ctx, d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
	} else if kind == sideDiskUnequal {
		unequalNative, err = trimmed.CutUnequalSideOpenDualDisks(ctx, d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
	} else {
		native, err = trimmed.CutSideOpenDualDisks(ctx, d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth, tolerance, trimvalidate.Options{})
	}
	if err != nil {
		return sideDiskReadResult{}, err
	}
	if err = ctx.Err(); err != nil {
		return sideDiskReadResult{}, err
	}
	return sideDiskReadResult{offsetNative: offsetNative, unequalNative: unequalNative, generalNative: generalNative, exactWallNative: exactWallNative, SideOpenDualDiskProduct: SideOpenDualDiskProduct{Descriptor: d, Native: native, Body: bodies[0], Shell: shellID, Faces: append([]int64(nil), faces...), Edges: edgeIDs, ReversedModelLines: reversedLines, RephasedModelCircles: rephasedCircles, OppositeNormalModelCircles: oppositeCircles, OppositeNormalPlanes: oppositePlanes}}, nil
}
func sideDiskReadFrame(f *File, id int64) (geom.Frame, error) {
	rec, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return geom.Frame{}, e
	}
	var v [3]geom.Vec3
	for i := range v {
		rid, e := ref(rec.Args[i+1])
		if e != nil {
			return geom.Frame{}, e
		}
		kind := "DIRECTION"
		if i == 0 {
			kind = "CARTESIAN_POINT"
		}
		v[i], e = splitTriple(f, rid, kind)
		if e != nil {
			return geom.Frame{}, e
		}
	}
	for _, a := range v[1:] {
		n := 0
		for _, x := range []float64{a.X, a.Y, a.Z} {
			if x != 0 {
				if x != 1 && x != -1 {
					return geom.Frame{}, geom.ErrDomain
				}
				n++
			}
		}
		if n != 1 {
			return geom.Frame{}, geom.ErrDomain
		}
	}
	if v[1].Dot(v[2]) != 0 {
		return geom.Frame{}, geom.ErrDomain
	}
	return geom.NewFrame(v[0], v[2], v[1])
}
func sideDiskMatchWholeGraph(a, b *File) error {
	// Bind both independent AP203 roots in one synthetic root so shared context
	// identities are checked across metadata and geometry. Nothing is ignored.
	root := func(f *File) (*File, int64, error) {
		g := &File{Entities: make(map[int64]Entity, len(f.Entities)+1)}
		var ids [2]int64
		var max int64
		for id, en := range f.Entities {
			g.Entities[id] = en
			if id > max {
				max = id
			}
			for _, r := range en.Records {
				for i, k := range []string{"SHAPE_DEFINITION_REPRESENTATION", "APPLICATION_PROTOCOL_DEFINITION"} {
					if r.Type == k {
						if ids[i] != 0 || len(en.Records) != 1 {
							return nil, 0, geom.ErrDomain
						}
						ids[i] = id
					}
				}
			}
		}
		if ids[0] == 0 || ids[1] == 0 || max == math.MaxInt64 {
			return nil, 0, geom.ErrDomain
		}
		max++
		g.Entities[max] = Entity{ID: max, Records: []Record{{Type: "EXACT_SIDE_DISK_GRAPH", Args: []Value{{Kind: Reference, Ref: ids[0]}, {Kind: Reference, Ref: ids[1]}}}}}
		return g, max, nil
	}
	aa, ar, e := root(a)
	if e != nil {
		return e
	}
	bb, br, e := root(b)
	if e != nil {
		return e
	}
	used, e := equalCircularExtrusionGraph(aa, ar, bb, br)
	if e != nil {
		return e
	}
	if len(used) != len(aa.Entities) || len(aa.Entities) != len(bb.Entities) {
		return geom.ErrDomain
	}
	// Header file names/timestamps/descriptions are documentary, not topology.
	// The schema remains the exact AP203 declaration used by this dialect.
	schema := func(f *File) bool {
		n := 0
		for _, r := range f.Header {
			if r.Type == "FILE_SCHEMA" {
				n++
				if len(r.Args) != 1 || r.Args[0].Kind != Aggregate || len(r.Args[0].Items) != 1 || r.Args[0].Items[0].Kind != String || r.Args[0].Items[0].Text != "CONFIG_CONTROL_DESIGN" {
					return false
				}
			}
		}
		return n == 1
	}
	if !schema(a) || !schema(b) {
		return geom.ErrDomain
	}
	return nil
}
