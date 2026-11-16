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

func TestVerifiedSweepOpenChainBRepReceipt(t *testing.T) {
	ctx := context.Background()
	for _, control := range []struct {
		name        string
		nonparallel bool
	}{
		{"affine", false},
		{"nonparallel", true},
	} {
		t.Run(control.name, func(t *testing.T) {
			chain, cuts := authoredSweepBRepSource(t, control.nonparallel, false, big.NewRat(1, 8))
			original, err := PlanarBRepFromVerifiedNormalOffsetSweepOpenChain(ctx, chain, VerifiedSweepOpenChainBRepOptions{})
			if err != nil {
				t.Fatal(err)
			}
			data := verifiedSweepBRepReceiptBytes(t, original)
			var indented bytes.Buffer
			if err := json.Indent(&indented, data, "", "  "); err != nil {
				t.Fatal(err)
			}
			reopened, err := ReadVerifiedSweepOpenChainBRepReceipt(ctx, &indented, VerifiedSweepOpenChainBRepReceiptOptions{})
			if err != nil {
				t.Fatal(err)
			}
			replayed := reopened.Original()
			if replayed == chain || !reflect.DeepEqual(replayed.Joins(), chain.Joins()) ||
				!reflect.DeepEqual(replayed.NonadjacentSeparators(), chain.NonadjacentSeparators()) {
				t.Fatal("a fresh replay changed the baked original joins and separators")
			}
			replayedSweeps := replayed.Sweeps()
			for i, sweep := range chain.Sweeps() {
				a, projectionA := sweep.OriginalSource()
				b, projectionB := replayedSweeps[i].OriginalSource()
				ra, ea := a.ExportExactBezierRecipe()
				rb, eb := b.ExportExactBezierRecipe()
				if a == b || projectionA != projectionB || ea != nil || eb != nil || !reflect.DeepEqual(ra, rb) {
					t.Fatal("replay changed an original control, raw domain or proof projection", i, ea, eb)
				}
			}
			if !reflect.DeepEqual(original.Solid().Vertices(), reopened.Solid().Vertices()) ||
				!reflect.DeepEqual(original.Solid().Edges(), reopened.Solid().Edges()) ||
				!reflect.DeepEqual(original.Solid().Faces(), reopened.Solid().Faces()) {
				t.Fatal("durable reproof changed indexed native ancestry")
			}
			checkOriginalSweepBRep(t, replayed, reopened, cuts)
			// The replay's immutable separator evidence is safe to export repeatedly.
			copyOfSeparators := replayed.NonadjacentSeparators()
			copyOfSeparators[0].Separator.Coordinate.SetInt64(-99)
			copyOfSeparators[0].Separator.Covector[0].SetInt64(0)
			if !bytes.Equal(verifiedSweepBRepReceiptBytes(t, reopened), data) {
				t.Fatal("source and native claims did not rewrite byte-for-byte")
			}
			t.Logf("%d receipt bytes, five charts, four joins, six separators, 22 recovered native face identities, volume %s",
				len(data), reopened.ExactVolume())
		})
	}
}

