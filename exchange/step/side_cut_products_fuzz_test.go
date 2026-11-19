package step

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
	"time"
)

// Resource bounds are deliberately smaller than the public defaults. Each fuzz
// input selects one actual typed reader and can perform only one accepted-output
// write and one readback. No retry, mesh, integration or external oracle is used.
const sideCutFuzzKinds = 6

var sideCutFuzzLimits = Limits{MaxBytes: 64 << 10, MaxEntities: 768, MaxValues: 8192, MaxDepth: 16, MaxVertices: 64, MaxFaces: 24, MaxSolids: 1, MaxEdgeUses: 256}

func sideCutFuzzSeed(t testing.TB, kind byte, through bool) []byte {
	t.Helper()
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	d := SideOpenDualDiskDescriptor{Name: "bounded side-cut seed", Frame: frame, Height: 6, Depth: 3, LengthUnit: .001, Tolerance: 1e-9}
	if through {
		d.Depth = 6
	}
	var join [2]string
	switch kind {
	case 0:
		d.Min = geom.Vec2{Y: 4}
		d.Max = geom.Vec2{X: 10, Y: 10}
		d.Discs = [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}}
	case 1:
		d.Max = geom.Vec2{X: 60, Y: 22}
		d.Discs = [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 34}, Radius: 15}}
	case 2:
		d.Max = geom.Vec2{X: 50, Y: 12}
		d.Discs = [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 5}, {Center: geom.Vec2{X: 24.5}, Radius: 8.5}}
	case 3:
		d.Max = geom.Vec2{X: 50, Y: 22}
		d.Discs = [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 24}, Radius: 15}}
	case 4:
		d.Max = geom.Vec2{X: 40, Y: 10}
		d.Discs = [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 12.5, Y: -7.5}, Radius: 65. / 8}, {Center: geom.Vec2{X: 22, Y: -7.5}, Radius: 87. / 8}}
	case 5:
		d.Max = geom.Vec2{X: 60, Y: 10}
		d.Discs = [2]trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 7, Y: -3}, Radius: 5}, {Center: geom.Vec2{X: 30, Y: -20}, Radius: 29}}
		join = [2]string{"10", "1"}
	default:
		t.Fatal("invalid seed kind")
	}
	var b bytes.Buffer
	if e = sideCutFuzzWrite(context.Background(), &b, kind, sideCutFuzzRecipe{d, join}); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}

type sideCutFuzzRecipe struct {
	Descriptor SideOpenDualDiskDescriptor
	Join       [2]string
}

