package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"strconv"
	"strings"
	"testing"
)

func TestPreparedNativeObliqueCavityDistance(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	frame := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(-4, 5), r(0, 1), r(3, 5), r(0, 1)}}
	s, _, e := ObliqueTorusCavity(ctx, 3, 1, 8, -6, 6, frame, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteJSON(&b, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(bytes.NewReader(b.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	prepared, e := PrepareObliqueTorusCavity(ctx, s, 3, 1, 8, -6, 6, frame, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	worldFrame := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}
	pose, e := placement.NewRationalRigid(worldFrame, 4096)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		point [3]*big.Rat
		faces int
	}{{[3]*big.Rat{r(4, 1), r(0, 1), r(-4, 1)}, 4}, {[3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, 8}, {[3]*big.Rat{r(14, 5), r(4, 1), r(-12, 5)}, 2}, {[3]*big.Rat{r(9, 1), r(0, 1), r(7, 1)}, 0}} {
		o := geom.TorusCylinderDistanceOptions{SquaredTolerance: r(1, 1000000)}
		local, e := prepared.BoundBoundaryDistance(ctx, tc.point, o)
		if e != nil {
			t.Fatal(e)
		}
		if tc.faces > 0 && len(local.WitnessFaces[0]) != tc.faces {
			t.Fatal("exact chart ownership", local.WitnessFaces[0], tc.faces)
		}
		world, e := pose.TransformPoint(ctx, tc.point, false, 4096)
		if e != nil {
			t.Fatal(e)
		}
		got, e := prepared.BoundPlacedBoundaryDistance(ctx, pose, world, o)
		if e != nil {
			t.Fatal(e)
		}
		if got.Base.Lower.Cmp(local.Lower) != 0 || got.Base.Upper.Cmp(local.Upper) != 0 || got.Base.ExactOperations != local.ExactOperations {
			t.Fatal("one invariant source query")
		}
		if got.Witnesses[0].Axis[0].Cmp(r(24, 25)) != 0 || got.Witnesses[0].Axis[2].Cmp(r(-7, 25)) != 0 || got.Witnesses[1].Axis[0].Cmp(r(4, 5)) != 0 {
			t.Fatal("independent support axes")
		}
		if got.Witnesses[0].Base.WholeCircle && got.Witnesses[0].CircleDirections[0][0] == nil {
			t.Fatal("family lost")
		}
		if path := os.Getenv("PLAINSOLID_OCCT_OBLIQUE_CAVITY_DISTANCE"); path != "" {
			args := []string{}
			for _, x := range world {
				f, _ := x.Float64()
				args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
			}
			args = append(args, "oblique")
			data, e := exec.Command(path, args...).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			d, e := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
			lo, _ := got.Base.Lower.Float64()
			hi, _ := got.Base.Upper.Float64()
			if e != nil || d < lo-1e-7 || d > hi+1e-7 {
				t.Fatal("world OCCT", d, lo, hi, e)
			}
		}
	}
	// A positive azimuth below binary64 range must not become a quarter seam.
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 1200))
	exactSide, e := prepared.BoundBoundaryDistance(ctx, [3]*big.Rat{r(4, 1), tiny, r(-4, 1)}, geom.TorusCylinderDistanceOptions{SquaredTolerance: r(1, 1000000)})
	if e != nil || len(exactSide.WitnessFaces[0]) != 2 {
		t.Fatal("subfloat chart owner", e, exactSide.WitnessFaces[0])
	}
	changed := frame
	changed[0][3] = r(2, 1)
	if _, e := PrepareObliqueTorusCavity(ctx, s, 3, 1, 8, -6, 6, changed, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("wrong native source")
	}
	bad, e := prepared.BoundPlacedBoundaryDistance(ctx, pose, [3]*big.Rat{r(1, 1), r(2, 1), r(3, 1)}, geom.TorusCylinderDistanceOptions{SquaredTolerance: r(1, 100), Arithmetic: geom.SurfacePatchOptions{MaxOperations: 1}})
	if e == nil || bad.Base.Lower != nil {
		t.Fatal("atomic quota")
	}
}
