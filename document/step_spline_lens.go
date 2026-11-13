package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type lensProvenance struct {
	Body     int64
	Faces    [2]int64
	Edges    [4]int64
	Geometry string
}

func lensGeometryDigest(s trimmed.Solid) (string, error) {
	var b bytes.Buffer
	if e := trimmed.WriteJSON(&b, s); e != nil {
		return "", e
	}
	sum := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(sum[:]), nil
}
func lensProvenanceDigest(p []byte) string {
	sum := sha256.Sum256(p)
	return hex.EncodeToString(sum[:])
}

// ReadSplineLensSTEP imports one independently validated material body. Native
// labels preserve source roles; geometry remains a real trimmed asset.
func ReadSplineLensSTEP(r io.Reader, l step.Limits, o step.CurvedOvalOptions, historyLimit int) (*Document, error) {
	m, e := step.ReadSplineLensBody(r, l, o)
	if e != nil {
		return nil, e
	}
	return fromVerifiedSplineLens(m, historyLimit)
}

// FromSplineLensSTEP re-proves supplied geometry before creating the document.
// Source IDs refer to the validated staged STEP graph; use ReadSplineLensSTEP
// to retain IDs from an existing source file.
func FromSplineLensSTEP(m step.SplineLensBody, historyLimit int) (*Document, error) {
	var b bytes.Buffer
	if e := step.WriteSplineLensBody(&b, m, step.CurvedOvalOptions{}); e != nil {
		return nil, e
	}
	return ReadSplineLensSTEP(&b, step.Limits{}, step.CurvedOvalOptions{}, historyLimit)
}
func fromVerifiedSplineLens(m step.SplineLensBody, historyLimit int) (*Document, error) {
	d, e := FromTrimmedSTEP(step.TrimmedModel{LengthUnit: m.LengthUnit, Bodies: []step.TrimmedBody{{Name: m.Name, Solid: m.Solid}}}, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	digest, e := lensGeometryDigest(m.Solid)
	if e != nil {
		return nil, e
	}
	p, e := json.Marshal(lensProvenance{m.SourceBodyID, m.SourceFaceIDs, m.SourceEdges, digest})
	if e != nil {
		return nil, e
	}
	attributes := map[string]Attribute{"step.body.name": {Kind: "string", Text: m.Name}, "step.lens.upper.name": {Kind: "string", Text: m.FaceNames[0]}, "step.lens.lower.name": {Kind: "string", Text: m.FaceNames[1]}, "step.lens.center.x": {Kind: "number", Number: m.Center.X}, "step.lens.center.y": {Kind: "number", Number: m.Center.Y}, "step.lens.radius.x": {Kind: "number", Number: m.Radii.X}, "step.lens.radius.y": {Kind: "number", Number: m.Radii.Y}, "step.lens.shear": {Kind: "number", Number: m.Shear}, "step.lens.provenance": {Kind: "string", Text: string(p)}, "step.lens.binding": {Kind: "string", Text: lensProvenanceDigest(p)}}
	s.Labels = map[string]Label{"step:body-000001": {Name: m.Name, Target: AssetLabel, TargetID: "body-000001", Attributes: attributes}}
	return New(s, historyLimit)
}

// WriteSplineLensSTEP exports one identity-placed canonical asset. It rejects
// stale geometry/provenance and unsupported placement instead of flattening
// occurrences or changing the unit-XY material proof.
func (s State) WriteSplineLensSTEP(w io.Writer) error {
	return s.writeSplineLensSTEP(w, false)
}
func (s State) writeSplineLensSTEP(w io.Writer, partitioned bool, allowRefined ...bool) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := s.Validate(); e != nil {
		return e
	}
	if len(s.Nodes) > 256 || len(s.Features) != 0 || len(s.Assets) != 0 || len(s.Planar) != 0 || len(s.Parametric) != 0 || len(s.Trimmed) != 1 {
		return fmt.Errorf("document: one native spline lens asset required")
	}
	var asset, nodeID string
	for id, n := range s.Nodes {
		if n.Transform != geom.Identity() {
			return fmt.Errorf("document: lens placement must be identity")
		}
		if n.Asset != "" {
			if nodeID != "" {
				return fmt.Errorf("document: lens requires one occurrence")
			}
			nodeID, asset = id, n.Asset
		}
	}
	if nodeID == "" {
		return geom.ErrInvalid
	}
	solid, ok := s.Trimmed[asset]
	if !ok {
		return geom.ErrInvalid
	}
	label, ok := s.Labels["step:"+asset]
	if !ok || label.Target != AssetLabel || label.TargetID != asset {
		return fmt.Errorf("document: missing lens provenance")
	}
	_, hasNormalizedRecipe := label.Attributes["step.lens.curve.recipe"]
	_, hasRationalRecipe := label.Attributes["step.lens.rational.recipe"]
	if (hasNormalizedRecipe || hasRationalRecipe) && !(len(allowRefined) > 0 && allowRefined[0]) {
		return fmt.Errorf("document: refined source curves require explicit refined lens export")
	}
	getString := func(k string) (string, error) {
		a, ok := label.Attributes[k]
		if !ok || a.Kind != "string" {
			return "", geom.ErrInvalid
		}
		return a.Text, nil
	}
	provenanceKey, bindingKey := "step.lens.provenance", "step.lens.binding"
	if partitioned {
		provenanceKey, bindingKey = "step.lens.partitioned.provenance", "step.lens.partitioned.binding"
	}
	raw, e := getString(provenanceKey)
	if e != nil {
		return e
	}
	binding, e := getString(bindingKey)
	maxProvenanceBytes := 4096
	if partitioned {
		maxProvenanceBytes = 65536
	}
	if e != nil || len(raw) > maxProvenanceBytes || binding != lensProvenanceDigest([]byte(raw)) {
		return fmt.Errorf("document: stale lens source roles")
	}
	var p lensProvenance
	var partitionedProvenance partitionedLensProvenance
	var target any = &p
	if partitioned {
		target = &partitionedProvenance
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(target); e != nil {
		return e
	}
	if e = decoder.Decode(new(any)); e != io.EOF {
		return geom.ErrInvalid
	}
	var roles []int64
	if partitioned {
		p = lensProvenance{Body: partitionedProvenance.Body, Faces: partitionedProvenance.Faces, Geometry: partitionedProvenance.Geometry}
		if len(partitionedProvenance.Pieces) < 4 || len(partitionedProvenance.Pieces) > 128 {
			return geom.ErrInvalid
		}
		for _, piece := range partitionedProvenance.Pieces {
			roles = append(roles, piece.SourceEdgeID)
		}
	} else {
		roles = append(roles, p.Edges[:]...)
	}
	seen := map[int64]bool{}
	for _, id := range append(append([]int64{p.Body}, p.Faces[:]...), roles...) {
		if id <= 0 || seen[id] {
			return fmt.Errorf("document: invalid lens source roles")
		}
		seen[id] = true
	}
	if source, ok := label.Attributes["step.source.body.id"]; ok && (source.Kind != "integer" || source.Integer != p.Body) {
		return fmt.Errorf("document: stale lens source body")
	}
	digest, e := lensGeometryDigest(solid)
	if e != nil {
		return e
	}
	if digest != p.Geometry {
		return fmt.Errorf("document: lens geometry changed since import")
	}
	var values [4]float64
	for i, k := range []string{"step.lens.center.x", "step.lens.center.y", "step.lens.radius.x", "step.lens.radius.y"} {
		a, ok := label.Attributes[k]
		if !ok || a.Kind != "number" {
			return geom.ErrInvalid
		}
		values[i] = a.Number
	}
	name := s.Nodes[nodeID].Name
	if name == "" {
		name, e = getString("step.body.name")
		if e != nil {
			return e
		}
	}
	shear := 0.
	if a, ok := label.Attributes["step.lens.shear"]; ok {
		if a.Kind != "number" {
			return geom.ErrInvalid
		}
		shear = a.Number
	}
	m, e := step.AffineSplineLensFromSolid(solid, geom.Vec2{X: values[0], Y: values[1]}, geom.Vec2{X: values[2], Y: values[3]}, shear, name, s.LengthUnit, step.CurvedOvalOptions{})
	if e != nil {
		return e
	}
	for i, k := range []string{"step.lens.upper.name", "step.lens.lower.name"} {
		m.FaceNames[i], e = getString(k)
		if e != nil {
			return e
		}
	}
	if partitioned {
		return writePartitionedLensWithProvenance(w, m, partitionedProvenance)
	}
	return step.WriteSplineLensBody(w, m, step.CurvedOvalOptions{})
}
