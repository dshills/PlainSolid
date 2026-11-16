package offset

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

const (
	sweepBRepReceiptKind       = "verified-planar-sweep-open-chain"
	sweepBRepReceiptMaxBytes   = 1 << 20
	sweepBRepReceiptMaxRatText = 16387
)

// VerifiedSweepOpenChainBRepReceiptOptions bounds each original sweep
// and join proof, the shared nonadjacent chain proof and native validation.
// The coverage patch limits also bound exact chart import and planar promotion.
type VerifiedSweepOpenChainBRepReceiptOptions struct {
	Coverage   geom.NormalOffsetCoverageOptions
	Validation collision.LocationOptions
}

type sweepBRepReceiptChart struct {
	Source       geom.ExactBezierSurfaceRecipe    `json:"source"`
	Displacement [2]string                        `json:"displacement"`
	Projection   geom.NormalOffsetSweepProjection `json:"projection"`
}

type sweepBRepReceiptJoin struct {
	Owners        [2]int              `json:"owners"`
	Edges         [2]geom.SurfaceEdge `json:"edges"`
	ReverseB      bool                `json:"reverse_b"`
	BStationRatio string              `json:"b_station_ratio,omitempty"`
	Axis          int                 `json:"axis"`
	AInteriorSign int                 `json:"a_interior_sign"`
	Covector      *[3]string          `json:"covector,omitempty"`
	Coordinate    string              `json:"coordinate"`
}

type sweepBRepReceiptSeparator struct {
	Owners     [2]int    `json:"owners"`
	Covector   [3]string `json:"covector"`
	Coordinate string    `json:"coordinate"`
}

type sweepBRepReceiptBoundary struct {
	Vertices      []geom.Vec3               `json:"vertices"`
	Edges         []brep.Edge               `json:"edges"`
	Faces         []brep.Face               `json:"faces"`
	FaceSources   []SweepBRepFaceSource     `json:"face_sources"`
	VertexSources [][]SweepBRepVertexSource `json:"vertex_sources"`
	ExactVolume   string                    `json:"exact_volume"`
}

type sweepBRepReceipt struct {
	Kind       string                      `json:"kind"`
	Version    int                         `json:"version"`
	Charts     []sweepBRepReceiptChart     `json:"charts"`
	Joins      []sweepBRepReceiptJoin      `json:"joins"`
	Separators []sweepBRepReceiptSeparator `json:"separators"`
	Boundary   sweepBRepReceiptBoundary    `json:"boundary"`
}