func TestVerifiedSweepOpenChainBRepReceiptTwoOwners(t *testing.T) {
	ctx := context.Background()
	chain, _ := authoredSweepBRepSource(t, false, false, big.NewRat(1, 8))
	sweeps, join := chain.Sweeps(), chain.Joins()[0].Original
	join.Axis, join.Covector, join.BStationRatio = 0, nil, nil
	pair, _, err := geom.NewVerifiedNormalOffsetSweepPair(ctx, sweeps[0], sweeps[1], join, geom.NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	two, _, err := geom.NewVerifiedNormalOffsetSweepOpenChain(ctx, []*geom.VerifiedNormalOffsetSweepPair{pair}, nil, geom.NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	original, err := PlanarBRepFromVerifiedNormalOffsetSweepOpenChain(ctx, two, VerifiedSweepOpenChainBRepOptions{})
	if err != nil {
		t.Fatal(err)
	}
	data := verifiedSweepBRepReceiptBytes(t, original)
	got, err := ReadVerifiedSweepOpenChainBRepReceipt(ctx, bytes.NewReader(data), VerifiedSweepOpenChainBRepReceiptOptions{})
	if err != nil || len(got.Original().NonadjacentSeparators()) != 0 || !reflect.DeepEqual(got.Original().Joins(), two.Joins()) ||
		len(got.FaceSources()) != 10 || got.ExactVolume().Cmp(big.NewRat(5, 8)) != 0 ||
		!bytes.Equal(verifiedSweepBRepReceiptBytes(t, got), data) {
		t.Fatal("two-owner axial receipt lost an implicit affine map or introduced a separator", err)
	}
}

func TestVerifiedSweepOpenChainBRepReceiptRefusesChangedEvidence(t *testing.T) {
	ctx := context.Background()
	chain, _ := authoredSweepBRepSource(t, true, false, big.NewRat(1, 8))
	original, err := PlanarBRepFromVerifiedNormalOffsetSweepOpenChain(ctx, chain, VerifiedSweepOpenChainBRepOptions{})
	if err != nil {
		t.Fatal(err)
	}
	good := verifiedSweepBRepReceiptBytes(t, original)
	refuse := func(t *testing.T, ctx context.Context, data []byte, o VerifiedSweepOpenChainBRepReceiptOptions, want error) {
		t.Helper()
		got, err := ReadVerifiedSweepOpenChainBRepReceipt(ctx, bytes.NewReader(data), o)
		if err == nil || (want != nil && !errors.Is(err, want)) || got.Original() != nil || got.ExactVolume() != nil ||
			len(got.FaceSources()) != 0 || len(got.VertexSources(0)) != 0 || len(got.Solid().Faces()) != 0 {
			t.Fatal("altered receipt published source ancestry or a native solid", err)
		}
	}
	for _, control := range []struct {
		name   string
		change func(*sweepBRepReceipt)
		want   error
	}{
		{"changed retained face owner", func(r *sweepBRepReceipt) { r.Boundary.FaceSources[0].Owner++ }, geom.ErrDomain},
		{"changed joined vertex owner", func(r *sweepBRepReceipt) { r.Boundary.VertexSources[4][1].Owner++ }, geom.ErrDomain},
		{"changed native edge index", func(r *sweepBRepReceipt) { r.Boundary.Edges[0].Start++ }, geom.ErrDomain},
		{"changed exact volume", func(r *sweepBRepReceipt) { r.Boundary.ExactVolume = "5/4" }, geom.ErrDomain},
		{"lost independent separator", func(r *sweepBRepReceipt) { r.Separators = r.Separators[:5] }, geom.ErrInvalid},
		{"separator assigned to wrong owner pair", func(r *sweepBRepReceipt) { r.Separators[0].Owners = r.Separators[1].Owners }, geom.ErrInvalid},
		{"false independent separator", func(r *sweepBRepReceipt) { r.Separators[0].Coordinate = "-99" }, geom.ErrDomain},
		{"changed join plane claim", func(r *sweepBRepReceipt) { r.Joins[0].Coordinate = "1/64" }, geom.ErrDomain},
		{"changed original Möbius ratio", func(r *sweepBRepReceipt) { r.Joins[0].BStationRatio = "17/11" }, nil},
		{"changed original source control", func(r *sweepBRepReceipt) { r.Charts[0].Source.Controls[0][0] = "-2" }, nil},
		{"exponent in an exact rational", func(r *sweepBRepReceipt) { r.Charts[0].Displacement[0] = "1e99999999" }, geom.ErrInvalid},
	} {
		t.Run(control.name, func(t *testing.T) {
			var r sweepBRepReceipt
			if err := json.Unmarshal(good, &r); err != nil {
				t.Fatal(err)
			}
			control.change(&r)
			changed, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			refuse(t, ctx, changed, VerifiedSweepOpenChainBRepReceiptOptions{}, control.want)
		})
	}
	t.Run("duplicate source attribute", func(t *testing.T) {
		duplicate := bytes.Replace(good, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
		refuse(t, ctx, duplicate, VerifiedSweepOpenChainBRepReceiptOptions{}, geom.ErrDomain)
	})
	t.Run("source budget", func(t *testing.T) {
		o := VerifiedSweepOpenChainBRepReceiptOptions{Coverage: geom.NormalOffsetCoverageOptions{
			Patch: geom.SurfacePatchOptions{MaxOperations: 1},
		}}
		refuse(t, ctx, good, o, geom.ErrConvergence)
	})
	t.Run("collision budget", func(t *testing.T) {
		o := VerifiedSweepOpenChainBRepReceiptOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1}}
		refuse(t, ctx, good, o, collision.ErrLimit)
	})
	t.Run("receipt byte limit", func(t *testing.T) {
		refuse(t, ctx, bytes.Repeat([]byte(" "), sweepBRepReceiptMaxBytes+1), VerifiedSweepOpenChainBRepReceiptOptions{}, ErrLimit)
	})
	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		refuse(t, ctx, good, VerifiedSweepOpenChainBRepReceiptOptions{}, context.Canceled)
	})
}

func verifiedSweepBRepReceiptBytes(t *testing.T, result VerifiedSweepOpenChainBRep) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := WriteVerifiedSweepOpenChainBRepReceipt(context.Background(), &out, result); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
