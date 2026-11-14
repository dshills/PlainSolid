package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// BiquadraticHexahedronSolid retains original source geometry and a fully
// certified native spline material, never its temporary planar chart scaffold.
type BiquadraticHexahedronSolid struct {
	LengthUnit                                                         float64
	Solid                                                              trimmed.Solid
	SourceSolidID, SourceShellID, SourceVertexListID, SourceEdgeListID int
	SourceFaceIDs                                                      [6]int
	CurvedSurfaceID                                                    int
	Geometry                                                           map[int]Entity
	Metadata                                                           map[int]CylinderDatumMetadata
	Embedding                                                          trimmed.SplineRectangleEmbedding
}

func biquadFailure() error {
	return fmt.Errorf("iges: unsupported or uncertified biquadratic hexahedron")
}
func biquadFile(d topologySource, replaceID int, replacement Entity) ([]byte, error) {
	a := topologyAuthor{}
	zero := geom.Vec3{}
	a.geometry.Entities = []Entity{{Point: &zero}}
	for _, r := range d.entities {
		var data string
		if r.id == replaceID {
			_, data = encode(replacement)
		} else {
			v := make([]string, len(r.values))
			for i, p := range r.values {
				v[i] = p.text
			}
			data = joinTopologyValues(v)
		}
		form := map[int]int{502: 1, 504: 1, 508: 1, 510: 1, 514: 1}[r.kind]
		a.add(r.kind, form, data)
		a.records[len(a.records)-1].label = r.label
		a.records[len(a.records)-1].subscript = r.subscript
	}
	return a.file(d.unit)
}

