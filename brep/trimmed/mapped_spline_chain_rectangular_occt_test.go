package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestMappedSplineChainRectangularSolidOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_SHELL")
	if exe == "" {
		t.Skip("optional independently authored OCCT mapped-chain shell")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, exe).CombinedOutput()
	if err != nil {
		t.Fatalf("independent OCCT shell: %v\n%s", err, data)
	}
	var reference struct {
		Vertices, Edges, Faces int
		Volume, Area           float64
		Center                 [3]float64
	}
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	boundary, closing, planar, seams := mappedChainClosedFixture(t)
	shell, err := CertifyMappedSplineChainShell(ctx, boundary, closing, [2]bool{false, true}, planar, seams, MappedChainShellProofOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := shell.BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if reference.Vertices != len(out.Solid.vertices) || reference.Edges != len(out.Solid.edges) || reference.Faces != len(out.Solid.faces) {
		t.Fatal("native and independently built OCCT topology differ", reference)
	}
	mass, err := out.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil {
		t.Fatal(err)
	}
	distance := mass.Centroid.Sub(geom.Vec3{X: reference.Center[0], Y: reference.Center[1], Z: reference.Center[2]}).Length()
	if !(math.Abs(mass.Volume-reference.Volume) < 1e-8 && math.Abs(mass.Area-reference.Area) < 1e-8 && distance < 1e-8) {
		t.Fatalf("native mass %+v differs from independent OCCT %+v", mass, reference)
	}
	t.Logf("OCCT vertices=%d edges=%d faces=%d volume=%.15g area=%.15g center=%v",
		reference.Vertices, reference.Edges, reference.Faces, reference.Volume, reference.Area, reference.Center)
}