// WriteVerifiedSweepOpenChainBRepReceipt writes the canonical v1 exact
// source recipe, with the native index graph and original ancestry as
// claims that will be checked on reload.
func WriteVerifiedSweepOpenChainBRepReceipt(ctx context.Context, w io.Writer, result VerifiedSweepOpenChainBRep) error {
	if ctx == nil || w == nil || result.original == nil || result.volume == nil {
		return geom.ErrInvalid
	}
	raw, err := encodeSweepBRepReceipt(ctx, result)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

// ReadVerifiedSweepOpenChainBRepReceipt replays the exact original
// source, adjacent joins and every independent nonadjacent separator.
// It rebuilds the planar B-rep and checks the complete stored native
// topology, volume and source-index claims. V1 accepts the writer's
// canonical form with optional JSON whitespace. Failure is atomic.
func ReadVerifiedSweepOpenChainBRepReceipt(ctx context.Context, r io.Reader,
	o VerifiedSweepOpenChainBRepReceiptOptions) (VerifiedSweepOpenChainBRep, error) {
	var empty VerifiedSweepOpenChainBRep
	if ctx == nil || r == nil {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, sweepBRepReceiptMaxBytes+1))
	if err != nil {
		return empty, err
	}
	if len(raw) > sweepBRepReceiptMaxBytes {
		return empty, ErrLimit
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return empty, fmt.Errorf("offset: invalid sweep B-rep receipt: %w", err)
	}
	var receipt sweepBRepReceipt
	decoder := json.NewDecoder(bytes.NewReader(compact.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return empty, fmt.Errorf("offset: invalid sweep B-rep receipt: %w", err)
	}
	n := len(receipt.Charts)
	b := receipt.Boundary
	if receipt.Kind != sweepBRepReceiptKind || receipt.Version != 1 || n < 2 || n > 8 ||
		len(receipt.Joins) != n-1 || len(receipt.Separators) != (n-1)*(n-2)/2 ||
		len(b.Vertices) != 4*(n+1) || len(b.VertexSources) != len(b.Vertices) ||
		len(b.Edges) != 8*n+4 || len(b.Faces) != 4*n+2 || len(b.FaceSources) != len(b.Faces) {
		return empty, geom.ErrInvalid
	}
	chain, err := receipt.reprove(ctx, o.Coverage)
	if err != nil {
		return empty, err
	}
	result, err := PlanarBRepFromVerifiedNormalOffsetSweepOpenChain(ctx, chain,
		VerifiedSweepOpenChainBRepOptions{Source: o.Coverage.Patch, Validation: o.Validation})
	if err != nil {
		return empty, err
	}
	canonical, err := encodeSweepBRepReceipt(ctx, result)
	if err != nil {
		return empty, err
	}
	if !bytes.Equal(bytes.TrimSpace(canonical), compact.Bytes()) {
		return empty, fmt.Errorf("offset: sweep B-rep receipt differs from exact replay: %w", geom.ErrDomain)
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	return result, nil
}

func encodeSweepBRepReceipt(ctx context.Context, result VerifiedSweepOpenChainBRep) ([]byte, error) {
	tooLong := false
	ratText := func(q *big.Rat) string {
		text := q.RatString()
		tooLong = tooLong || len(text) > sweepBRepReceiptMaxRatText
		return text
	}
	chain := result.original
	sweeps, joins, separators := chain.Sweeps(), chain.Joins(), chain.NonadjacentSeparators()
	receipt := sweepBRepReceipt{Kind: sweepBRepReceiptKind, Version: 1,
		Charts: make([]sweepBRepReceiptChart, len(sweeps)), Joins: make([]sweepBRepReceiptJoin, len(joins)),
		Separators: make([]sweepBRepReceiptSeparator, len(separators)),
		Boundary: sweepBRepReceiptBoundary{
			Vertices: result.solid.Vertices(), Edges: result.solid.Edges(), Faces: result.solid.Faces(),
			FaceSources: result.faceSource, VertexSources: result.vertices, ExactVolume: ratText(result.volume),
		},
	}
	for i, sweep := range sweeps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		chart, projection := sweep.OriginalSource()
		source, err := chart.ExportExactBezierRecipe()
		if err != nil {
			return nil, err
		}
		_, layer := sweep.Domain()
		receipt.Charts[i] = sweepBRepReceiptChart{
			Source: source, Displacement: [2]string{ratText(layer.Lower), ratText(layer.Upper)}, Projection: projection,
		}
	}
	for i, joined := range joins {
		j := joined.Original
		r := sweepBRepReceiptJoin{
			Owners: joined.Owners, Edges: j.Edges, ReverseB: j.ReverseB, Axis: j.Axis,
			AInteriorSign: j.AInteriorSign, Coordinate: ratText(joined.Coordinate),
		}
		if j.BStationRatio != nil {
			r.BStationRatio = ratText(j.BStationRatio)
		}
		if j.Covector != nil {
			r.Covector = new([3]string)
			for axis, q := range j.Covector {
				if q != nil {
					r.Covector[axis] = ratText(q)
				}
			}
		}
		receipt.Joins[i] = r
	}
	for i, entry := range separators {
		s := entry.Separator
		receipt.Separators[i] = sweepBRepReceiptSeparator{
			Owners: entry.Owners, Coordinate: ratText(s.Coordinate),
			Covector: [3]string{ratText(s.Covector[0]), ratText(s.Covector[1]), ratText(s.Covector[2])},
		}
	}
	if tooLong {
		return nil, ErrLimit
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	if len(raw)+1 > sweepBRepReceiptMaxBytes {
		return nil, ErrLimit
	}
	return append(raw, '\n'), nil
}

func (r sweepBRepReceipt) reprove(ctx context.Context, o geom.NormalOffsetCoverageOptions) (*geom.VerifiedNormalOffsetSweepOpenChain, error) {
	sweeps := make([]*geom.VerifiedNormalOffsetSweep, len(r.Charts))
	for i, c := range r.Charts {
		if c.Source.DegreeU != 1 || c.Source.DegreeV != 1 {
			return nil, geom.ErrDomain
		}
		chart, err := geom.ImportExactBezierSurfaceRecipe(ctx, c.Source, o.Patch)
		if err != nil {
			return nil, fmt.Errorf("offset: sweep receipt original chart %d: %w", i, err)
		}
		d, err := sweepBRepReceiptRationals(c.Displacement[:], false)
		if err != nil {
			return nil, err
		}
		sweeps[i], _, err = geom.NewVerifiedNormalOffsetSweep(ctx, chart, d[0], d[1], c.Projection, o)
		if err != nil {
			return nil, fmt.Errorf("offset: sweep receipt original sweep %d: %w", i, err)
		}
	}
	pairs := make([]*geom.VerifiedNormalOffsetSweepPair, len(r.Joins))
	for i, r := range r.Joins {
		if r.Owners != [2]int{i, i + 1} {
			return nil, geom.ErrInvalid
		}
		j := geom.VerifiedNormalOffsetSweepJoin{
			Edges: r.Edges, ReverseB: r.ReverseB, Axis: r.Axis, AInteriorSign: r.AInteriorSign,
		}
		ratio, err := sweepBRepReceiptRationals([]string{r.BStationRatio}, true)
		if err != nil {
			return nil, err
		}
		j.BStationRatio = ratio[0]
		if r.Covector != nil {
			q, err := sweepBRepReceiptRationals(r.Covector[:], true)
			if err != nil {
				return nil, err
			}
			j.Covector = (*[3]*big.Rat)(q)
		}
		pairs[i], _, err = geom.NewVerifiedNormalOffsetSweepPair(ctx, sweeps[i], sweeps[i+1], j, o)
		if err != nil {
			return nil, fmt.Errorf("offset: sweep receipt original join %d: %w", i, err)
		}
	}
	separators := make([]geom.VerifiedNormalOffsetSweepNonadjacentSeparator, len(r.Separators))
	for i, s := range r.Separators {
		q, err := sweepBRepReceiptRationals([]string{s.Covector[0], s.Covector[1], s.Covector[2], s.Coordinate}, false)
		if err != nil {
			return nil, err
		}
		separators[i] = geom.VerifiedNormalOffsetSweepNonadjacentSeparator{
			Owners: s.Owners, Separator: geom.VerifiedNormalOffsetSweepChainSeparator{
				Covector: [3]*big.Rat{q[0], q[1], q[2]}, Coordinate: q[3],
			},
		}
	}
	chain, _, err := geom.NewVerifiedNormalOffsetSweepOpenChain(ctx, pairs, separators, o)
	return chain, err
}

func sweepBRepReceiptRationals(texts []string, optional bool) ([]*big.Rat, error) {
	out := make([]*big.Rat, len(texts))
	for i, text := range texts {
		if text == "" && optional {
			continue
		}
		if len(text) > sweepBRepReceiptMaxRatText {
			return nil, ErrLimit
		}
		for _, ch := range text {
			if ch != '-' && ch != '/' && (ch < '0' || ch > '9') {
				return nil, geom.ErrInvalid
			}
		}
		q, ok := new(big.Rat).SetString(text)
		if !ok || q.RatString() != text {
			return nil, geom.ErrInvalid
		}
		out[i] = q
	}
	return out, nil
}
