package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// PosedRepairedOCCTConvexArcExtrusion retains explicit world-space repair
// evidence and a product-local exact solid. Descriptor.Frame maps that solid
// to original model coordinates; no rounded geometry baking is performed.
type PosedRepairedOCCTConvexArcExtrusion struct {
	base      RepairedOCCTConvexArcExtrusion
	rotation  geom.Affine
	reference int64
}

func (m PosedRepairedOCCTConvexArcExtrusion) OriginalSTEP() string  { return m.base.OriginalSTEP() }
func (m PosedRepairedOCCTConvexArcExtrusion) CorrectedSTEP() string { return m.base.CorrectedSTEP() }
func (m PosedRepairedOCCTConvexArcExtrusion) Solid() trimmed.Solid  { return m.base.Solid() }
func (m PosedRepairedOCCTConvexArcExtrusion) Descriptor() ExactCircularLoopExtrusion {
	return m.base.Descriptor()
}
func (m PosedRepairedOCCTConvexArcExtrusion) Repairs() []ConvexArcRepairSite { return m.base.Repairs() }
func (m PosedRepairedOCCTConvexArcExtrusion) AzimuthDirections() []int64 {
	return m.base.AzimuthDirections()
}
func (m PosedRepairedOCCTConvexArcExtrusion) Provenance() ConvexArcRepairProvenance {
	return m.base.Provenance()
}
func (m PosedRepairedOCCTConvexArcExtrusion) NormalizationRotation() geom.Affine { return m.rotation }
func (m PosedRepairedOCCTConvexArcExtrusion) ReferenceCircleID() int64           { return m.reference }

func convexArcRotateTriple(m geom.Affine, p [3]*big.Rat, inverse bool) (out [3]*big.Rat) {
	for i := range out {
		out[i] = new(big.Rat)
		for j := range p {
			k := m[i][j]
			if inverse {
				k = m[j][i]
			}
			out[i].Add(out[i], new(big.Rat).Mul(sideRat(k), p[j]))
		}
	}
	return
}

// Transform every 3D point and direction in the authenticated body dependency
// candidate, leaving neutral representation metadata unchanged. Unsupported
// geometry remains present and must be rejected by the complete strict reader.
func convexArcRotateView(ctx context.Context, f *File, body, metadata map[int64]bool, m geom.Affine, inverse bool) (*File, error) {
	return convexArcTransformView(ctx, f, body, metadata, func(_ string, p [3]*big.Rat) ([3]*big.Rat, error) {
		return convexArcRotateTriple(m, p, inverse), nil
	})
}

func convexArcTransformView(ctx context.Context, f *File, body, metadata map[int64]bool, transform func(string, [3]*big.Rat) ([3]*big.Rat, error)) (*File, error) {
	view := &File{Header: f.Header, Order: append([]int64(nil), f.Order...), Entities: make(map[int64]Entity, len(f.Entities))}
	for id, en := range f.Entities {
		view.Entities[id] = en
	}
	for _, id := range f.Order {
		if e := ctx.Err(); e != nil {
			return nil, e
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
		p, e := occtLoopTriple(f, id, kind)
		if e != nil {
			return nil, e
		}
		q, e := transform(kind, p)
		if e != nil {
			return nil, e
		}
		values := make([]Value, 3)
		for i := range q {
			if metadata[id] && q[i].Cmp(p[i]) != 0 {
				return nil, geom.ErrDomain
			}
			text, e := extrusionDirectionDecimal(q[i])
			if e != nil {
				return nil, e
			}
			values[i] = Value{Kind: Number, Text: text}
		}
		r := en.Records[0]
		r.Args = append([]Value(nil), r.Args...)
		r.Args[1] = Value{Kind: Aggregate, Items: values}
		view.Entities[id] = Entity{ID: id, Records: []Record{r}}
	}
	return view, nil
}

// RepairPosedOCCTConvexArcExtrusion adds a proper signed-coordinate rigid
// normalization to the original explicit tangent repair. Original decimal
// positions are rotated exactly, all corrections are transported back exactly,
// and the corrected world graph is independently re-proved. Translation is
// retained in original profile coordinates and the descriptor's axial origin.
// The unposed repair entry point is unchanged.
func RepairPosedOCCTConvexArcExtrusion(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (PosedRepairedOCCTConvexArcExtrusion, error) {
	return repairPosedOCCTConvexArcExtrusion(ctx, r, l, o, false)
}
func repairPosedOCCTConvexArcExtrusion(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions, grouped bool) (PosedRepairedOCCTConvexArcExtrusion, error) {
	var zero PosedRepairedOCCTConvexArcExtrusion
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 1024)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	f, e := Parse(bytes.NewReader(raw), l)
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
	var matrix geom.Affine
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
		if grouped {
			found := false
			for _, sid := range f.Order {
				if !body[sid] {
					continue
				}
				cyl, err := simple(f, sid, "CYLINDRICAL_SURFACE", 3)
				if err != nil {
					continue
				}
				_, z, _, err = occtLoopFrame(f, cyl.Args[1].Ref)
				if err != nil {
					return zero, err
				}
				found = true
				break
			}
			if !found {
				return zero, geom.ErrDomain
			}
		}
		za, err := posedLoopCardinal(z)
		if err != nil {
			return zero, err
		}
		xa, err := posedLoopCardinal(x)
		if err != nil {
			return zero, err
		}
		frame, err := geom.NewFrame(geom.Vec3{}, xa, za)
		if err != nil || !planarSplineRigid(frameTransform(frame)) {
			return zero, geom.ErrDomain
		}
		matrix = frameTransform(frame)
		reference = id
		break
	}
	if reference == 0 {
		return zero, geom.ErrDomain
	}
	view, e := convexArcRotateView(ctx, f, body, metadata, matrix, true)
	if e != nil {
		return zero, e
	}
	local, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	repair, e := repairOCCTConvexArcExtrusion(ctx, bytes.NewReader(local), l, o, grouped)
	if e != nil {
		return zero, e
	}
	corrected, e := Parse(bytes.NewBufferString(repair.CorrectedSTEP()), l)
	if e != nil {
		return zero, e
	}
	world, e := convexArcRotateView(ctx, corrected, body, metadata, matrix, false)
	if e != nil {
		return zero, e
	}
	worldRaw, e := mappedPlanarSourceBytes(world)
	if e != nil {
		return zero, e
	}
	proof, e := ReadPosedOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(worldRaw), l)
	if e != nil {
		return zero, e
	}
	// Every reported source site must return to its original exact world value;
	// rigid rotation preserves the squared displacement, checked independently.
	for i := range repair.sites {
		site := &repair.sites[i]
		site.Original = convexArcRotateTriple(matrix, site.Original, false)
		site.Repaired = convexArcRotateTriple(matrix, site.Repaired, false)
		original, e := occtLoopTriple(f, site.PointID, "CARTESIAN_POINT")
		if e != nil {
			return zero, e
		}
		for j := range original {
			if original[j].Cmp(site.Original[j]) != 0 {
				return zero, geom.ErrDomain
			}
		}
		d := occtLoopSub(site.Repaired, site.Original)
		if occtLoopDot(d, d).Cmp(site.SquaredDistance) != 0 {
			return zero, geom.ErrDomain
		}
	}
	for i := range repair.tangencies {
		repair.tangencies[i].Point = convexArcRotateTriple(matrix, repair.tangencies[i].Point, false)
	}
	repair.original = string(raw)
	repair.corrected = string(worldRaw)
	repair.material = proof.OCCTExactCircularLoopExtrusion
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return PosedRepairedOCCTConvexArcExtrusion{repair, matrix, reference}, nil
}
