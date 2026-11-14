package step

import (
	"bytes"
	"context"
	"io"
	"math"
	"math/big"
	"plainsolid/geom"
)

// RationalHierarchyInstance owns an exact composed pose and original NAUO path.
// Matrix and ApplyExact return independent rational copies.
type RationalHierarchyInstance struct {
	ProductID int64
	Path      []int64
	pose      mappedRationalBasis
}

func (u RationalHierarchyInstance) Matrix() [3][4]*big.Rat {
	return (ExactMappedOccurrence{pose: u.pose}).Matrix()
}
func (u RationalHierarchyInstance) ApplyExact(p geom.Vec3) ([3]*big.Rat, error) {
	return (ExactMappedOccurrence{pose: u.pose}).ApplyExact(p)
}
func (u RationalHierarchyInstance) ApplyRepresented(p geom.Vec3) (geom.Vec3, error) {
	return (ExactMappedOccurrence{pose: u.pose}).ApplyRepresented(p)
}

// RationalSphericalHierarchy keeps native local bodies separate from immutable
// rational poses. No approximate geom.Affine is supplied for these placements.
type RationalSphericalHierarchy struct {
	Source        *File
	LengthUnit    float64
	Definitions   []AssemblyProduct
	Roots         []int64
	Products      map[int64]IndependentExactMaterial
	Relationships []ExactNAUOOccurrence
	Instances     []RationalHierarchyInstance
	proof         IndependentSphericalHierarchy
}

func rationalHierarchyFrameBound(b mappedRationalBasis) error {
	for _, v := range [][3]*big.Rat{b.X, b.Y, b.Z, b.Origin} {
		for _, q := range v {
			if q == nil || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
				return geom.ErrInvalid
			}
			if _, e := extrusionDirectionDecimal(q); e != nil {
				return e
			}
		}
	}
	return nil
}

