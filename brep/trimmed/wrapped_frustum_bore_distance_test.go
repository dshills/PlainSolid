package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestOCCTWrappedFrustumBoreBoundaryDistance(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_FRUSTUM_BORE_DISTANCE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_FRUSTUM_BORE_DISTANCE to the optional OCCT executable")
	}
	ctx, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	raw, e := exec.CommandContext(ctx, program).CombinedOutput()
	if e != nil {
		t.Fatalf("independent OCCT bore distance: %v: %s", e, raw)
	}
	var result []float64
	if e := json.Unmarshal(raw, &result); e != nil || len(result) != 5 {
		t.Fatal("invalid OCCT distance record", string(raw), e)
	}
	for i, want := range [5]float64{2, 1 + math.Pow(1-math.Sqrt(.125), 2), 1, 1.0 / 16, 9.0 / 64} {
		if math.Abs(result[i]-want) > 1e-8 {
			t.Fatal("OCCT cut distance disagrees with the independent analytic fixture", i, result[i], want)
		}
	}
}
