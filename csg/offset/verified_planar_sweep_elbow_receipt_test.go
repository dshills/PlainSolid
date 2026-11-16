package offset

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestVerifiedPlanarSweepElbowBRepReceipt(t *testing.T) {
	ctx := context.Background()
	for _, control := range []struct {
		name  string
		shear bool
	}{
		{"perpendicular", false},
		{"oblique", true},
	} {
		t.Run(control.name, func(t *testing.T) {
			result := authoredElbowBRepReceiptFixture(t, control.shear)
			data := planarElbowReceiptBytes(t, result)
			var indented bytes.Buffer
			if err := json.Indent(&indented, data, "", "  "); err != nil {
				t.Fatal(err)
			}
			reopened, err := ReadVerifiedPlanarSweepElbowBRepReceipt(ctx, &indented, VerifiedSweepOpenChainBRepReceiptOptions{})
			if err != nil {
				t.Fatal(err)
			}
			original, replayed := result.Original(), reopened.Original()
			if original == replayed || !reflect.DeepEqual(original.Joins(), replayed.Joins()) ||
				!reflect.DeepEqual(original.Contact(), replayed.Contact()) ||
				reopened.ReentrantEdge() != result.ReentrantEdge() || reopened.ExactVolume().Cmp(big.NewRat(9, 2)) != 0 {
				t.Fatal("fresh replay changed the original joins, reentrant edge or exact volume")
			}
			checkPlanarSweepElbowBRep(t, replayed, reopened, control.shear)
			if control.shear {
				sweeps := replayed.Sweeps()
				raw := [2][2]*big.Rat{
					{big.NewRat(-27, 4), big.NewRat(7, 1)},
					{big.NewRat(10, 1), big.NewRat(-25, 3)},
				}
				world := [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 1), big.NewRat(1, 17)}
				for i, uv := range raw {
					p, err := sweeps[i+1].EvaluateRational(ctx, uv[0], uv[1], world[2],
						geom.SurfacePatchEvaluationOptions{}, geom.SurfaceNormalOffsetEvaluationOptions{})
					if err != nil {
						t.Fatal(err)
					}
					for axis, expected := range world {
						if p.Lower[axis].Cmp(expected) != 0 || p.Upper[axis].Cmp(expected) != 0 {
							t.Fatal("the original nonbinary Möbius join station failed exact replay", i, p)
						}
					}
				}
			}
			if !bytes.Equal(planarElbowReceiptBytes(t, reopened), data) {
				t.Fatal("replayed source or native claims changed canonical bytes")
			}
			t.Logf("%d bytes, three original charts, two full wall joins, 16V/28E/14F, exact edge %d and volume %s",
				len(data), reopened.ReentrantEdge(), reopened.ExactVolume())
		})
	}
}

