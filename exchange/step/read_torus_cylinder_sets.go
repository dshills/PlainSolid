package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strconv"
	"strings"
)

// ImportedTorusCylinder is an immutable native reconstruction of the paired
// analytic STEP dialect. Source returns original observable support scalars.
// For Common/Cut the absent tool caps are explicitly synthetic, never inferred
// as authored history. No vendor payload is used to authenticate the boundary.
type ImportedTorusCylinder struct {
	source                TorusCylinderBody
	unit, tolerance       float64
	retainedCaps          bool
	solid                 parametric.Solid
	query                 geom.TorusCylinderMaterial
	radius, heightSquared *big.Rat
	roots                 [2]*geom.ExactAlgebraicRoot
}

func (s ImportedTorusCylinder) Source() (TorusCylinderBody, bool) { return s.source, s.retainedCaps }
func (s ImportedTorusCylinder) LengthUnit() float64               { return s.unit }
func (s ImportedTorusCylinder) Solid() parametric.Solid           { return s.solid }
func (s ImportedTorusCylinder) Query() geom.TorusCylinderMaterial { return s.query }
func (s ImportedTorusCylinder) RimProof() (*big.Rat, *big.Rat, [2]*geom.ExactAlgebraicRoot, error) {
	if s.radius == nil {
		return nil, nil, [2]*geom.ExactAlgebraicRoot{}, geom.ErrInvalid
	}
	return new(big.Rat).Set(s.radius), new(big.Rat).Set(s.heightSquared), s.roots, nil
}

// SourceFaces maps original torus/wall/lower-cap/upper-cap ownership to the
// deterministic reconstructed native face IDs, with absent caps left empty.
func (s ImportedTorusCylinder) SourceFaces() ([4][]int, error) {
	if s.radius == nil {
		return [4][]int{}, geom.ErrInvalid
	}
	if s.retainedCaps {
		return [4][]int{{0}, {1, 2}, {3}, {4}}, nil
	}
	return [4][]int{{0}, {1}, nil, nil}, nil
}
func (s ImportedTorusCylinder) RimEdges() ([2]int, error) {
	if s.radius == nil {
		return [2]int{}, geom.ErrInvalid
	}
	return [2]int{0, 2}, nil
}
func (s ImportedTorusCylinder) WriteSTEP(w io.Writer) error {
	if s.radius == nil {
		return geom.ErrInvalid
	}
	return WriteTorusCylinderSets(w, TorusCylinderModel{[]TorusCylinderBody{s.source}, s.unit, s.tolerance})
}

