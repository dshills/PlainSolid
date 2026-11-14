package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// ImportedObliqueCylinderTorusCavity is an immutable reconstruction from observable
// analytic supports and the complete paired STEP topology, without payloads.
type ImportedObliqueCylinderTorusCavity struct {
	source ObliqueCylinderTorusCavityBody
	unit   float64
	query  geom.ObliqueCylinderTorusCavityMaterial
	valid  bool
}

func (s ImportedObliqueCylinderTorusCavity) Source() ObliqueCylinderTorusCavityBody {
	out := s.source
	if s.valid {
		for i := range out.Frame {
			for j := range out.Frame[i] {
				out.Frame[i][j] = new(big.Rat).Set(s.source.Frame[i][j])
			}
		}
	}
	return out
}
func (s ImportedObliqueCylinderTorusCavity) Query() geom.ObliqueCylinderTorusCavityMaterial {
	return s.query
}
func (s ImportedObliqueCylinderTorusCavity) LengthUnit() float64 { return s.unit }
func (s ImportedObliqueCylinderTorusCavity) WriteSTEP(w io.Writer) error {
	if !s.valid {
		return geom.ErrInvalid
	}
	return WriteObliqueCylinderTorusCavity(w, s.source, s.unit)
}

// ReadObliqueCylinderTorusCavity accepts the complete writer graph modulo entity IDs
// and exactly equal decimal numeric atoms. Encounter order is preserved. It
// rejects detached entities, changed pcurves, senses and supports, and is not a
// general healed OCCT dialect reader. Limits cap parsing at 512 entities,
// 16384 values and 2 MiB; two fixed-size analytic source proofs are used.
func ReadObliqueCylinderTorusCavity(r io.Reader, l Limits) (ImportedObliqueCylinderTorusCavity, error) {
	var zero ImportedObliqueCylinderTorusCavity
	if r == nil {
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
	counts := map[string][]int64{}
	for _, id := range f.Order {
		for _, rec := range f.Entities[id].Records {
			counts[rec.Type] = append(counts[rec.Type], id)
		}
	}
	for typ, n := range map[string]int{"TOROIDAL_SURFACE": 1, "CYLINDRICAL_SURFACE": 1, "PLANE": 2, "BREP_WITH_VOIDS": 1, "ORIENTED_CLOSED_SHELL": 1, "CLOSED_SHELL": 2, "ADVANCED_FACE": 4, "ADVANCED_BREP_SHAPE_REPRESENTATION": 1, "MANIFOLD_SOLID_BREP": 0} {
		if len(counts[typ]) != n {
			return zero, geom.ErrInvalid
		}
	}
	if l.MaxFaces < 4 || len(counts["ORIENTED_EDGE"]) > l.MaxEdgeUses || len(counts["VERTEX_POINT"]) > l.MaxVertices {
		return zero, geom.ErrInvalid
	}
	var b ObliqueCylinderTorusCavityBody
	torus, e := simple(f, counts["TOROIDAL_SURFACE"][0], "TOROIDAL_SURFACE", 4)
	if e != nil {
		return zero, e
	}
	pid, e := ref(torus.Args[1])
	if e != nil {
		return zero, e
	}
	pose, e := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return zero, e
	}
	var vectors [3][3]*big.Rat
	for k := 0; k < 3; k++ {
		id, e := ref(pose.Args[k+1])
		if e != nil {
			return zero, e
		}
		typ := "DIRECTION"
		if k == 0 {
			typ = "CARTESIAN_POINT"
		}
		rec, e := simple(f, id, typ, 2)
		if e != nil {
			return zero, e
		}
		if rec.Args[1].Kind != Aggregate || len(rec.Args[1].Items) != 3 {
			return zero, geom.ErrInvalid
		}
		for j := 0; j < 3; j++ {
			vectors[k][j], e = torusCylinderImportNumber(rec.Args[1].Items[j])
			if e != nil {
				return zero, e
			}
		}
	}
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		b.Frame[i][0] = vectors[2][i]
		b.Frame[i][2] = vectors[1][i]
		b.Frame[i][3] = vectors[0][i]
		b.Frame[i][1] = new(big.Rat).Sub(new(big.Rat).Mul(vectors[1][j], vectors[2][k]), new(big.Rat).Mul(vectors[1][k], vectors[2][j]))
	}
	wall, e := simple(f, counts["CYLINDRICAL_SURFACE"][0], "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return zero, e
	}
	if b.MajorRadius, e = torusCylinderImportScalar(torus.Args[2]); e != nil {
		return zero, e
	}
	if b.MinorRadius, e = torusCylinderImportScalar(torus.Args[3]); e != nil {
		return zero, e
	}
	if b.CylinderRadius, e = torusCylinderImportScalar(wall.Args[2]); e != nil {
		return zero, e
	}
	root, e := simple(f, counts["BREP_WITH_VOIDS"][0], "BREP_WITH_VOIDS", 3)
	if e != nil {
		return zero, e
	}
	if b.Name, e = textValue(root.Args[0]); e != nil {
		return zero, e
	}
	for i, id := range counts["PLANE"] {
		plane, e := simple(f, id, "PLANE", 2)
		if e != nil {
			return zero, e
		}
		pid, e := ref(plane.Args[1])
		if e != nil {
			return zero, e
		}
		pose, e := simple(f, pid, "AXIS2_PLACEMENT_3D", 4)
		if e != nil {
			return zero, e
		}
		oid, e := ref(pose.Args[1])
		if e != nil {
			return zero, e
		}
		point, e := simple(f, oid, "CARTESIAN_POINT", 2)
		if e != nil {
			return zero, e
		}
		if point.Args[1].Kind != Aggregate || len(point.Args[1].Items) != 3 {
			return zero, geom.ErrInvalid
		}
		z, e := torusCylinderImportScalar(point.Args[1].Items[2])
		if e != nil {
			return zero, e
		}
		if i == 0 {
			b.Lower = z
		} else {
			b.Upper = z
		}
	}
	if b.Lower > b.Upper {
		b.Lower, b.Upper = b.Upper, b.Lower
	}
	rep, e := simple(f, counts["ADVANCED_BREP_SHAPE_REPRESENTATION"][0], "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return zero, e
	}
	cid, e := ref(rep.Args[2])
	if e != nil {
		return zero, e
	}
	unit, e := analyticSupportContextUnits(f, cid)
	if e != nil {
		return zero, e
	}
	var raw bytes.Buffer
	if e = WriteObliqueCylinderTorusCavity(&raw, b, unit); e != nil {
		return zero, e
	}
	canonical, e := Parse(&raw, l)
	if e != nil {
		return zero, e
	}
	if !torusCylinderSameGraph(f, canonical) {
		return zero, geom.ErrInvalid
	}
	native, e := geom.NewObliqueCylinderTorusCavityMaterial(context.Background(), b.MajorRadius, b.MinorRadius, b.CylinderRadius, b.Lower, b.Upper, b.Frame, geom.SurfacePatchOptions{})
	if e != nil {
		return zero, e
	}
	return ImportedObliqueCylinderTorusCavity{b, unit, native, true}, nil
}