// ReadBiquadraticHexahedronSolid requires one original full 3x3 polynomial
// biquadratic chart and five affine quad charts. All original model/pcurve
// compositions are compared exactly; the sewn original charts must additionally
// pass the complete global spline-rectangle embedding certificate.
func ReadBiquadraticHexahedronSolid(ctx context.Context, r io.Reader, l Limits) (BiquadraticHexahedronSolid, error) {
	fail := func() (BiquadraticHexahedronSolid, error) { return BiquadraticHexahedronSolid{}, biquadFailure() }
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return BiquadraticHexahedronSolid{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 58)
	l.MaxRecords = min(l.MaxRecords, 1024)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxControlPoints = min(l.MaxControlPoints, 101)
	d, e := readTopologySource(r, l, true)
	if e != nil {
		return BiquadraticHexahedronSolid{}, e
	}
	if len(d.entities) != 58 {
		return fail()
	}
	// Appearance/level references are outside this material-only dialect.
	for _, line := range bytes.Split(d.bytes, []byte("\n")) {
		if len(line) == 80 && line[72] == 'D' {
			seq, err := field(string(line[73:]))
			if err != nil {
				return fail()
			}
			indexes := []int{1, 2}
			if seq%2 == 1 {
				kind, err := field(string(line[:8]))
				if err != nil {
					return fail()
				}
				status, err := field(string(line[64:72]))
				if err != nil {
					return fail()
				}
				want := 10000
				if kind == 186 {
					want = 0
				}
				if kind == 504 {
					want = 10001
				}
				if status != want {
					return fail()
				}
				indexes = []int{3, 4}
			}
			for _, j := range indexes {
				v, err := field(string(line[j*8 : j*8+8]))
				if err != nil || v != 0 {
					return fail()
				}
			}
		}
	}
	records := map[int]topologyRecord{}
	var curvedID int
	var original Entity
	for _, rec := range d.entities {
		records[rec.id] = rec
		if rec.kind != 128 {
			continue
		}
		if e = compositeSourceNumbers(110, rec.values); e != nil {
			return fail()
		}
		en, e := decode(128, rec.values, l.MaxControlPoints)
		if e != nil || en.Surface == nil || en.sourceWeightsRounded {
			return fail()
		}
		if e = en.validate(); e != nil {
			return fail()
		}
		sd := en.Surface.Geometry.Data()
		if sd.DegreeU == 1 && sd.DegreeV == 1 {
			continue
		}
		if curvedID != 0 || sd.DegreeU != 2 || sd.DegreeV != 2 || sd.CountU != 3 || sd.CountV != 3 {
			return fail()
		}
		s := en.Surface
		if s.UStart != 0 || s.UEnd != 1 || s.VStart != 0 || s.VEnd != 1 || !s.Polynomial || s.ClosedU || s.ClosedV || s.PeriodicU || s.PeriodicV {
			return fail()
		}
		for _, k := range [][]float64{sd.KnotsU, sd.KnotsV} {
			if len(k) != 6 || k[0] != 0 || k[1] != 0 || k[2] != 0 || k[3] != 1 || k[4] != 1 || k[5] != 1 {
				return fail()
			}
		}
		for _, w := range sd.Weights {
			if w != 1 {
				return fail()
			}
		}
		curvedID, original = rec.id, en
	}
	if curvedID == 0 {
		return fail()
	}
	sd := original.Surface.Geometry.Data()
	flat, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{sd.Poles[0], sd.Poles[2], sd.Poles[6], sd.Poles[8]}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		return fail()
	}
	raw, e := biquadFile(d, curvedID, Entity{Surface: &Surface{Geometry: flat, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1, Polynomial: true}})
	if e != nil {
		return fail()
	}
	scaffold, e := ReadHexahedronSolid(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	// Count original controls separately: normalization must not bypass the budget.
	if l.MaxControlPoints < 101 {
		return fail()
	}
	geometry := scaffold.Geometry
	geometry[curvedID] = original
	edgeValues, e := topologyInts(records[scaffold.SourceEdgeListID], 62)
	if e != nil {
		return fail()
	}
	surfaces := make([]*geom.BSplineSurface, 6)
	options := curvedOvalOptions()
	for fi, fid := range scaffold.SourceFaceIDs {
		if e = ctx.Err(); e != nil {
			return BiquadraticHexahedronSolid{}, e
		}
		fv, e := topologyInts(records[fid], 5)
		if e != nil {
			return fail()
		}
		sid := fv[1]
		surfaces[fi] = geometry[sid].Surface.Geometry
		patch, e := geom.NewSurfacePatch(surfaces[fi], options)
		if e != nil {
			return fail()
		}
		lv, e := topologyInts(records[fv[4]], 30)
		if e != nil {
			return fail()
		}
		for j := 0; j < 4; j++ {
			a := lv[2+7*j : 9+7*j]
			modelID := edgeValues[2+5*(a[2]-1)]
			pc := geometry[a[6]].Spline
			model := geometry[modelID].Spline
			uv, e := geom.NewExactSplineCurve(pc.Curve, options)
			if e != nil {
				return fail()
			}
			lift, e := patch.ComposeParameterCurve(ctx, uv, options)
			if e != nil {
				return BiquadraticHexahedronSolid{}, e
			}
			want, e := geom.NewExactSplineCurve(model.Curve, options)
			if e != nil {
				return fail()
			}
			proof, e := geom.CompareExactCurvesContext(ctx, lift, want, geom.ExactCurveComparisonOptions{})
			if e != nil {
				return BiquadraticHexahedronSolid{}, e
			}
			if !proof.Complete || !proof.Equal {
				return fail()
			}
		}
	}
	solid, _, e := trimmed.SewSplineRectangles(ctx, surfaces, make([]geom.ParameterRectangle, 6), make([]bool, 6), trimmed.SewOptions{Tolerance: 1e-10, MaxFaces: 6, MaxUses: 24})
	if e != nil {
		return BiquadraticHexahedronSolid{}, e
	}
	embedding, e := solid.ValidateSplineRectangleEmbedding(ctx, trimmed.SplineRectangleEmbeddingOptions{MaxFaces: 6})
	if e != nil {
		return BiquadraticHexahedronSolid{}, e
	}
	if !embedding.Complete {
		return fail()
	}
	return BiquadraticHexahedronSolid{LengthUnit: d.unit, Solid: solid, SourceSolidID: scaffold.SourceSolidID, SourceShellID: scaffold.SourceShellID, SourceVertexListID: scaffold.SourceVertexListID, SourceEdgeListID: scaffold.SourceEdgeListID, SourceFaceIDs: scaffold.SourceFaceIDs, CurvedSurfaceID: curvedID, Geometry: geometry, Metadata: scaffold.Metadata, Embedding: embedding}, nil
}

// WriteBiquadraticHexahedronSolid replaces exactly one canonical hexahedron face
// support with the supplied original full biquadratic chart. Face numbering is
// bottom, top, V-min, V-max, U-min, U-max. Every source rim and global embedding
// must prove unchanged/valid before output. The chart is not fitted or repaired.
func WriteBiquadraticHexahedronSolid(ctx context.Context, w io.Writer, vertices [8]geom.Vec3, face int, surface *geom.BSplineSurface, unit float64) error {
	if ctx == nil || w == nil || face < 0 || face >= 6 || surface == nil {
		return biquadFailure()
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	desc := surface.Data()
	if desc.DegreeU != 2 || desc.DegreeV != 2 || desc.CountU != 3 || desc.CountV != 3 {
		return biquadFailure()
	}
	var b bytes.Buffer
	if e := WriteHexahedronSolid(&b, vertices, unit); e != nil {
		return e
	}
	m, e := ReadHexahedronSolid(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		return e
	}
	d, e := readTopologySource(bytes.NewReader(b.Bytes()), Limits{}, true)
	if e != nil {
		return e
	}
	sid := 0
	for _, r := range d.entities {
		if r.id == m.SourceFaceIDs[face] {
			v, e := topologyInts(r, 5)
			if e != nil {
				return e
			}
			sid = v[1]
		}
	}
	raw, e := biquadFile(d, sid, Entity{Surface: &Surface{Geometry: surface, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1, Polynomial: true}})
	if e != nil {
		return e
	}
	if _, e = ReadBiquadraticHexahedronSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
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
