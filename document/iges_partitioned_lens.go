package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

const maxPartitionedIGESRecipeBytes = 128 << 10

type partitionedIGESPiece struct {
	Quarter           int
	Start, End        *big.Rat
	Cells             []geom.ParameterRectangle
	ModelCurveID      int
	ParameterCurveIDs []int
}
type partitionedIGESRecipe struct {
	Solid, Shell, EdgeList, VertexList int
	Faces                              []int
	Geometry                           string
	Center, Radii                      geom.Vec2
	Shear                              float64
	Pieces                             []partitionedIGESPiece
	Curves                             [][]iges.RationalLensCurve
}

// ReadPartitionedSplineLensIGES retains exact source seam partitions and raw
// owner/model bases alongside canonical native material, without conflating
// source piece IDs with the native solid's four ellipse edges.
func ReadPartitionedSplineLensIGES(r io.Reader, l iges.Limits, historyLimit int) (*Document, error) {
	m, e := iges.ReadPartitionedRationalSplineLensSolid(r, l)
	if e != nil {
		return nil, e
	}
	d, e := FromTrimmedSTEP(step.TrimmedModel{LengthUnit: m.Body.LengthUnit, Bodies: []step.TrimmedBody{{Name: "Partitioned IGES lens", Solid: m.Body.Solid}}}, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	digest, e := lensGeometryDigest(m.Body.Solid)
	if e != nil {
		return nil, e
	}
	p := partitionedIGESRecipe{Solid: m.Body.SourceSolidID, Shell: m.Body.SourceShellID, EdgeList: m.Body.SourceEdgeListID, VertexList: m.Body.SourceVertexListID, Faces: append([]int(nil), m.Body.SourceFaceIDs[:]...), Geometry: digest, Center: m.Body.Center, Radii: m.Body.Radii, Shear: m.Body.Shear, Pieces: nil}
	for _, v := range m.Pieces {
		p.Pieces = append(p.Pieces, partitionedIGESPiece{Quarter: v.Quarter, Start: v.Start, End: v.End, Cells: append([]geom.ParameterRectangle(nil), v.Cells[:]...), ModelCurveID: v.ModelCurveID, ParameterCurveIDs: append([]int(nil), v.ParameterCurveIDs[:]...)})
	}
	for _, row := range m.Curves {
		p.Curves = append(p.Curves, append([]iges.RationalLensCurve(nil), row[:]...))
	}
	raw, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	if len(raw) > maxPartitionedIGESRecipeBytes {
		return nil, geom.ErrInvalid
	}
	s.Labels = map[string]Label{"iges:" + "body-000001": {Name: "Partitioned IGES lens", Target: AssetLabel, TargetID: "body-000001", Attributes: map[string]Attribute{"iges.lens.partitioned.recipe": {Kind: "string", Text: string(raw)}, "iges.lens.partitioned.binding": {Kind: "string", Text: lensProvenanceDigest(raw)}, "iges.source.solid": {Kind: "integer", Integer: int64(p.Solid)}}}}
	return New(s, historyLimit)
}

// FromPartitionedSplineLensIGES stages and re-proves supplied source geometry;
// its source IDs refer to that newly authored IGES graph.
func FromPartitionedSplineLensIGES(m iges.PartitionedRationalSplineLens, historyLimit int) (*Document, error) {
	var b bytes.Buffer
	if e := iges.WritePartitionedRationalSplineLensSolid(&b, m); e != nil {
		return nil, e
	}
	return ReadPartitionedSplineLensIGES(&b, iges.Limits{}, historyLimit)
}

// WritePartitionedSplineLensIGES re-derives the exact union of original chart
// knot cuts before export. Only one unchanged identity occurrence is supported.
func (s State) WritePartitionedSplineLensIGES(w io.Writer) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if e := s.Validate(); e != nil {
		return e
	}
	if len(s.Nodes) > 256 || len(s.Features) != 0 || len(s.Assets) != 0 || len(s.Planar) != 0 || len(s.Parametric) != 0 || len(s.Trimmed) != 1 {
		return fmt.Errorf("document: one partitioned IGES material required")
	}
	asset := ""
	for _, n := range s.Nodes {
		if n.Transform != geom.Identity() {
			return fmt.Errorf("document: partitioned IGES placement must be identity")
		}
		if n.Asset != "" {
			if asset != "" {
				return geom.ErrInvalid
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
	raw, binding := label.Attributes["iges.lens.partitioned.recipe"], label.Attributes["iges.lens.partitioned.binding"]
	if raw.Kind != "string" || binding.Kind != "string" || len(raw.Text) > maxPartitionedIGESRecipeBytes || binding.Text != lensProvenanceDigest([]byte(raw.Text)) {
		return fmt.Errorf("document: stale partitioned IGES recipe")
	}
	var p partitionedIGESRecipe
	dec := json.NewDecoder(bytes.NewBufferString(raw.Text))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&p); e != nil {
		return e
	}
	if dec.Decode(new(any)) != io.EOF {
		return geom.ErrInvalid
	}
	n := len(p.Pieces)
	if n < 4 || n > 32 || len(p.Curves) != n || len(p.Faces) != 2 {
		return geom.ErrInvalid
	}
	seen := map[int]bool{}
	ids := []int{p.Solid, p.Shell, p.EdgeList, p.VertexList}
	ids = append(ids, p.Faces...)
	for i, piece := range p.Pieces {
		if len(piece.Cells) != 2 || len(piece.ParameterCurveIDs) != 2 {
			return geom.ErrInvalid
		}
		if piece.Quarter < 0 || piece.Quarter > 3 || piece.Start == nil || piece.End == nil || piece.Start.Sign() < 0 || piece.Start.Cmp(piece.End) >= 0 || piece.End.Cmp(big.NewRat(1, 1)) > 0 {
			return geom.ErrInvalid
		}
		for _, v := range []*big.Rat{piece.Start, piece.End} {
			if v.Num().BitLen() > 8192 || v.Denom().BitLen() > 8192 {
				return geom.ErrInvalid
			}
		}
		for _, cell := range piece.Cells {
			if !cell.Valid() {
				return geom.ErrInvalid
			}
		}
		ids = append(ids, piece.ModelCurveID, piece.ParameterCurveIDs[0], piece.ParameterCurveIDs[1])
		row := p.Curves[i]
		if len(row) != 3 {
			return geom.ErrInvalid
		}
		for _, c := range row {
			if c.Degree < 1 || c.Degree > 16 || len(c.Poles) < 2 || len(c.Poles) > 64 || len(c.Weights) != len(c.Poles) || len(c.Knots) != len(c.Poles)+c.Degree+1 {
				return geom.ErrInvalid
			}
		}
	}
	for _, id := range ids {
		if id <= 0 || id > 2*(10+3*n)-1 || id%2 != 1 || seen[id] {
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
		return fmt.Errorf("document: partitioned IGES material changed")
	}
	native, e := step.AffineSplineLensFromSolid(solid, p.Center, p.Radii, p.Shear, "", s.LengthUnit, step.CurvedOvalOptions{})
	if e != nil {
		return e
	}
	m := iges.PartitionedRationalSplineLens{Body: iges.SplineLensSolid{Upper: native.Upper, Lower: native.Lower, Center: p.Center, Radii: p.Radii, Shear: p.Shear, LengthUnit: s.LengthUnit}, Pieces: make([]iges.LensSeamPiece, n), Curves: make([][3]iges.RationalLensCurve, n)}
	for i, row := range p.Curves {
		v := p.Pieces[i]
		m.Pieces[i] = iges.LensSeamPiece{Quarter: v.Quarter, Start: v.Start, End: v.End, ModelCurveID: v.ModelCurveID}
		copy(m.Pieces[i].Cells[:], v.Cells)
		copy(m.Pieces[i].ParameterCurveIDs[:], v.ParameterCurveIDs)
		copy(m.Curves[i][:], row)
	}
	return iges.WritePartitionedRationalSplineLensSolid(w, m)
}