func sideCutFuzzWrite(ctx context.Context, b *bytes.Buffer, kind byte, recipe sideCutFuzzRecipe) error {
	d := recipe.Descriptor
	switch kind {
	case 0:
		return WriteSideOpenDualDiskProduct(ctx, b, d)
	case 1:
		return WriteUnequalSideOpenDualDiskProduct(ctx, b, UnequalSideOpenDualDiskDescriptor(d))
	case 3:
		return WriteExactWallSideOpenDualDiskProduct(ctx, b, ExactWallSideOpenDualDiskDescriptor(d))
	case 4:
		return WriteOffsetSideOpenDualDiskProduct(ctx, b, OffsetSideOpenDualDiskDescriptor(d))
	case 5:
		var join [2]*big.Rat
		for i, text := range recipe.Join {
			q, ok := new(big.Rat).SetString(text)
			if !ok {
				return geom.ErrInvalid
			}
			join[i] = q
		}
		return WriteUnequalCenterSideOpenDualDiskProduct(ctx, b, UnequalCenterSideOpenDualDiskDescriptor{SideOpenDualDiskDescriptor: d, Join: join})
	default:
		return WriteGeneralSideOpenDualDiskProduct(ctx, b, GeneralSideOpenDualDiskDescriptor(d))
	}
}
func sideCutFuzzRead(t testing.TB, ctx context.Context, kind byte, raw []byte) (sideCutFuzzRecipe, error) {
	t.Helper()
	switch kind {
	case 0:
		r, e := ReadSideOpenDualDiskProduct(ctx, bytes.NewReader(raw), sideCutFuzzLimits, 1e-9)
		if e != nil {
			if r.Body != 0 || len(r.Faces) != 0 || len(r.Edges) != 0 {
				t.Fatal("partial equal result")
			}
			return sideCutFuzzRecipe{}, e
		}
		if r.Body <= 0 || r.Shell <= 0 || len(r.Native.Solid.Topology().Shells) != 1 {
			t.Fatal("incomplete equal result")
		}
		return sideCutFuzzRecipe{Descriptor: r.Descriptor}, nil
	case 1:
		r, e := ReadUnequalSideOpenDualDiskProduct(ctx, bytes.NewReader(raw), sideCutFuzzLimits, 1e-9)
		if e != nil {
			if r.Body != 0 || len(r.Faces) != 0 || len(r.Edges) != 0 {
				t.Fatal("partial unequal result")
			}
			return sideCutFuzzRecipe{}, e
		}
		if r.Body <= 0 || r.Shell <= 0 || len(r.Native.Solid.Topology().Shells) != 1 || r.SourceWallFaces[0] <= 0 || r.SourceWallFaces[1] <= 0 {
			t.Fatal("incomplete unequal result")
		}
		return sideCutFuzzRecipe{Descriptor: SideOpenDualDiskDescriptor(r.Descriptor)}, nil
	case 3:
		r, e := ReadExactWallSideOpenDualDiskProduct(ctx, bytes.NewReader(raw), sideCutFuzzLimits, 1e-9)
		if e != nil {
			if r.Body != 0 || len(r.Faces) != 0 || len(r.Edges) != 0 || r.RadicalJoin[0] != nil {
				t.Fatal("partial ExactWall result")
			}
			return sideCutFuzzRecipe{}, e
		}
		if r.Body <= 0 || r.Shell <= 0 || len(r.Native.Solid.Topology().Shells) != 1 || r.RadicalJoin[0] == nil || r.RadicalJoin[1] == nil || r.SourceWallFaces[0] <= 0 || r.SourceWallFaces[1] <= 0 {
			t.Fatal("incomplete ExactWall result")
		}
		return sideCutFuzzRecipe{Descriptor: SideOpenDualDiskDescriptor(r.Descriptor)}, nil
	case 4:
		r, e := ReadOffsetSideOpenDualDiskProduct(ctx, bytes.NewReader(raw), sideCutFuzzLimits, 1e-9)
		if e != nil {
			if r.Body != 0 || len(r.Faces) != 0 || len(r.Edges) != 0 || r.RadicalJoin[0] != nil {
				t.Fatal("partial Offset result")
			}
			return sideCutFuzzRecipe{}, e
		}
		if r.Body <= 0 || r.Shell <= 0 || len(r.Native.Solid.Topology().Shells) != 1 || r.RadicalJoin[0] == nil || r.RadicalJoin[1] == nil || r.SourceWallFaces[0] <= 0 || r.SourceWallFaces[1] <= 0 {
			t.Fatal("incomplete Offset result")
		}
		return sideCutFuzzRecipe{Descriptor: SideOpenDualDiskDescriptor(r.Descriptor)}, nil
	case 5:
		r, e := ReadUnequalCenterSideOpenDualDiskProduct(ctx, bytes.NewReader(raw), sideCutFuzzLimits, 1e-9)
		if e != nil {
			if r.Body != 0 || len(r.Faces) != 0 || len(r.Edges) != 0 || r.RadicalJoin[0] != nil {
				t.Fatal("partial different-center result")
			}
			return sideCutFuzzRecipe{}, e
		}
		if r.Body <= 0 || r.Shell <= 0 || len(r.Native.Solid.Topology().Shells) != 1 || r.RadicalJoin[0] == nil || r.RadicalJoin[1] == nil || r.SourceWallFaces[0] <= 0 || r.SourceWallFaces[1] <= 0 || r.SourceWallFaces[0] == r.SourceWallFaces[1] {
			t.Fatal("incomplete different-center result")
		}
		return sideCutFuzzRecipe{r.Descriptor.SideOpenDualDiskDescriptor, [2]string{r.RadicalJoin[0].RatString(), r.RadicalJoin[1].RatString()}}, nil
	default:
		r, e := ReadGeneralSideOpenDualDiskProduct(ctx, bytes.NewReader(raw), sideCutFuzzLimits, 1e-9)
		if e != nil {
			if r.Body != 0 || len(r.Faces) != 0 || len(r.Edges) != 0 || r.RadicalJoin[0] != nil {
				t.Fatal("partial general result")
			}
			return sideCutFuzzRecipe{}, e
		}
		if r.Body <= 0 || r.Shell <= 0 || len(r.Native.Solid.Topology().Shells) != 1 || r.RadicalJoin[0] == nil || r.RadicalJoin[1] == nil || (r.RadicalSide != ExternalRadicalLeft && r.RadicalSide != ExternalRadicalRight) {
			t.Fatal("incomplete general result")
		}
		return sideCutFuzzRecipe{Descriptor: SideOpenDualDiskDescriptor(r.Descriptor)}, nil
	}
}

func TestSideCutFuzzSeedsUnderWorkLimits(t *testing.T) {
	slowtest.Skip(t)
	for kind := byte(0); kind < sideCutFuzzKinds; kind++ {
		for _, through := range []bool{false, true} {
			raw := sideCutFuzzSeed(t, kind, through)
			if len(raw) > int(sideCutFuzzLimits.MaxBytes) {
				t.Fatal("seed byte limit")
			}
			if _, e := sideCutFuzzRead(t, context.Background(), kind, raw); e != nil {
				t.Fatal(kind, through, e)
			}
			for other := byte(0); other < sideCutFuzzKinds; other++ {
				if other != kind {
					if _, e := sideCutFuzzRead(t, context.Background(), other, raw); e == nil {
						t.Fatal("source category crossed", kind, other)
					}
				}
			}
		}
	}
}