// ReadRationalSphericalHierarchy admits proper rational orthonormal target
// frames with terminating decimal coefficients, neutral child frames and common
// metre/mm units. Exact composition is bounded at every prefix. Local geometry
// is authenticated by a disjoint, private neutral-placement proof view.
func ReadRationalSphericalHierarchy(ctx context.Context, r io.Reader, l Limits, tol float64) (RationalSphericalHierarchy, error) {
	return readRationalSphericalHierarchy(ctx, r, l, tol, false)
}
func readRationalSphericalHierarchy(ctx context.Context, r io.Reader, l Limits, tol float64, occt bool) (RationalSphericalHierarchy, error) {
	bad := func() (RationalSphericalHierarchy, error) { return RationalSphericalHierarchy{}, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return RationalSphericalHierarchy{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 4096)
	l.MaxValues = min(l.MaxValues, 65536)
	l.MaxDepth = min(l.MaxDepth, 16)
	source, e := Parse(r, l)
	if e != nil {
		return RationalSphericalHierarchy{}, e
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return bad()
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return bad()
	}
	material := map[int64]bool{}
	for id, en := range source.Entities {
		if e = ctx.Err(); e != nil {
			return RationalSphericalHierarchy{}, e
		}
		if len(en.Records) == 1 && en.Records[0].Type == "MANIFOLD_SOLID_BREP" {
			c, e := nauoClosure(source, []int64{id}, l.MaxDepth)
			if e != nil {
				return bad()
			}
			for k := range c {
				material[k] = true
			}
		}
	}
	frames := map[int64]mappedRationalBasis{}
	removed := map[int64]bool{}
	for _, id := range source.Order {
		if e = ctx.Err(); e != nil {
			return RationalSphericalHierarchy{}, e
		}
		en := source.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		rec := en.Records[0]
		if rec.Type != "SHAPE_REPRESENTATION" && rec.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		if len(rec.Args) != 3 {
			return bad()
		}
		items, e := listRefs(rec.Args[1], 1)
		if e != nil {
			return bad()
		}
		for _, fr := range items {
			en := source.Entities[fr]
			if len(en.Records) != 1 || en.Records[0].Type != "AXIS2_PLACEMENT_3D" {
				continue
			}
			b, e := mappedRationalFrame(source, fr)
			if e != nil || rationalHierarchyFrameBound(b) != nil {
				return bad()
			}
			frames[fr] = b
			c, e := nauoClosure(source, []int64{fr}, l.MaxDepth)
			if e != nil {
				return bad()
			}
			for key := range c {
				if material[key] {
					return bad()
				}
				if key == fr {
					continue
				}
				dep := source.Entities[key]
				if len(dep.Records) != 1 || len(dep.Records[0].Args) != 2 || (dep.Records[0].Type != "CARTESIAN_POINT" && dep.Records[0].Type != "DIRECTION") {
					return bad()
				}
				if _, e = textValue(dep.Records[0].Args[0]); e != nil {
					return bad()
				}
				removed[key] = true
			}
		}
	}
	if len(frames) == 0 || len(frames) > 256 {
		return bad()
	}
	next := int64(0)
	for id := range source.Entities {
		if id > next {
			next = id
		}
	}
	if next > math.MaxInt64-3 {
		return bad()
	}
	// Only original frame dependencies disjoint from every body are removed from
	// this private view. Original source remains byte-value authoritative.
	for id := range removed {
		delete(view.Entities, id)
	}
	order := []int64{}
	for _, id := range view.Order {
		if !removed[id] {
			order = append(order, id)
		}
	}
	view.Order = order
	add := func(kind string, v [3]string) int64 {
		next++
		view.Order = append(view.Order, next)
		items := []Value{}
		for _, s := range v {
			items = append(items, Value{Kind: Number, Text: s})
		}
		view.Entities[next] = Entity{ID: next, Records: []Record{{Type: kind, Args: []Value{{Kind: String}, {Kind: Aggregate, Items: items}}}}}
		return next
	}
	point := add("CARTESIAN_POINT", [3]string{"0.", "0.", "0."})
	z := add("DIRECTION", [3]string{"0.", "0.", "1."})
	x := add("DIRECTION", [3]string{"1.", "0.", "0."})
	for id := range frames {
		en := view.Entities[id]
		en.Records[0].Args[1] = Value{Kind: Reference, Ref: point}
		en.Records[0].Args[2] = Value{Kind: Reference, Ref: z}
		en.Records[0].Args[3] = Value{Kind: Reference, Ref: x}
		view.Entities[id] = en
	}
	b, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return bad()
	}
	proof, e := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(b), l, tol)
	if e != nil {
		return RationalSphericalHierarchy{}, e
	}
	out := RationalSphericalHierarchy{Source: source, LengthUnit: proof.Assembly.LengthUnit, Definitions: append([]AssemblyProduct(nil), proof.Assembly.Products...), Roots: append([]int64(nil), proof.Assembly.Roots...), Products: map[int64]IndependentExactMaterial{}, proof: proof}
	for id, p := range proof.Products {
		c, e := nauoClosure(source, []int64{p.SourceRepresentationID}, l.MaxDepth)
		if e != nil {
			return bad()
		}
		f := &File{Header: source.Header, Entities: map[int64]Entity{}}
		for _, key := range source.Order {
			if c[key] {
				f.Order = append(f.Order, key)
				f.Entities[key] = source.Entities[key]
			}
		}
		p.Source = f
		out.Products[id] = p
	}
	uses := map[int64]AssemblyOccurrence{}
	for _, u := range proof.Assembly.Occurrences {
		uses[u.ID] = u
	}
	poses := map[int64]mappedRationalBasis{}
	seenRel, seenTr := map[int64]bool{}, map[int64]bool{}
	for _, id := range source.Order {
		if e = ctx.Err(); e != nil {
			return RationalSphericalHierarchy{}, e
		}
		en := source.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION" {
			continue
		}
		rec := en.Records[0]
		pds, e := simple(source, rec.Args[1].Ref, "PRODUCT_DEFINITION_SHAPE", 3)
		if e != nil {
			return bad()
		}
		u, ok := uses[pds.Args[2].Ref]
		if !ok {
			return bad()
		}
		relID := rec.Args[0].Ref
		var tid int64
		for _, v := range source.Entities[relID].Records {
			if v.Type == "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION" {
				if len(v.Args) != 1 {
					return bad()
				}
				tid = v.Args[0].Ref
			}
		}
		if seenRel[relID] || seenTr[tid] {
			return bad()
		}
		seenRel[relID] = true
		seenTr[tid] = true
		tr, e := simple(source, tid, "ITEM_DEFINED_TRANSFORMATION", 4)
		if e != nil {
			return bad()
		}
		from, ok := frames[tr.Args[2].Ref]
		if !ok {
			return bad()
		}
		to, ok := frames[tr.Args[3].Ref]
		if !ok {
			return bad()
		}
		if occt {
			to = composeMappedPose(inverseMappedPose(from), to)
			if e = rationalHierarchyFrameBound(to); e != nil {
				return RationalSphericalHierarchy{}, e
			}
		} else {
			neutral, err := independentTorusAssemblyPose(from)
			if err != nil || neutral != (geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}) {
				return bad()
			}
		}
		if _, exists := poses[u.ID]; exists {
			return bad()
		}
		poses[u.ID] = to
		out.Relationships = append(out.Relationships, ExactNAUOOccurrence{Name: u.Name, NAUOID: u.ID, CDSRID: id, RelationshipID: relID, TransformationID: tid, FromPlacementID: tr.Args[2].Ref, ToPlacementID: tr.Args[3].Ref, ParentID: u.Parent, ChildID: u.Child, pose: to})
	}
	if len(poses) != len(uses) {
		return bad()
	}
	for _, u := range proof.Instances {
		var pose mappedRationalBasis
		for i, id := range u.Path {
			p, ok := poses[id]
			if !ok {
				return bad()
			}
			if i == 0 {
				pose = p
			} else {
				pose = composeMappedPose(pose, p)
			}
			if e = rationalHierarchyFrameBound(pose); e != nil {
				return RationalSphericalHierarchy{}, e
			}
		}
		out.Instances = append(out.Instances, RationalHierarchyInstance{ProductID: u.ProductID, Path: append([]int64(nil), u.Path...), pose: pose})
	}
	return out, nil
}

// WriteRationalSphericalHierarchySource re-proves original rational source and
// emits its actual placements; the private neutral graph is never exported.
func WriteRationalSphericalHierarchySource(ctx context.Context, w io.Writer, source *File, l Limits, tol float64) error {
	return writeRationalSphericalHierarchySource(ctx, w, source, l, tol, false)
}
func writeRationalSphericalHierarchySource(ctx context.Context, w io.Writer, source *File, l Limits, tol float64, occt bool) error {
	if ctx == nil || w == nil || !independentNativeSourceBound(source, 4096) {
		return geom.ErrInvalid
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), l, tol, occt); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
