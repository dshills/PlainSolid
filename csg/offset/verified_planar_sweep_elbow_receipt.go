package offset

import (
	"context"
	"io"

	"plainsolid/geom"
)

const planarSweepElbowReceiptKind = "verified-planar-sweep-elbow"

type planarSweepElbowReceiptContact struct {
	Corners    [3][2]int    `json:"corners"`
	Endpoints  [2][3]string `json:"endpoints"`
	NativeEdge int          `json:"native_edge"`
}

type planarSweepElbowReceipt struct {
	Kind     string                         `json:"kind"`
	Version  int                            `json:"version"`
	Charts   []sweepBRepReceiptChart        `json:"charts"`
	Joins    []sweepBRepReceiptJoin         `json:"joins"`
	Contact  planarSweepElbowReceiptContact `json:"contact"`
	Boundary sweepBRepReceiptBoundary       `json:"boundary"`
}

// WriteVerifiedPlanarSweepElbowBRepReceipt writes the canonical v1
// original source recipe and native claims. It records all three original
// contact corners, both exact world endpoints, and the native edge index.
func WriteVerifiedPlanarSweepElbowBRepReceipt(ctx context.Context, w io.Writer, result VerifiedPlanarSweepElbowBRep) error {
	if ctx == nil || w == nil || result.original == nil || result.volume == nil {
		return geom.ErrInvalid
	}
	raw, err := encodePlanarSweepElbowReceipt(ctx, result)
	if err != nil {
		return err
	}
	return writeSweepBRepReceiptBytes(ctx, w, raw)
}

// ReadVerifiedPlanarSweepElbowBRepReceipt reproves the original three
// charts, both full wall joins and the complete exact planar sectors.
// The rebuilt shared edge, native graph and owner ledger must match the
// canonical receipt. Optional JSON whitespace is accepted. Failure is atomic.
func ReadVerifiedPlanarSweepElbowBRepReceipt(ctx context.Context, r io.Reader,
	o VerifiedSweepOpenChainBRepReceiptOptions) (VerifiedPlanarSweepElbowBRep, error) {
	var empty VerifiedPlanarSweepElbowBRep
	var receipt planarSweepElbowReceipt
	compact, err := decodeSweepBRepReceipt(ctx, r, &receipt)
	if err != nil {
		return empty, err
	}
	if receipt.Kind != planarSweepElbowReceiptKind || receipt.Version != 1 ||
		len(receipt.Charts) != 3 || len(receipt.Joins) != 2 || !receipt.Boundary.hasCounts(3) {
		return empty, geom.ErrInvalid
	}
	pairs, err := reproveSweepBRepReceiptJoins(ctx, receipt.Charts, receipt.Joins, o.Coverage)
	if err != nil {
		return empty, err
	}
	elbow, _, err := geom.NewVerifiedPlanarNormalOffsetSweepElbow(ctx, pairs[0], pairs[1], o.Coverage.Patch)
	if err != nil {
		return empty, err
	}
	result, err := PlanarBRepFromVerifiedPlanarNormalOffsetSweepElbow(ctx, elbow,
		VerifiedSweepOpenChainBRepOptions{Source: o.Coverage.Patch, Validation: o.Validation})
	if err != nil {
		return empty, err
	}
	canonical, err := encodePlanarSweepElbowReceipt(ctx, result)
	if err != nil {
		return empty, err
	}
	if err := matchSweepBRepReceipt(ctx, canonical, compact); err != nil {
		return empty, err
	}
	return result, nil
}

func encodePlanarSweepElbowReceipt(ctx context.Context, result VerifiedPlanarSweepElbowBRep) ([]byte, error) {
	var encoder sweepBRepReceiptEncoder
	elbow := result.original
	sweeps, joins := elbow.Sweeps(), elbow.Joins()
	charts, walls, err := encoder.source(ctx, sweeps[:], joins[:])
	if err != nil {
		return nil, err
	}
	contact := elbow.Contact()
	edge := planarSweepElbowReceiptContact{Corners: contact.Corners, NativeEdge: result.reentrant}
	for i, point := range contact.Endpoints {
		for axis, q := range point {
			edge.Endpoints[i][axis] = encoder.ratText(q)
		}
	}
	return encoder.marshal(planarSweepElbowReceipt{
		Kind: planarSweepElbowReceiptKind, Version: 1, Charts: charts, Joins: walls, Contact: edge,
		Boundary: encoder.boundary(result.sweepPlanarBRepBoundary),
	})
}