func FuzzReadSideCutProducts(f *testing.F) {
	for kind := byte(0); kind < sideCutFuzzKinds; kind++ {
		for _, through := range []bool{false, true} {
			raw := sideCutFuzzSeed(f, kind, through)
			f.Add(kind, raw)
			for _, bad := range sideCutFuzzSemanticSeeds(f, raw) {
				f.Add(kind, bad)
			}
			f.Add((kind+1)%sideCutFuzzKinds, raw)
			f.Add(kind, raw[:len(raw)/2])
			f.Add(kind, append([]byte("unexpected-prefix\x00"), raw...))
			f.Add(kind, append(append([]byte(nil), raw...), []byte("#999999=CARTESIAN_POINT('',(0.,0.,0.));")...))
			f.Add(kind, bytes.Replace(raw, []byte("END-ISO-10303-21;"), []byte("END-ISO-10303-21;\x00"), 1))
		}
	}
	f.Add(byte(0), []byte{})
	f.Add(byte(1), []byte("ISO-10303-21;HEADER;ENDSEC;DATA;#1=#1;ENDSEC;END-ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, selector byte, raw []byte) {
		if len(raw) > int(sideCutFuzzLimits.MaxBytes) {
			t.Skip()
		}
		kind := selector % sideCutFuzzKinds
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		descriptor, e := sideCutFuzzRead(t, ctx, kind, raw)
		if e != nil {
			return
		}
		var out bytes.Buffer
		if e = sideCutFuzzWrite(ctx, &out, kind, descriptor); e != nil {
			if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
				return
			}
			t.Fatalf("accepted descriptor cannot re-emit: %v", e)
		}
		if out.Len() > int(sideCutFuzzLimits.MaxBytes) {
			t.Fatal("accepted output exceeds work budget")
		}
		rebuilt, e := sideCutFuzzRead(t, ctx, kind, out.Bytes())
		if e != nil {
			if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
				return
			}
			t.Fatalf("accepted output cannot re-import: %v", e)
		}
		if rebuilt != descriptor {
			t.Fatal("normalized descriptor drift")
		}
	})
}

func TestSideCutFuzzOuterPayloadsReject(t *testing.T) {
	t.Parallel()
	for kind := byte(0); kind < sideCutFuzzKinds; kind++ {
		raw := sideCutFuzzSeed(t, kind, false)
		bad := [][]byte{raw[:len(raw)/2], append([]byte("unexpected-prefix\x00"), raw...), append(append([]byte(nil), raw...), []byte("#999999=CARTESIAN_POINT('',(0.,0.,0.));")...), bytes.Replace(raw, []byte("END-ISO-10303-21;"), []byte("END-ISO-10303-21;\x00"), 1)}
		for i, payload := range bad {
			if _, e := sideCutFuzzRead(t, context.Background(), kind, payload); e == nil {
				t.Fatal("malformed outer payload accepted", kind, i)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e := sideCutFuzzRead(t, ctx, kind, raw); !errors.Is(e, context.Canceled) {
			t.Fatal("cancellation", kind, e)
		}
	}
}

func sideCutFuzzSemanticSeeds(t testing.TB, raw []byte) [][]byte {
	edits := []func(*File){
		func(f *File) {
			sideDiskEditRecord(f, "PRODUCT", func(r *Record) { r.Args[0].Text = "forged-side-cut-class" })
		},
		func(f *File) {
			var first Value
			for _, id := range f.Order {
				r := f.Entities[id].Records[0]
				if r.Type == "EDGE_CURVE" && f.Entities[r.Args[3].Ref].Records[0].Type == "CIRCLE" {
					if first.Kind == Reference {
						r.Args[3] = first
						return
					}
					first = r.Args[3]
				}
			}
		},
		func(f *File) {
			var first Value
			for _, id := range f.Order {
				r := f.Entities[id].Records[0]
				if r.Type == "CYLINDRICAL_SURFACE" {
					if first.Kind == Reference {
						r.Args[1] = first
						return
					}
					first = r.Args[1]
				}
			}
		},
	}
	var out [][]byte
	for _, edit := range edits {
		out = append(out, sideDiskMutateSource(t, raw, edit))
	}
	return out
}
func TestSideCutFuzzSemanticOwnersReject(t *testing.T) {
	slowtest.Skip(t)
	for kind := byte(0); kind < sideCutFuzzKinds; kind++ {
		for _, through := range []bool{false, true} {
			raw := sideCutFuzzSeed(t, kind, through)
			for i, bad := range sideCutFuzzSemanticSeeds(t, raw) {
				if bytes.Equal(bad, raw) {
					t.Fatal("unmodified mutation")
				}
				if _, e := sideCutFuzzRead(t, context.Background(), kind, bad); e == nil {
					t.Fatal("semantic owner accepted", kind, i)
				}
			}
		}
	}
}