func TestVerifiedPlanarSweepElbowBRepReceiptRefusesChangedEvidence(t *testing.T) {
	ctx := context.Background()
	result := authoredElbowBRepReceiptFixture(t, true)
	good := planarElbowReceiptBytes(t, result)
	refuse := func(t *testing.T, raw []byte, o VerifiedSweepOpenChainBRepReceiptOptions, want error) {
		t.Helper()
		got, err := ReadVerifiedPlanarSweepElbowBRepReceipt(ctx, bytes.NewReader(raw), o)
		if err == nil || (want != nil && !errors.Is(err, want)) || got.Original() != nil || got.ExactVolume() != nil ||
			got.ReentrantEdge() != -1 || len(got.Solid().Faces()) != 0 || len(got.FaceSources()) != 0 {
			t.Fatal("changed receipt published an elbow", err)
		}
	}
	for _, control := range []struct {
		name   string
		change func(*planarSweepElbowReceipt)
		want   error
	}{
		{"changed contact owner corner", func(r *planarSweepElbowReceipt) { r.Contact.Corners[0][1] = 0 }, geom.ErrDomain},
		{"changed swept edge endpoint", func(r *planarSweepElbowReceipt) { r.Contact.Endpoints[1][2] = "1/2" }, geom.ErrDomain},
		{"changed reentrant edge index", func(r *planarSweepElbowReceipt) { r.Contact.NativeEdge++ }, geom.ErrDomain},
		{"changed original face owner", func(r *planarSweepElbowReceipt) { r.Boundary.FaceSources[0].Owner++ }, geom.ErrDomain},
		{"lost triple owner", func(r *planarSweepElbowReceipt) {
			for i, owners := range r.Boundary.VertexSources {
				if len(owners) == 3 {
					r.Boundary.VertexSources[i] = owners[:2]
					return
				}
			}
		}, geom.ErrDomain},
		{"lost complete join", func(r *planarSweepElbowReceipt) { r.Joins = r.Joins[:1] }, geom.ErrInvalid},
		{"changed exact station clock", func(r *planarSweepElbowReceipt) { r.Joins[1].BStationRatio = "7/5" }, nil},
	} {
		t.Run(control.name, func(t *testing.T) {
			refuse(t, changedElbowReceipt(t, good, control.change), VerifiedSweepOpenChainBRepReceiptOptions{}, control.want)
		})
	}
	for _, control := range []struct {
		name string
		high int64
	}{
		{"valid pair joins with end volume overlap", 5},
		{"valid pair joins with extra swept face contact", 3},
	} {
		t.Run(control.name, func(t *testing.T) {
			pairs := authoredPlanarSweepElbow(t, false, control.high, -3)
			ends := [2]*geom.VerifiedNormalOffsetSweep{pairs[0].Sweeps()[0], pairs[1].Sweeps()[1]}
			base := planarElbowReceiptBytes(t, authoredElbowBRepReceiptFixture(t, false))
			raw := changedElbowReceipt(t, base, func(r *planarSweepElbowReceipt) {
				for i, end := range ends {
					chart, _ := end.OriginalSource()
					source, err := chart.ExportExactBezierRecipe()
					if err != nil {
						t.Fatal(err)
					}
					r.Charts[2*i].Source = source
				}
			})
			refuse(t, raw, VerifiedSweepOpenChainBRepReceiptOptions{}, geom.ErrDomain)
		})
	}
	t.Run("short source work", func(t *testing.T) {
		o := VerifiedSweepOpenChainBRepReceiptOptions{Coverage: geom.NormalOffsetCoverageOptions{Patch: geom.SurfacePatchOptions{MaxOperations: 1}}}
		refuse(t, good, o, geom.ErrConvergence)
	})
	t.Run("short native collision work", func(t *testing.T) {
		o := VerifiedSweepOpenChainBRepReceiptOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1}}
		refuse(t, good, o, collision.ErrLimit)
	})
	t.Run("strict open chain does not read elbow receipt", func(t *testing.T) {
		got, err := ReadVerifiedSweepOpenChainBRepReceipt(ctx, bytes.NewReader(good), VerifiedSweepOpenChainBRepReceiptOptions{})
		if err == nil || got.Original() != nil || len(got.Solid().Faces()) != 0 {
			t.Fatal("strict chain reader accepted the contacting elbow", err)
		}
	})
}

func authoredElbowBRepReceiptFixture(t *testing.T, shear bool) VerifiedPlanarSweepElbowBRep {
	t.Helper()
	ctx := context.Background()
	pairs := authoredPlanarSweepElbow(t, shear, 1, 0)
	elbow, _, err := geom.NewVerifiedPlanarNormalOffsetSweepElbow(ctx, pairs[0], pairs[1], geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := PlanarBRepFromVerifiedPlanarNormalOffsetSweepElbow(ctx, elbow, VerifiedSweepOpenChainBRepOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func planarElbowReceiptBytes(t *testing.T, r VerifiedPlanarSweepElbowBRep) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := WriteVerifiedPlanarSweepElbowBRepReceipt(context.Background(), &out, r); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func changedElbowReceipt(t *testing.T, raw []byte, change func(*planarSweepElbowReceipt)) []byte {
	t.Helper()
	var receipt planarSweepElbowReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	change(&receipt)
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
