package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// EllipticalHole preserves a distinct original model/pcurve owner and exact
// enclosing-rectangle containment certificate in the common plane chart.
type EllipticalHole struct {
	Boundary                                                                        geom.Ellipse
	Vertex                                                                          geom.Vec3
	MajorRadius, MinorRadius                                                        float64
	SourceBoundID, SourceEdgeID, SourceVertexID, SourcePCurveID, SourceModelCurveID int64
	Clearance                                                                       EccentricEllipseClearance
}

// TwoEllipticalHoleFace is one planar face, not a solid. Native edge roles are
// outer0 and holes1,2 in original FACE_BOUND order. SeparationLower is an exact
// physical lower bound from disjoint UV enclosure intervals along Axis (0 or1).
type TwoEllipticalHoleFace struct {
	EllipticalDiskFace
	Holes           [2]EllipticalHole
	SeparationAxis  int
	SeparationLower *big.Rat
}

func twoEllipseHoleFailure() error {
	return fmt.Errorf("step: unsupported or unproved two-ellipse-hole face")
}

// ReadTwoEllipticalHoleFace authenticates the entire original85-record OCCT
// graph by two strictly proved owner partitions whose only shared geometry is
// the original plane/outer boundary. Both holes must satisfy the existing exact
// eccentric/cardinal containment gate, and their UV rectangles must separate.
func ReadTwoEllipticalHoleFace(r io.Reader, l Limits) (TwoEllipticalHoleFace, error) {
	fail := func() (TwoEllipticalHoleFace, error) { return TwoEllipticalHoleFace{}, twoEllipseHoleFailure() }
	if r == nil {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 85)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return TwoEllipticalHoleFace{}, e
	}
	if len(f.Entities) != 85 {
		return fail()
	}
	var fid int64
	for id, en := range f.Entities {
		for _, rec := range en.Records {
			if rec.Type == "ADVANCED_FACE" {
				if fid != 0 {
					return fail()
				}
				fid = id
			}
		}
	}
	fr, e := simple(f, fid, "ADVANCED_FACE", 4)
	if e != nil {
		return fail()
	}
	bounds, e := listRefs(fr.Args[1], 1)
	if e != nil || len(bounds) != 3 || bounds[1] == bounds[2] {
		return fail()
	}
	planeID, e := ref(fr.Args[2])
	if e != nil {
		return fail()
	}
	if _, e = simple(f, planeID, "PLANE", 2); e != nil {
		return fail()
	}
	// Exactly nineteen exclusive records form each qualified original hole.
	// Stop at the explicitly shared plane; every other reached record is owned.
	var owned [2]map[int64]bool
	for i := range 2 {
		owned[i] = map[int64]bool{}
		active := map[int64]bool{}
		var visit func(int64, int) bool
		visit = func(id int64, depth int) bool {
			if id == planeID {
				return true
			}
			if depth > 24 || active[id] {
				return false
			}
			if owned[i][id] {
				return true
			}
			en, ok := f.Entities[id]
			if !ok {
				return false
			}
			active[id] = true
			owned[i][id] = true
			if len(owned[i]) > 19 {
				return false
			}
			var walk func(Value) bool
			walk = func(v Value) bool {
				if v.Kind == Reference {
					return visit(v.Ref, depth+1)
				}
				for _, a := range v.Items {
					if !walk(a) {
						return false
					}
				}
				return true
			}
			for _, rec := range en.Records {
				for _, v := range rec.Args {
					if !walk(v) {
						return false
					}
				}
			}
			active[id] = false
			return true
		}
		if !visit(bounds[i+1], 0) || len(owned[i]) != 19 {
			return fail()
		}
	}
	for id := range owned[0] {
		if owned[1][id] {
			return fail()
		}
	}
	var parts [2]EccentricEllipticalFace
	for i := range 2 {
		g := &File{Header: f.Header, Entities: map[int64]Entity{}}
		for _, id := range f.Order {
			if owned[1-i][id] {
				continue
			}
			g.Order = append(g.Order, id)
			g.Entities[id] = f.Entities[id]
		}
		en := g.Entities[fid]
		en.Records = append([]Record(nil), en.Records...)
		en.Records[0].Args = append([]Value(nil), en.Records[0].Args...)
		en.Records[0].Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: bounds[0]}, {Kind: Reference, Ref: bounds[i+1]}}}
		g.Entities[fid] = en
		raw, e := mappedPlanarSourceBytes(g)
		if e != nil {
			return fail()
		}
		parts[i], e = ReadEccentricEllipticalFace(bytes.NewReader(raw), l)
		if e != nil {
			return fail()
		}
		if parts[i].SourceFaceID != fid || parts[i].SourcePlaneID != planeID || parts[i].SourceInnerBoundID != bounds[i+1] {
			return fail()
		}
	}
	p, q := parts[0], parts[1]
	if p.Boundary != q.Boundary || p.Vertex != q.Vertex || p.Face.Surface != q.Face.Surface || p.Name != q.Name || p.LengthUnit != q.LengthUnit || p.SourceEdgeID != q.SourceEdgeID || p.SourcePCurveID != q.SourcePCurveID {
		return fail()
	}
	// Disjoint full bounding rectangles imply disjoint complete curves/interiors.
	var lo, hi [2][2]*big.Rat
	for i, p := range parts {
		center := []float64{p.Clearance.Center.X, p.Clearance.Center.Y}
		ext := []float64{p.InnerMajorRadius, p.InnerMinorRadius}
		if p.Clearance.Reference.X == 0 {
			ext[0], ext[1] = ext[1], ext[0]
		}
		for j := range 2 {
			c := new(big.Rat).SetFloat64(center[j])
			r := new(big.Rat).SetFloat64(ext[j])
			lo[i][j] = new(big.Rat).Sub(c, r)
			hi[i][j] = new(big.Rat).Add(c, r)
		}
	}
	gap := new(big.Rat)
	axis := -1
	for j := range 2 {
		for _, v := range []*big.Rat{new(big.Rat).Sub(lo[1][j], hi[0][j]), new(big.Rat).Sub(lo[0][j], hi[1][j])} {
			if v.Cmp(gap) > 0 {
				gap.Set(v)
				axis = j
			}
		}
	}
	if axis < 0 {
		return fail()
	}
	result := TwoEllipticalHoleFace{EllipticalDiskFace: p.EllipticalDiskFace, SeparationAxis: axis, SeparationLower: gap}
	result.Source = f
	result.Face.Holes = make([][]trimmed.Use, 2)
	for i, p := range parts {
		use := p.Face.Holes[0][0]
		use.Edge = i + 1
		result.Face.Holes[i] = []trimmed.Use{use}
		result.Holes[i] = EllipticalHole{Boundary: p.InnerBoundary, Vertex: p.InnerVertex, MajorRadius: p.InnerMajorRadius, MinorRadius: p.InnerMinorRadius, SourceBoundID: p.SourceInnerBoundID, SourceEdgeID: p.SourceInnerEdgeID, SourceVertexID: p.SourceInnerVertexID, SourcePCurveID: p.SourceInnerPCurveID, SourceModelCurveID: p.SourceInnerModelCurveID, Clearance: p.Clearance}
	}
	return result, nil
}

// WriteTwoEllipticalHoleFaceSource re-proves all owners, containment and pairwise
// separation before re-emitting the original graph. No native edits are baked.
func WriteTwoEllipticalHoleFaceSource(w io.Writer, f *File, l Limits) error {
	if w == nil || !ellipseOwnedSourceBound(f, 85) {
		return twoEllipseHoleFailure()
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadTwoEllipticalHoleFace(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