// ReadTorusCylinderSet accepts one body in the exact graph dialect written by
// WriteTorusCylinderSets. Entity IDs may be relabelled, but encounter ordering
// and every support/curve/pcurve/adjacency/orientation must match the proved
// native construction. Numeric atoms are compared as exact decimal rationals,
// not with tolerance. The explicit tolerance only bounds represented irrational
// rim coordinates against the original exact source. Genuine OCCT exports and
// independently healed/reordered graphs are not accepted by this first dialect.
//
// The parser is additionally bounded to 512 entities, 16384 values and 2 MiB.
// At most two native factory proofs run, each with its fixed exact root budget;
// no graph search or unbounded geometric discovery occurs.
func ReadTorusCylinderSet(r io.Reader, l Limits, tolerance float64) (ImportedTorusCylinder, error) {
	var zero ImportedTorusCylinder
	if r == nil || !finiteNumber(tolerance) || tolerance <= 0 {
		return zero, geom.ErrInvalid
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxEntities = min(l.MaxEntities, 512)
	l.MaxValues = min(l.MaxValues, 16384)
	l.MaxBytes = min(l.MaxBytes, 2<<20)
	f, e := Parse(r, l)
	if e != nil {
		return zero, e
	}
	fail := func() (ImportedTorusCylinder, error) {
		return zero, fmt.Errorf("step: unsupported or altered transverse torus-cylinder graph")
	}
	counts := map[string][]int64{}
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			counts[rec.Type] = append(counts[rec.Type], id)
		}
	}
	if len(counts["TOROIDAL_SURFACE"]) != 1 || len(counts["CYLINDRICAL_SURFACE"]) != 1 || len(counts["MANIFOLD_SOLID_BREP"]) != 1 || len(counts["CLOSED_SHELL"]) != 1 || len(counts["ADVANCED_BREP_SHAPE_REPRESENTATION"]) != 1 {
		return fail()
	}
	source := TorusCylinderBody{}
	torus, e := simple(f, counts["TOROIDAL_SURFACE"][0], "TOROIDAL_SURFACE", 4)
	if e != nil {
		return fail()
	}
	wall, e := simple(f, counts["CYLINDRICAL_SURFACE"][0], "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return fail()
	}
	if source.MajorRadius, e = torusCylinderImportScalar(torus.Args[2]); e != nil {
		return fail()
	}
	if source.MinorRadius, e = torusCylinderImportScalar(torus.Args[3]); e != nil {
		return fail()
	}
	if source.CylinderRadius, e = torusCylinderImportScalar(wall.Args[2]); e != nil {
		return fail()
	}
	brep, e := simple(f, counts["MANIFOLD_SOLID_BREP"][0], "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	if source.Name, e = textValue(brep.Args[0]); e != nil {
		return fail()
	}
	faces := counts["ADVANCED_FACE"]
	retained := len(faces) == 5
	if retained {
		if len(counts["PLANE"]) != 2 {
			return fail()
		}
		source.Operation = geom.TorusCylinderUnion
		for i, id := range counts["PLANE"] {
			plane, e := simple(f, id, "PLANE", 2)
			if e != nil {
				return fail()
			}
			pid, e := ref(plane.Args[1])
			if e != nil {
				return fail()
			}
			pl, e := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
			if e != nil {
				return fail()
			}
			oid, e := ref(pl.Args[1])
			if e != nil {
				return fail()
			}
			p, e := simple(f, oid, "CARTESIAN_POINT", 2)
			if e != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 3 {
				return fail()
			}
			z, e := torusCylinderImportScalar(p.Args[1].Items[2])
			if e != nil {
				return fail()
			}
			if i == 0 {
				source.Lower = z
			} else {
				source.Upper = z
			}
		}
	} else {
		if len(faces) != 2 || len(counts["PLANE"]) != 0 {
			return fail()
		}
		side, e := simple(f, faces[1], "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		if side.Args[3].Kind != Enumeration {
			return fail()
		}
		switch side.Args[3].Text {
		case "T":
			source.Operation = geom.TorusCylinderCommon
		case "F":
			source.Operation = geom.TorusCylinderDifference
		default:
			return fail()
		}
		// The original unused caps cannot be recovered from these retained faces.
		// Since r<R<+Inf, nextafter(r,+Inf) exists and strictly exceeds r. Its
		// reflected pair is a safe, exact represented clear proxy at every scale.
		source.Upper = math.Nextafter(source.MinorRadius, math.Inf(1))
		source.Lower = -source.Upper
	}
	if len(faces) > l.MaxFaces || len(counts["ORIENTED_EDGE"]) > l.MaxEdgeUses || len(counts["VERTEX_POINT"])+2*torusCylinderBoolInt(retained) > l.MaxVertices {
		return fail()
	}
	rep, e := simple(f, counts["ADVANCED_BREP_SHAPE_REPRESENTATION"][0], "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	ctx, e := ref(rep.Args[2])
	if e != nil {
		return fail()
	}
	unit, e := analyticSupportContextUnits(f, ctx)
	if e != nil {
		return fail()
	}
	var canonical bytes.Buffer
	if e = WriteTorusCylinderSets(&canonical, TorusCylinderModel{[]TorusCylinderBody{source}, unit, tolerance}); e != nil {
		return zero, e
	}
	expected, e := Parse(bytes.NewReader(canonical.Bytes()), l)
	if e != nil {
		return zero, e
	}
	if !torusCylinderSameGraph(f, expected) {
		return fail()
	}
	out := ImportedTorusCylinder{source: source, unit: unit, tolerance: tolerance, retainedCaps: retained}
	if retained {
		s, e := parametric.NewTorusCylinderUnionSet(context.Background(), source.MajorRadius, source.MinorRadius, source.CylinderRadius, source.Lower, source.Upper, tolerance)
		if e != nil {
			return zero, e
		}
		out.solid, out.query = s.Solid(), s.Query()
		out.radius, out.heightSquared, out.roots, e = s.RimProof()
		if e != nil {
			return zero, e
		}
	} else {
		s, e := parametric.NewTorusCylinderSet(context.Background(), source.MajorRadius, source.MinorRadius, source.CylinderRadius, source.Lower, source.Upper, source.Operation, tolerance)
		if e != nil {
			return zero, e
		}
		out.solid, out.query = s.Solid(), s.Query()
		out.radius, out.heightSquared, out.roots, e = s.RimProof()
		if e != nil {
			return zero, e
		}
	}
	return out, nil
}
func torusCylinderBoolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func torusCylinderImportNumber(v Value) (*big.Rat, error) {
	if v.Kind != Number || len(v.Text) > 1100 {
		return nil, geom.ErrDomain
	}
	if i := strings.IndexAny(v.Text, "eE"); i >= 0 {
		x, e := strconv.Atoi(v.Text[i+1:])
		if e != nil || x < -1100 || x > 1100 {
			return nil, geom.ErrDomain
		}
	}
	q, ok := new(big.Rat).SetString(v.Text)
	if !ok || q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
		return nil, geom.ErrDomain
	}
	return q, nil
}
func torusCylinderImportScalar(v Value) (float64, error) {
	q, e := torusCylinderImportNumber(v)
	if e != nil {
		return 0, e
	}
	x, exact := q.Float64()
	if !exact || !finiteNumber(x) {
		return 0, geom.ErrDomain
	}
	return x, nil
}
func torusCylinderSameGraph(a, b *File) bool {
	if len(a.Order) != len(b.Order) {
		return false
	}
	mapID := map[int64]int64{}
	for i, id := range a.Order {
		mapID[id] = b.Order[i]
	}
	var equal func(Value, Value) bool
	equal = func(x, y Value) bool {
		if x.Kind != y.Kind {
			return false
		}
		if x.Kind == Reference {
			return mapID[x.Ref] == y.Ref
		}
		if x.Kind == Number {
			p, e := torusCylinderImportNumber(x)
			q, f := torusCylinderImportNumber(y)
			return e == nil && f == nil && p.Cmp(q) == 0
		}
		if x.Text != y.Text || len(x.Items) != len(y.Items) {
			return false
		}
		for i := range x.Items {
			if !equal(x.Items[i], y.Items[i]) {
				return false
			}
		}
		return true
	}
	for i, id := range a.Order {
		x, y := a.Entities[id], b.Entities[b.Order[i]]
		if len(x.Records) != len(y.Records) {
			return false
		}
		for j, r := range x.Records {
			s := y.Records[j]
			if r.Type != s.Type || len(r.Args) != len(s.Args) {
				return false
			}
			for k := range r.Args {
				if !equal(r.Args[k], s.Args[k]) {
					return false
				}
			}
		}
	}
	return true
}
