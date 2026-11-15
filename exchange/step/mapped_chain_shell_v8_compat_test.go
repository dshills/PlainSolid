package step

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"plainsolid/brep/trimmed"
)

func TestMappedChainSupportKnotNativeRecordCompatibilityV8(t *testing.T) {
	ctx := context.Background()
	// Native and canonical STEP bytes from the original v7 and four-cut v8
	// implementations, before the bounded acceptance of five and six cuts.
	for _, tc := range []struct {
		path, nativeSHA256, stepSHA256 string
		options                        MappedChainShellSTEPOptions
	}{
		{mixedBasisUnsplitMappedChainPath, "ab561af2b387b267a6d0fcc3a008a364a45f8fdbd7911a3a4bc74f519a556412",
			"ecc8b0b80565da5f3eb34585060f9c5ac6ed9d3a358ca9c83ab2b0995a553137", unsplitMappedChainMixedBasisOptions()},
		{fourKnotMappedChainPath, "2e773fa632fc5810d95adf8bd44467de0bb49c3d18e0b06276fe0712eaad2475",
			"c248b639ad9b8206b4f45736786cfea2db97823e1d6cdbf38bc08fd660a9ab24", fourKnotMappedChainOptions()},
	} {
		raw, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		tc.options.AllowUpToSixSupportKnots = true
		m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, tc.options)
		if err != nil {
			t.Fatal(err)
		}
		var native, step bytes.Buffer
		if err := m.Shell.WriteJSON(ctx, &native, trimmed.MappedChainShellNativeOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := WriteMappedChainRectangularOriginalShell(ctx, &step, m, Limits{}, tc.options); err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(native.Bytes())) != tc.nativeSHA256 || fmt.Sprintf("%x", sha256.Sum256(step.Bytes())) != tc.stepSHA256 {
			t.Fatal("the same original source must produce the same native and canonical bytes with the extended gate", tc.path)
		}
	}
}
