package step

import (
	"context"
	"errors"
	"plainsolid/geom"
)

// WrappedSplineFace preserves the explicit topology of a full fundamental
// rectangle. SideEdges are bottom, right, top, left; CornerVertices follow
// counterclockwise UV order. Repeated IDs identify actual source seams.
// This certifies boundary agreement, not surface regularity or solid embedding.
type WrappedSplineFace struct {
	Name           string
	Geometry       *geom.WrappedSurfacePatch
	SideEdges      [4]int64
	CornerVertices [4]int64
}

// DecodeWrappedSplineFace accepts a positive ADVANCED_FACE with four oriented
// uses, exact full-domain spline isocurves and unit LINE pcurves. Closure flags
// must agree with explicitly shared seam edges and corner vertex identities.
func DecodeWrappedSplineFace(ctx context.Context, f *File, id int64, options WrappedSurfaceOptions) (WrappedSplineFace, error) {
	fail := func() (WrappedSplineFace, error) {
		return WrappedSplineFace{}, errors.New("step: unsupported wrapped fundamental face topology")
	}
	if ctx == nil || f == nil {
		return fail()
	}
	o, e := options.defaults()
	if e != nil {
		return WrappedSplineFace{}, e
	}
	face, uses, e := analyticFaceLoop(f, id, 4)
	if e != nil {
		return fail()
	}
	sense, e := boolean(face.Args[3])
	if e != nil || !sense {
		return fail()
	}
	sid, e := ref(face.Args[2])
	if e != nil {
		return fail()
	}
	support, e := DecodeWrappedSurface(ctx, f, sid, o)
	if e != nil {
		return WrappedSplineFace{}, e
	}
	out := WrappedSplineFace{Geometry: support.Geometry}
	out.Name, e = textValue(face.Args[0])
	if e != nil {
		return fail()
	}
	patch := out.Geometry.FundamentalPatch()
	d := patch.Domain()
	periods := out.Geometry.Periods()
	wrapU, wrapV := periods[0] != nil, periods[1] != nil
	remaining := o.MaxOperations - patch.ExactOperations() - out.Geometry.SeamCertificate().ExactOperations
	seen := map[int64]bool{}
	pcsSeen := map[int64]bool{}
	endpoints := [4][2]int64{}
	for _, use := range uses {
		if seen[use.id] {
			continue
		}
		seen[use.id] = true
		if e := ctx.Err(); e != nil {
			return WrappedSplineFace{}, e
		}
		a, b, g, e := analyticEdge(f, use.id)
		if e != nil {
			return fail()
		}
		ent := f.Entities[g]
		if len(ent.Records) != 1 {
			return fail()
		}
		kind := ent.Records[0].Type
		count := 1
		if kind == "SEAM_CURVE" {
			count = 2
		} else if kind != "SURFACE_CURVE" {
			return fail()
		}
		sc, e := simple(f, g, kind, 4)
		if e != nil {
			return fail()
		}
		if _, e = textValue(sc.Args[0]); e != nil {
			return fail()
		}
		if sc.Args[3].Kind != Enumeration || sc.Args[3].Text != "PCURVE_S1" {
			return fail()
		}
		cid, e := ref(sc.Args[1])
		if e != nil {
			return fail()
		}
		pcs, e := listRefs(sc.Args[2], 1)
		if e != nil || len(pcs) != count {
			return fail()
		}
		if e = exactSplineWeightNormalization(f, cid, "RATIONAL_B_SPLINE_CURVE"); e != nil {
			return WrappedSplineFace{}, e
		}
		curve, e := DecodeBSplineCurve(f, cid, o.MaxPoles)
		if e != nil {
			return WrappedSplineFace{}, e
		}
		for _, pc := range pcs {
			if pcsSeen[pc] {
				return fail()
			}
			pcsSeen[pc] = true
			axis := 0
			fixed, err := wrappedIsoPCurve(f, pc, sid, axis)
			if err != nil {
				axis = 1
				fixed, err = wrappedIsoPCurve(f, pc, sid, axis)
			}
			if err != nil {
				return fail()
			}
			side := -1
			if axis == 0 {
				if fixed == d.VMin {
					side = 0
				}
				if fixed == d.VMax {
					side = 2
				}
			} else {
				if fixed == d.UMax {
					side = 1
				}
				if fixed == d.UMin {
					side = 3
				}
			}
			if side < 0 || out.SideEdges[side] != 0 {
				return fail()
			}
			if remaining < 1 {
				return WrappedSplineFace{}, geom.ErrConvergence
			}
			po := geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits}
			iso, e := patch.IsoCurve(1-axis, fixed, po)
			if e != nil {
				return WrappedSplineFace{}, e
			}
			remaining -= iso.ExactOperations()
			if remaining < 1 {
				return WrappedSplineFace{}, geom.ErrConvergence
			}
			po.MaxOperations = remaining
			model, e := geom.NewExactSplineCurve(curve.Geometry, po)
			if e != nil {
				return WrappedSplineFace{}, e
			}
			remaining -= model.ExactOperations()
			lo, hi := iso.Domain()
			ml, mh := model.Domain()
			if lo != ml || hi != mh {
				return fail()
			}
			if remaining < 1 {
				return WrappedSplineFace{}, geom.ErrConvergence
			}
			proof, e := geom.CompareExactCurves(model, iso, geom.ExactCurveComparisonOptions{MaxSpans: o.MaxPoles, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return WrappedSplineFace{}, e
			}
			remaining -= proof.ExactOperations
			if !proof.Complete || !proof.Equal {
				return fail()
			}
			for j, v := range []int64{a, b} {
				t := lo
				if j == 1 {
					t = hi
				}
				want, e := iso.ExactPoint(t)
				if e != nil {
					return WrappedSplineFace{}, e
				}
				got, e := analyticVertex(f, v)
				if e != nil || got != want {
					return fail()
				}
			}
			out.SideEdges[side] = use.id
			endpoints[side] = [2]int64{a, b}
		}
	}
	for i := 0; i < 4; i++ {
		if out.SideEdges[i] == 0 {
			return fail()
		}
		pair := endpoints[i]
		if i >= 2 {
			pair = [2]int64{pair[1], pair[0]}
		}
		out.CornerVertices[i] = pair[0]
		next := endpoints[(i+1)%4]
		if (i+1)%4 >= 2 {
			next = [2]int64{next[1], next[0]}
		}
		if pair[1] != next[0] {
			return fail()
		}
	}
	if (out.SideEdges[1] == out.SideEdges[3]) != wrapU || (out.SideEdges[0] == out.SideEdges[2]) != wrapV {
		return fail()
	}
	wantEdges := 4
	if wrapU {
		wantEdges--
	}
	if wrapV {
		wantEdges--
	}
	if len(seen) != wantEdges {
		return fail()
	}
	vertices := map[int64]bool{}
	for _, v := range out.CornerVertices {
		vertices[v] = true
	}
	wantVertices := 4
	if wrapU {
		wantVertices /= 2
	}
	if wrapV {
		wantVertices /= 2
	}
	if len(vertices) != wantVertices {
		return fail()
	}
	start := -1
	for k, u := range uses {
		side := -1
		for i, edge := range out.SideEdges {
			if edge == u.id && u.sense == (i < 2) {
				if side != -1 {
					return fail()
				}
				side = i
			}
		}
		if side < 0 {
			return fail()
		}
		if k == 0 {
			start = side
		} else if side != (start+k)%4 {
			return fail()
		}
	}
	return out, nil
}
