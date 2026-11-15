package step

import (
	"bytes"
	"context"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ConvexArcRepairOptions authorizes only independently proved tangent point
// and parallel axial-line translation, in source length units. It does not
// authorize support fitting or arbitrary trim repair.
type ConvexArcRepairOptions struct{ MaxDistance float64 }

// ConvexArcRepairSite identifies an original point or line-origin record.
// The squared displacement is an exact rational, not an equation residual.
type ConvexArcRepairSite struct {
	PointID            int64
	Role               string
	Original, Repaired [3]*big.Rat
	SquaredDistance    *big.Rat
}

// RepairedOCCTConvexArcExtrusion owns newly reconstructed exact material, not
// source-identical geometry. Original malformed sites remain in OriginalSTEP.
type RepairedOCCTConvexArcExtrusion struct {
	original, corrected string
	material            OCCTExactCircularLoopExtrusion
	sites               []ConvexArcRepairSite
	azimuths            []int64
}

// ConvexArcRepairProvenance maps corrected native roles to the unchanged
// original entity IDs. Repairs identify the original records that moved.
type ConvexArcRepairProvenance struct {
	ProductID, DefinitionID, Body, Shell int64
	Caps                                 [2]int64
	Walls, VerticalEdges                 []int64
	Rims, Vertices                       [2][]int64
}

func (m RepairedOCCTConvexArcExtrusion) Provenance() ConvexArcRepairProvenance {
	q := m.material
	p := ConvexArcRepairProvenance{ProductID: q.Product.ProductID, DefinitionID: q.Product.DefinitionID, Body: q.Body, Shell: q.Shell, Caps: q.Caps, Walls: append([]int64(nil), q.Walls...), VerticalEdges: append([]int64(nil), q.VerticalEdges...)}
	for j := 0; j < 2; j++ {
		p.Rims[j] = append([]int64(nil), q.Rims[j]...)
		p.Vertices[j] = append([]int64(nil), q.Vertices[j]...)
	}
	return p
}

func (m RepairedOCCTConvexArcExtrusion) OriginalSTEP() string  { return m.original }
func (m RepairedOCCTConvexArcExtrusion) CorrectedSTEP() string { return m.corrected }
func (m RepairedOCCTConvexArcExtrusion) Solid() trimmed.Solid  { return m.material.Native }
func (m RepairedOCCTConvexArcExtrusion) Descriptor() ExactCircularLoopExtrusion {
	d := m.material.Descriptor
	d.Segments = append([]trimmed.ExactCircularLoopSegment(nil), d.Segments...)
	return d
}
func (m RepairedOCCTConvexArcExtrusion) AzimuthDirections() []int64 {
	return append([]int64(nil), m.azimuths...)
}
func (m RepairedOCCTConvexArcExtrusion) Repairs() []ConvexArcRepairSite {
	out := append([]ConvexArcRepairSite(nil), m.sites...)
	for i := range out {
		for j := 0; j < 3; j++ {
			out[i].Original[j] = new(big.Rat).Set(out[i].Original[j])
			out[i].Repaired[j] = new(big.Rat).Set(out[i].Repaired[j])
		}
		out[i].SquaredDistance = new(big.Rat).Set(out[i].SquaredDistance)
	}
	return out
}

// RepairOCCTConvexArcExtrusion supports the six-face model-only single-fillet
// prism: five planes, one +Z cylinder, four profile segments. Each arc endpoint
// must be the unique exact cylinder/side-plane tangency at its cap station.
// Positive cardinal circle azimuth changes are support-identical; there are
// no source pcurves. The corrected whole source passes the existing exact
// circular-loop importer, including original incidence and directed trims.
func RepairOCCTConvexArcExtrusion(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (RepairedOCCTConvexArcExtrusion, error) {
	var zero RepairedOCCTConvexArcExtrusion
	if ctx == nil || r == nil || math.IsNaN(o.MaxDistance) || math.IsInf(o.MaxDistance, 0) || o.MaxDistance <= 0 {
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
	faces := map[int64]occtLoopFace{}
	edges := map[int64]Record{}
	vertices := map[int64]int64{}
	incident := map[int64]map[int64]bool{}
	var cylinder occtLoopFace
	for _, id := range f.Order {
		if _, err := simple(f, id, "ADVANCED_FACE", 4); err != nil {
			continue
		}
		face, err := occtLoopReadFace(f, id)
		if err != nil {
			return zero, err
		}
		faces[id] = face
		if face.kind == "CYLINDRICAL_SURFACE" {
			if cylinder.id != 0 {
				return zero, geom.ErrDomain
			}
			cylinder = face
		}
		for _, u := range face.uses {
			ed, err := simple(f, u.id, "EDGE_CURVE", 5)
			if err != nil {
				return zero, err
			}
			edges[u.id] = ed
			for _, vid := range []int64{ed.Args[1].Ref, ed.Args[2].Ref} {
				v, err := simple(f, vid, "VERTEX_POINT", 2)
				if err != nil {
					return zero, err
				}
				vertices[vid] = v.Args[1].Ref
				if incident[vid] == nil {
					incident[vid] = map[int64]bool{}
				}
				incident[vid][id] = true
			}
		}
	}
	if len(faces) != 6 || len(edges) != 12 || len(vertices) != 8 || cylinder.id == 0 || !cylinder.sense || !occtLoopPositiveAxis(cylinder.z, 2) {
		return zero, geom.ErrDomain
	}
	tolerance := new(big.Rat).SetFloat64(o.MaxDistance)
	if tolerance.Cmp(new(big.Rat).Quo(cylinder.radius, big.NewRat(1000000, 1))) > 0 {
		return zero, geom.ErrDomain
	}
	tol2 := new(big.Rat).Mul(tolerance, tolerance)
	original := map[int64][3]*big.Rat{}
	target := map[int64][3]*big.Rat{}
	roles := map[int64]string{}
	for _, pid := range vertices {
		v, err := occtLoopTriple(f, pid, "CARTESIAN_POINT")
		if err != nil {
			return zero, err
		}
		original[pid] = v
	}
	for vid, pid := range vertices {
		if !incident[vid][cylinder.id] {
			continue
		}
		var cap, side occtLoopFace
		for fid := range incident[vid] {
			face := faces[fid]
			if face.kind != "PLANE" {
				continue
			}
			if occtLoopParallel(face.z, occtLoopVec(geom.Vec3{Z: 1})) {
				if cap.id != 0 {
					return zero, geom.ErrDomain
				}
				cap = face
			} else if face.z[2].Sign() == 0 {
				if side.id != 0 {
					return zero, geom.ErrDomain
				}
				side = face
			} else {
				return zero, geom.ErrDomain
			}
		}
		if cap.id == 0 || side.id == 0 || len(incident[vid]) != 3 {
			return zero, geom.ErrDomain
		}
		n2 := occtLoopDot(side.z, side.z)
		dot := occtLoopDot(occtLoopSub(cylinder.origin, side.origin), side.z)
		if new(big.Rat).Mul(dot, dot).Cmp(new(big.Rat).Mul(new(big.Rat).Mul(cylinder.radius, cylinder.radius), n2)) != 0 {
			return zero, geom.ErrDomain
		}
		k := new(big.Rat).Quo(dot, n2)
		var p [3]*big.Rat
		for j := range p {
			p[j] = new(big.Rat).Sub(cylinder.origin[j], new(big.Rat).Mul(k, side.z[j]))
		}
		p[2] = new(big.Rat).Set(cap.origin[2])
		d := occtLoopSub(p, original[pid])
		d2 := occtLoopDot(d, d)
		if d2.Cmp(tol2) > 0 {
			return zero, geom.ErrDomain
		}
		if d2.Sign() != 0 {
			target[pid] = p
			roles[pid] = "tangent vertex"
		}
	}
	// Only axial model lines can translate. Both original endpoint corrections
	// must be identical; their complete infinite support then moves by that vector.
	for _, ed := range edges {
		c, err := simple(f, ed.Args[3].Ref, "LINE", 3)
		if err != nil {
			continue
		}
		v, err := simple(f, c.Args[2].Ref, "VECTOR", 3)
		if err != nil {
			return zero, err
		}
		d, err := occtLoopTriple(f, v.Args[1].Ref, "DIRECTION")
		if err != nil {
			return zero, err
		}
		if !occtLoopParallel(d, occtLoopVec(geom.Vec3{Z: 1})) {
			continue
		}
		pa, pb := vertices[ed.Args[1].Ref], vertices[ed.Args[2].Ref]
		a, b := original[pa], original[pb]
		ta, ok := target[pa]
		if !ok {
			ta = a
		}
		tb, ok := target[pb]
		if !ok {
			tb = b
		}
		da, db := occtLoopSub(ta, a), occtLoopSub(tb, b)
		for j := range da {
			if da[j].Cmp(db[j]) != 0 {
				return zero, geom.ErrDomain
			}
		}
		if occtLoopDot(da, da).Sign() == 0 {
			continue
		}
		pid := c.Args[1].Ref
		orig, err := occtLoopTriple(f, pid, "CARTESIAN_POINT")
		if err != nil {
			return zero, err
		}
		offset := occtLoopSub(a, orig)
		if occtLoopDot(offset, offset).Sign() != 0 && !occtLoopParallel(offset, d) {
			return zero, geom.ErrDomain
		}
		var p [3]*big.Rat
		for j := range p {
			p[j] = new(big.Rat).Add(orig[j], da[j])
		}
		if old, ok := target[pid]; ok {
			for j := range old {
				if old[j].Cmp(p[j]) != 0 {
					return zero, geom.ErrDomain
				}
			}
		} else {
			target[pid] = p
			original[pid] = orig
			roles[pid] = "axial line origin"
		}
	}
	if len(target) > 6 {
		return zero, geom.ErrDomain
	}
	// Reverse references forbid using moved points for supports or unrelated
	// curves, and forbid azimuth normalization outside a selected carrier frame.
	refs := map[int64][]int64{}
	var collect func(int64, Value)
	collect = func(owner int64, v Value) {
		if v.Kind == Reference {
			refs[v.Ref] = append(refs[v.Ref], owner)
		}
		for _, q := range v.Items {
			collect(owner, q)
		}
	}
	for id, en := range f.Entities {
		for _, rec := range en.Records {
			for _, v := range rec.Args {
				collect(id, v)
			}
		}
	}
	out := RepairedOCCTConvexArcExtrusion{original: string(raw)}
	for _, pid := range f.Order {
		p, ok := target[pid]
		if !ok {
			continue
		}
		for _, owner := range refs[pid] {
			rec := f.Entities[owner].Records
			if len(rec) != 1 {
				return zero, geom.ErrDomain
			}
			if rec[0].Type == "VERTEX_POINT" {
				if vertices[owner] != pid {
					return zero, geom.ErrDomain
				}
			} else if rec[0].Type == "LINE" {
				valid := false
				for _, ed := range edges {
					if ed.Args[3].Ref == owner {
						v, err := simple(f, rec[0].Args[2].Ref, "VECTOR", 3)
						if err != nil {
							return zero, err
						}
						d, err := occtLoopTriple(f, v.Args[1].Ref, "DIRECTION")
						if err == nil && occtLoopParallel(d, occtLoopVec(geom.Vec3{Z: 1})) {
							valid = true
						}
					}
				}
				if !valid {
					return zero, geom.ErrDomain
				}
			} else {
				return zero, geom.ErrDomain
			}
		}
		d := occtLoopSub(p, original[pid])
		out.sites = append(out.sites, ConvexArcRepairSite{PointID: pid, Role: roles[pid], Original: original[pid], Repaired: p, SquaredDistance: occtLoopDot(d, d)})
		rec := f.Entities[pid].Records[0]
		for j := range p {
			text, err := mappedTerminatingDecimal(p[j])
			if err != nil {
				return zero, err
			}
			rec.Args[1].Items[j] = Value{Kind: Number, Text: text}
		}
		f.Entities[pid] = Entity{ID: pid, Records: []Record{rec}}
	}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		rec := en.Records[0]
		if rec.Type != "CIRCLE" && rec.Type != "CYLINDRICAL_SURFACE" {
			continue
		}
		if len(rec.Args) != 3 {
			return zero, geom.ErrDomain
		}
		_, z, x, err := occtLoopFrame(f, rec.Args[1].Ref)
		if err != nil || !occtLoopPositiveAxis(z, 2) {
			return zero, geom.ErrDomain
		}
		if x[2].Sign() != 0 || ((x[0].Sign() == 0) == (x[1].Sign() == 0)) {
			return zero, geom.ErrDomain
		}
		frame, _ := simple(f, rec.Args[1].Ref, "AXIS2_PLACEMENT_3D", 4)
		did := frame.Args[3].Ref
		for _, owner := range refs[did] {
			if owner != rec.Args[1].Ref {
				return zero, geom.ErrDomain
			}
		}
		for _, owner := range refs[rec.Args[1].Ref] {
			if owner != id {
				return zero, geom.ErrDomain
			}
		}
		if !occtLoopPositiveAxis(x, 0) {
			direction, _ := simple(f, did, "DIRECTION", 2)
			direction.Args[1].Items = []Value{{Kind: Number, Text: "1."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}}
			f.Entities[did] = Entity{ID: did, Records: []Record{direction}}
			out.azimuths = append(out.azimuths, did)
		}
	}
	corrected, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return zero, e
	}
	material, e := ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(corrected), l)
	if e != nil {
		return zero, e
	}
	if len(material.Descriptor.Segments) != 4 {
		return zero, geom.ErrDomain
	}
	count := 0
	for _, s := range material.Descriptor.Segments {
		if s.Radius > 0 {
			count++
		}
	}
	if count != 1 {
		return zero, geom.ErrDomain
	}
	out.corrected = string(corrected)
	out.material = material
	return out, nil
}
