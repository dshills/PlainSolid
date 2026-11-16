package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetAtlasProvesConnectedCurvedSheet(t *testing.T) {
	ctx := context.Background()
	faces := []VerifiedNormalOffsetFace{
		offsetAtlasFace(t, -1, 0, -1, 1, false, true),
		offsetAtlasFace(t, 0, 2, -1, 1, true, true),
		offsetAtlasFace(t, 2, 3, -1, 1, false, true),
	}
	joins := []VerifiedNormalOffsetJoin{
		{Faces: [2]int{0, 1}, Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, ReverseB: true},
		{Faces: [2]int{2, 1}, Edges: [2]SurfaceEdge{SurfaceEdgeUMin, SurfaceEdgeUMax}, ReverseB: true},
	}
	got, err := CertifyVerifiedNormalOffsetAtlas(ctx, faces, joins, VerifiedNormalOffsetAtlasOptions{})
	if err != nil || !got.Complete || got.Closed || got.BoundaryEdges != 8 ||
		got.ProvenJoins != 2 || got.SeparatedPairs != 1 || len(got.UnresolvedPairs) != 0 {
		t.Fatal("curved atlas did not certify both seams and the non-neighbor pair", got, err)
	}
	short, err := CertifyVerifiedNormalOffsetAtlas(ctx, faces, joins, VerifiedNormalOffsetAtlasOptions{
		MaxOperations: got.ExactOperations - 1,
	})
	if !errors.Is(err, ErrConvergence) || short.Complete || len(short.UnresolvedPairs) != 1 ||
		short.UnresolvedPairs[0] != [2]int{0, 2} {
		t.Fatal("shared proof limit did not retain the unqualified pair", short, err)
	}
	changed := append([]VerifiedNormalOffsetJoin(nil), joins...)
	changed[0].ReverseB = false
	if invalid, err := CertifyVerifiedNormalOffsetAtlas(ctx, faces, changed, VerifiedNormalOffsetAtlasOptions{}); !errors.Is(err, ErrInvalid) || invalid.Complete {
		t.Fatal("an incompatible oriented coedge pairing passed", invalid, err)
	}
	if invalid, err := CertifyVerifiedNormalOffsetAtlas(ctx, faces, joins[:1], VerifiedNormalOffsetAtlasOptions{}); !errors.Is(err, ErrInvalid) || invalid.Complete {
		t.Fatal("disconnected faces passed the connected atlas gate", invalid, err)
	}
}

func TestVerifiedNormalOffsetAtlasRetainsAnUnregisteredCornerContact(t *testing.T) {
	faces := []VerifiedNormalOffsetFace{
		offsetAtlasFace(t, -1, 0, 0, 1, false, false),
		offsetAtlasFace(t, 0, 1, 0, 1, false, false),
		offsetAtlasFace(t, 0, 1, 1, 2, false, false),
	}
	joins := []VerifiedNormalOffsetJoin{
		{Faces: [2]int{0, 1}, Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}},
		{Faces: [2]int{1, 2}, Edges: [2]SurfaceEdge{SurfaceEdgeVMax, SurfaceEdgeVMin}},
	}
	// The two declared line joins are good. The other two faces also
	// meet at (0,1,d), which must not be silently treated as disjoint.
	got, err := CertifyVerifiedNormalOffsetAtlas(context.Background(), faces, joins, VerifiedNormalOffsetAtlasOptions{
		Pair: VerifiedNormalOffsetSeparationOptions{MaxDepth: 4},
	})
	if !errors.Is(err, ErrConvergence) || got.Complete || got.ProvenJoins != 2 ||
		len(got.UnresolvedPairs) != 1 || got.UnresolvedPairs[0] != [2]int{0, 2} {
		t.Fatal("extra geometric corner contact was silently discarded", got, err)
	}
}

func offsetAtlasFace(t *testing.T, x0, x1, y0, y1 int64, reversed, curved bool) VerifiedNormalOffsetFace {
	t.Helper()
	var h [][4]*big.Rat
	for _, x := range []int64{x0, x1} {
		for i := range 3 {
			j := i
			if reversed {
				j = 2 - i
			}
			y, z := big.NewRat(2*y0+int64(j)*(y1-y0), 2), new(big.Rat)
			if curved {
				z.SetInt64([]int64{y0 * y0, y0 * y1, y1 * y1}[j])
			}
			h = append(h, [4]*big.Rat{big.NewRat(x, 1), y, z, big.NewRat(1, 1)})
		}
	}
	ctx := context.Background()
	p, err := NewExactHomogeneousBezierSurface(ctx, 1, 2, h, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sign := int64(1)
	if reversed {
		sign = -1
	}
	o, _, err := NewVerifiedNormalOffsetPatch(ctx, p, big.NewRat(sign, 10),
		NormalOffsetProjection{Axes: [2]int{0, 1}, Reversed: [2]bool{false, reversed}}, NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return VerifiedNormalOffsetFace{o, reversed}
}
