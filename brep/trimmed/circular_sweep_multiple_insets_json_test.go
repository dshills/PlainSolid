//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestExactCircularSweepMultipleInsetsJSONReprovesOriginalMembers(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	host, guests := multipleSweepInsetsFixture()
	var raw, again bytes.Buffer
	if err := WriteExactCircularSweepInsetsJSON(ctx, &raw, host, guests); err != nil {
		t.Fatal(err)
	}
	p, err := ReadExactCircularSweepInsetsJSON(ctx, bytes.NewReader(raw.Bytes()), CircularSweepInsetsJSONLimits{})
	if err != nil || len(p.Join.Solid.Faces()) != 40 || len(p.Join.Guests) != 3 || p.Join.Guests[2].HostCap != 0 ||
		p.Join.HostFaces[1][2] != 29 || p.Join.Guests[1].EdgeOffset != 48 {
		t.Fatalf("reopened source lost the original association of the three guests: %v", err)
	}
	if err := WriteExactCircularSweepInsetsJSON(ctx, &again, p.Host, p.Guests); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatalf("canonical source replay changed native bytes: %v", err)
	}
	bad := bytes.Replace(raw.Bytes(), []byte(`"EdgeOffset":48`), []byte(`"EdgeOffset":49`), 1)
	if bytes.Equal(bad, raw.Bytes()) {
		t.Fatal("test did not modify the actual original edge claim")
	}
	if bad, err := ReadExactCircularSweepInsetsJSON(ctx, bytes.NewReader(bad), CircularSweepInsetsJSONLimits{}); !errors.Is(err, geom.ErrInvalid) ||
		len(bad.Guests) != 0 || len(bad.Join.Solid.Faces()) != 0 {
		t.Fatalf("published an altered source ownership claim: %v", err)
	}
	if bad, err := ReadExactCircularSweepInsetsJSON(ctx, bytes.NewReader(raw.Bytes()), CircularSweepInsetsJSONLimits{MaxGuests: 2}); !errors.Is(err, geom.ErrConvergence) ||
		len(bad.Join.Solid.Faces()) != 0 {
		t.Fatalf("ignored the original member limit: %v", err)
	}
}
