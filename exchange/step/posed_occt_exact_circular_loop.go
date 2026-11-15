package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// PosedOCCTExactCircularLoopExtrusion retains the original source and a proved
// proper signed-axis rotation. Native remains local; Descriptor.Frame maps it
// into original model coordinates. Height is normalized positive, not recovered
// historical extrusion intent. ReferenceCircleID identifies the CIRCLE whose frame defines the rotation.
type PosedOCCTExactCircularLoopExtrusion struct {
	OCCTExactCircularLoopExtrusion
	// NormalizationRotation maps normalized axes to original world axes (no translation).
	NormalizationRotation geom.Affine
	ReferenceCircleID     int64
}

// ReadPosedOCCTExactCircularLoopExtrusion is a separate signed-coordinate pose
// mode. It rotates every coordinate-bearing body dependency exactly into a
// temporary proof view, requires unchanged neutral representation metadata,
// invokes the complete exact local vendor proof, and retains original source.
// It does not normalize arbitrary floating rotations, scale, or pcurve charts.
func ReadPosedOCCTExactCircularLoopExtrusion(ctx context.Context, r io.Reader, l Limits) (PosedOCCTExactCircularLoopExtrusion, error) {
	var zero PosedOCCTExactCircularLoopExtrusion
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
	f, e := Parse(r, l)
	if e != nil {
		return zero, e
	}
	var repID int64
	for _, id := range f.Order {
		if _, err := simple(f, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3); err == nil {
			if repID != 0 {
				return zero, geom.ErrDomain
			}
			repID = id
		}
	}
	rep, e := simple(f, repID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return zero, e
	}
	items, e := listRefs(rep.Args[1], 2)
	if e != nil || len(items) != 2 {
		return zero, geom.ErrDomain
	}
	neutral, e := sideDiskReadFrame(f, items[0])
	if e != nil || frameTransform(neutral) != geom.Identity() {
		return zero, geom.ErrDomain
	}
	if _, e = simple(f, items[1], "MANIFOLD_SOLID_BREP", 2); e != nil {
		return zero, e
	}
	body, e := nauoClosure(f, []int64{items[1]}, 16)
	if e != nil {
		return zero, e
	}
	metadata, e := nauoClosure(f, []int64{items[0]}, 16)
	if e != nil {
		return zero, e
	}
	var reference int64
	var rotation geom.Frame
	for _, id := range f.Order {
		if !body[id] {
			continue
		}
		c, err := simple(f, id, "CIRCLE", 3)
		if err != nil {
			continue
		}
		_, z, x, err := occtLoopFrame(f, c.Args[1].Ref)
		if err != nil {
			return zero, err
		}
		zAxis, err := posedLoopCardinal(z)
		if err != nil {
			return zero, err
		}
		xAxis, err := posedLoopCardinal(x)
		if err != nil {
			return zero, err
		}
		rotation, err = geom.NewFrame(geom.Vec3{}, xAxis, zAxis)
		if err != nil || !planarSplineRigid(frameTransform(rotation)) {
			return zero, geom.ErrDomain
		}
		reference = id
		break
	}
	if reference == 0 {
		return zero, geom.ErrDomain
	}
	matrix := frameTransform(rotation)
	view := &File{Header: f.Header, Order: append([]int64(nil), f.Order...), Entities: make(map[int64]Entity, len(f.Entities))}
	for id, en := range f.Entities {
		view.Entities[id] = en
	}
	for _, id := range f.Order {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		if !body[id] {
			continue
		}
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		kind := en.Records[0].Type
		if kind != "CARTESIAN_POINT" && kind != "DIRECTION" {
			continue
		}
		v, e := occtLoopTriple(f, id, kind)
		if e != nil {
			return zero, e
		}
		var values []Value
		for i := 0; i < 3; i++ {
			q := new(big.Rat)
			for j := 0; j < 3; j++ {
				q.Add(q, new(big.Rat).Mul(sideRat(matrix[j][i]), v[j]))
			}
			if metadata[id] && q.Cmp(v[i]) != 0 {
				return zero, geom.ErrDomain
			}
			text, e := extrusionDirectionDecimal(q)
			if e != nil {
				return zero, e
			}
			values = append(values, Value{Kind: Number, Text: text})
		}
		rec := en.Records[0]
		rec.Args = append([]Value(nil), rec.Args...)
		rec.Args[1] = Value{Kind: Aggregate, Items: values}
		view.Entities[id] = Entity{ID: id, Records: []Record{rec}}
	}
	raw, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	if len(raw) > 1<<20 {
		return zero, geom.ErrConvergence
	}
	proof, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	origin, e := sideDiskWorld(rotation, proof.Descriptor.Frame.Origin())
	if e != nil {
		return zero, e
	}
	posed, e := geom.NewFrame(origin, rotation.XAxis(), rotation.ZAxis())
	if e != nil {
		return zero, e
	}
	proof.Descriptor.Frame = posed
	proof.Source = f
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return PosedOCCTExactCircularLoopExtrusion{proof, matrix, reference}, nil
}

// WritePosedOCCTExactCircularLoopSource preserves only a newly reproved original
// posed graph, not the normalized temporary geometry used for authentication.
func WritePosedOCCTExactCircularLoopSource(ctx context.Context, w io.Writer, f *File, l Limits) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 16) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if len(raw) > 1<<20 {
		return geom.ErrConvergence
	}
	if _, e = ReadPosedOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

func posedLoopCardinal(v [3]*big.Rat) (geom.Vec3, error) {
	var a [3]float64
	n := 0
	for i, q := range v {
		if q == nil {
			return geom.Vec3{}, geom.ErrDomain
		}
		if q.Sign() != 0 {
			n++
			a[i] = float64(q.Sign())
		}
	}
	if n != 1 {
		return geom.Vec3{}, geom.ErrDomain
	}
	return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, nil
}
