package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"reflect"
)

type igesLensProvenance struct {
	Solid, Shell, EdgeList, VertexList int
	Faces, ModelCurves                 []int
	Cells                              [][]geom.ParameterRectangle
	ParameterCurves                    [][]int
	Center, Radii                      geom.Vec2
	Shear                              float64
	Geometry                           string
}

// ReadSplineLensIGES imports one certified axis/sheared/refined graph lens as
// an exact native trimmed asset, preserving explicit IGES source roles.
func ReadSplineLensIGES(r io.Reader, l iges.Limits, historyLimit int) (*Document, error) {
	m, e := iges.ReadSplineLensSolid(r, l)
	if e != nil {
		return nil, e
	}
	return fromVerifiedIGESLens(m, historyLimit)
}

// FromSplineLensIGES re-proves caller geometry through an authored IGES graph.
// Its source IDs describe that staged graph, not caller-supplied ancestry.
func FromSplineLensIGES(m iges.SplineLensSolid, historyLimit int) (*Document, error) {
	var b bytes.Buffer
	if e := iges.WriteSplineLensSolid(&b, m); e != nil {
		return nil, e
	}
	return ReadSplineLensIGES(&b, iges.Limits{}, historyLimit)
}
func fromVerifiedIGESLens(m iges.SplineLensSolid, historyLimit int) (*Document, error) {
	d, e := FromTrimmedSTEP(step.TrimmedModel{LengthUnit: m.LengthUnit, Bodies: []step.TrimmedBody{{Name: "IGES spline lens", Solid: m.Solid}}}, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	digest, e := lensGeometryDigest(m.Solid)
	if e != nil {
		return nil, e
	}
	p := igesLensProvenance{Solid: m.SourceSolidID, Shell: m.SourceShellID, EdgeList: m.SourceEdgeListID, VertexList: m.SourceVertexListID, Faces: append([]int(nil), m.SourceFaceIDs[:]...), ModelCurves: append([]int(nil), m.SourceModelCurveIDs[:]...), Center: m.Center, Radii: m.Radii, Shear: m.Shear, Geometry: digest}
	for _, cells := range m.SourceKnotCells {
		p.Cells = append(p.Cells, append([]geom.ParameterRectangle(nil), cells[:]...))
	}
	for _, ids := range m.SourceParameterCurveIDs {
		p.ParameterCurves = append(p.ParameterCurves, append([]int(nil), ids[:]...))
	}
	raw, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	s.Labels = map[string]Label{"iges:body-000001": {Name: "IGES spline lens", Target: AssetLabel, TargetID: "body-000001", Attributes: map[string]Attribute{"iges.lens.provenance": {Kind: "string", Text: string(raw)}, "iges.lens.binding": {Kind: "string", Text: lensProvenanceDigest(raw)}, "iges.source.solid": {Kind: "integer", Integer: int64(p.Solid)}}}}
	return New(s, historyLimit)
}

// WriteSplineLensIGES exports one unchanged, identity-placed native lens.
// It re-proves native geometry and original chart-cell lineage before output;
// assemblies, placements, stale provenance and unsupported edits are rejected.
func (s State) WriteSplineLensIGES(w io.Writer) error { return s.writeSplineLensIGES(w, false) }
func (s State) writeSplineLensIGES(w io.Writer, rational bool) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := s.Validate(); e != nil {
		return e
	}
	if len(s.Nodes) > 256 || len(s.Features) != 0 || len(s.Assets) != 0 || len(s.Planar) != 0 || len(s.Parametric) != 0 || len(s.Trimmed) != 1 {
		return fmt.Errorf("document: one IGES lens asset required")
	}
	asset := ""
	for _, n := range s.Nodes {
		if n.Transform != geom.Identity() {
			return fmt.Errorf("document: IGES lens placement must be identity")
		}
		if n.Asset != "" {
			if asset != "" {
				return fmt.Errorf("document: one IGES lens occurrence required")
			}
			asset = n.Asset
		}
	}
	solid, ok := s.Trimmed[asset]
	if !ok {
		return geom.ErrInvalid
	}
	label, ok := s.Labels["iges:"+asset]
	if !ok || label.Target != AssetLabel || label.TargetID != asset {
		return geom.ErrInvalid
	}
	if _, has := label.Attributes["iges.lens.partitioned.recipe"]; has {
		return fmt.Errorf("document: partitioned IGES recipe requires explicit partitioned export")
	}
	if _, has := label.Attributes["iges.lens.rational.recipe"]; has && !rational {
		return fmt.Errorf("document: raw IGES recipe requires explicit rational export")
	}
	raw, binding := label.Attributes["iges.lens.provenance"], label.Attributes["iges.lens.binding"]
	if raw.Kind != "string" || binding.Kind != "string" || len(raw.Text) > 8192 || binding.Text != lensProvenanceDigest([]byte(raw.Text)) {
		return fmt.Errorf("document: stale IGES lens source metadata")
	}
	var p igesLensProvenance
	dec := json.NewDecoder(bytes.NewBufferString(raw.Text))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&p); e != nil {
		return e
	}
	if dec.Decode(new(any)) != io.EOF {
		return geom.ErrInvalid
	}
	if len(p.Faces) != 2 || len(p.ModelCurves) != 4 || len(p.Cells) != 2 {
		return geom.ErrInvalid
	}
	for _, row := range p.Cells {
		if len(row) != 4 {
			return geom.ErrInvalid
		}
		for _, cell := range row {
			if !cell.Valid() {
				return geom.ErrInvalid
			}
		}
	}
	seen := map[int]bool{}
	ids := []int{p.Solid, p.Shell, p.EdgeList, p.VertexList}
	ids = append(ids, p.Faces...)
	ids = append(ids, p.ModelCurves...)
	if len(p.ParameterCurves) > 0 || rational {
		if len(p.ParameterCurves) != 2 {
			return geom.ErrInvalid
		}
		for _, row := range p.ParameterCurves {
			if len(row) != 4 {
				return geom.ErrInvalid
			}
			ids = append(ids, row...)
		}
	}
	for _, id := range ids {
		if id <= 0 || id%2 != 1 || seen[id] {
			return geom.ErrInvalid
		}
		seen[id] = true
	}
	source := label.Attributes["iges.source.solid"]
	if source.Kind != "integer" || source.Integer != int64(p.Solid) {
		return geom.ErrInvalid
	}
	digest, e := lensGeometryDigest(solid)
	if e != nil {
		return e
	}
	if digest != p.Geometry {
		return fmt.Errorf("document: IGES lens geometry changed")
	}
	recovered, e := step.AffineSplineLensFromSolid(solid, p.Center, p.Radii, p.Shear, "", s.LengthUnit, step.CurvedOvalOptions{})
	if e != nil {
		return e
	}
	m := iges.SplineLensSolid{Upper: recovered.Upper, Lower: recovered.Lower, Center: p.Center, Radii: p.Radii, Shear: p.Shear, LengthUnit: s.LengthUnit}
	var b bytes.Buffer
	var verified iges.SplineLensSolid
	if rational {
		recipe, e := readIGESLensRecipe(label, p, raw.Text)
		if e != nil {
			return e
		}
		value := iges.RationalSplineLens{Body: m}
		for i, row := range recipe.Curves {
			copy(value.Curves[i][:], row)
		}
		if e = iges.WriteRationalSplineLensSolid(&b, value); e != nil {
			return e
		}
		proved, e := iges.ReadRationalSplineLensSolid(bytes.NewReader(b.Bytes()), iges.Limits{})
		if e != nil {
			return e
		}
		verified = proved.Body
	} else {
		if e = iges.WriteSplineLensSolid(&b, m); e != nil {
			return e
		}
		verified, e = iges.ReadSplineLensSolid(bytes.NewReader(b.Bytes()), iges.Limits{})
		if e != nil {
			return e
		}
	}
	for f, row := range p.Cells {
		if !reflect.DeepEqual(row, verified.SourceKnotCells[f][:]) {
			return fmt.Errorf("document: stale IGES lens knot ancestry")
		}
	}
	n, e := w.Write(b.Bytes())
	if e == nil && n != b.Len() {
		return io.ErrShortWrite
	}
	return e
}
