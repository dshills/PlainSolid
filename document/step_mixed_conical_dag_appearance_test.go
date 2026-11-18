package document

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

const (
	appearanceRoot      = "step-product-5"
	appearanceLeftCore  = appearanceRoot + "/occ-54/occ-85"
	appearanceRightCore = appearanceRoot + "/occ-466/occ-472"
	appearanceSelected  = appearanceLeftCore + "/occ-251"
	appearanceSibling   = appearanceRightCore + "/occ-251"
	appearanceFace      = "step-style:step-product-254:face-3"
)

func TestMixedConicalAP242DAGAppearanceDocument(t *testing.T) {
	slowtest.Skip(t)
	raw, d := openMixedConicalDAGAppearanceDocument(t)
	ctx, o := context.Background(), geom.SurfacePatchOptions{}
	s, _ := d.Snapshot()
	if s.LengthUnit != .001 || len(s.Trimmed) != 2 || len(s.Nodes) != 12 ||
		s.Nodes[appearanceSelected].Asset != s.Nodes[appearanceSibling].Asset {
		t.Fatal("styled source did not preserve shared native definitions")
	}
	owner := s.Labels[mixedConicalDAGStyleKey(appearanceSelected)]
	if owner.Target != NodeLabel || owner.TargetID != appearanceSelected || owner.Parent != "source-node:"+appearanceSelected {
		t.Fatal("contextual paint is attached to the wrong document node", owner)
	}
	for attr, id := range map[string]int64{
		"outer-nauo": 54, "middle-nauo": 85, "nauo": 251, "body": 95, "prefix-shuo": 479, "terminal-shuo": 478,
		"styled-item": 492, "base-styled-item": 484, "style-assignment": 493, "style-context": 482,
		"style-definition-representation": 480, "style-product-shape": 481, "style-placement": 68,
		"context-dependent-representation": 247, "transformation": 249, "presentation": 483,
	} {
		if owner.Attributes["step.source."+attr] != (Attribute{Kind: "integer", Integer: id}) {
			t.Fatal("lost typed STEP occurrence paint owner", attr, owner)
		}
	}
	face := s.Labels[appearanceFace]
	if face.Target != FaceLabel || face.TargetID != "step-product-254" || face.Face == nil || *face.Face != 3 ||
		face.Attributes["step.source.face"] != conicalIntAttr(336) || face.Attributes["step.source.styled-item"] != conicalIntAttr(509) {
		t.Fatal("lost original face paint owner", face)
	}
	checkConicalDocPaint(t, s, appearanceSelected, 0, step.RGB{B: 1})
	checkConicalDocPaint(t, s, appearanceSibling, 0, step.RGB{R: 1})
	checkConicalDocPaint(t, s, appearanceLeftCore+"/occ-245", 0, step.RGB{R: 1})
	for _, path := range []string{appearanceLeftCore + "/occ-443", appearanceRightCore + "/occ-443", appearanceRoot + "/occ-477"} {
		checkConicalDocPaint(t, s, path, 0, step.RGB{G: 1})
		checkConicalDocPaint(t, s, path, 3, step.RGB{R: 1, G: 1})
	}
	l := mixedConicalAppearanceLimits()
	var unchanged bytes.Buffer
	if err := s.WriteMixedConicalAP242DAGAppearanceSTEP(ctx, &unchanged, bytes.NewReader(raw), l, o); err != nil ||
		!bytes.Equal(unchanged.Bytes(), raw) {
		t.Fatal("unchanged styled document changed its original bytes", err)
	}
	if strict, err := ReadMixedConicalAP242DAGSTEP(ctx, bytes.NewReader(raw), l, o, 0); err == nil || strict != nil {
		t.Fatal("ordinary strict document reader admitted the styled dialect")
	}
	l.MaxNodes--
	if partial, err := ReadMixedConicalAP242DAGAppearanceSTEP(ctx, bytes.NewReader(raw), l, o, 0); partial != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("styled reader published a partial document under a tight node limit", err)
	}
}

func TestMixedConicalAP242DAGAppearanceDocumentWrite(t *testing.T) {
	slowtest.Skip(t)
	raw, d := openMixedConicalDAGAppearanceDocument(t)
	ctx, l, o := context.Background(), mixedConicalAppearanceLimits(), geom.SurfacePatchOptions{}
	base, _ := d.Snapshot()
	tx := d.Begin()
	cyan, magenta, orange := step.RGB{G: 1, B: 1}, step.RGB{R: 1, B: 1}, step.RGB{R: 1, G: .5}
	repaintConicalDoc(t, tx.State, mixedConicalDAGStyleKey("step-product-89"), cyan)
	repaintConicalDoc(t, tx.State, mixedConicalDAGStyleKey(appearanceSelected), magenta)
	repaintConicalDoc(t, tx.State, appearanceFace, orange)
	for _, path := range []string{appearanceSelected, appearanceSibling} {
		n := tx.State.Nodes[path]
		n.Name = "Repositioned shared conical solid"
		n.Transform[0][3] += 5.125
		tx.State.Nodes[path] = n
	}
	n := tx.State.Nodes[appearanceRoot+"/occ-54"]
	n.Transform[1][3] += 8
	tx.State.Nodes[appearanceRoot+"/occ-54"] = n
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	s, _ := d.Snapshot()
	checkConicalDocPaint(t, s, appearanceSelected, 0, magenta)
	checkConicalDocPaint(t, s, appearanceSibling, 0, cyan)
	checkConicalDocPaint(t, s, appearanceRoot+"/occ-477", 3, orange)
	if s.Labels[mixedConicalDAGStyleKey(appearanceSelected)].Attributes["step.source.srgb.blue"] != (Attribute{Kind: "number", Number: 1}) ||
		s.Labels[mixedConicalDAGStyleKey(appearanceSelected)].Attributes["step.source.srgb.red"] != (Attribute{Kind: "number"}) ||
		!reflect.DeepEqual(s.Labels["source-node:"+appearanceSelected], base.Labels["source-node:"+appearanceSelected]) {
		t.Fatal("editing paint or placement changed source provenance")
	}
	var native, replay, out bytes.Buffer
	if err := d.Write(&native); err != nil {
		t.Fatal(err)
	}
	d, err := Read(bytes.NewReader(native.Bytes()), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Write(&replay); err != nil || !bytes.Equal(native.Bytes(), replay.Bytes()) {
		t.Fatal("styled document did not save and reopen byte-identically", err)
	}
	s, _ = d.Snapshot()
	if err := s.WriteMixedConicalAP242DAGAppearanceSTEP(ctx, &out, bytes.NewReader(raw), l, o); err != nil {
		t.Fatal(err)
	}
	a, err := step.ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(out.Bytes()), l.Assembly, o)
	if err != nil || a.PathOverride.Path != [3]int64{54, 85, 251} ||
		a.PathOverride.Context.StyledItemID != 492 || a.PathOverride.Context.AssignmentID != 493 ||
		a.PathOverride.Context.PlacementID != 68 || a.PathOverride.Context.ContextRepresentationID != 482 || a.PathOverride.Context.Color != magenta ||
		len(a.FaceStyles) != 1 || a.FaceStyles[0].FaceID != 336 || a.FaceStyles[0].Color != orange {
		t.Fatalf("writeback lost original style claims: %+v, %v", a.PathOverride, err)
	}
	checkConicalDocumentEditedEntities(t, raw, a.DAG.Assembly.Source)
	reopened, err := ReadMixedConicalAP242DAGAppearanceSTEP(ctx, bytes.NewReader(out.Bytes()), l, o, 0)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := reopened.Snapshot()
	if !reflect.DeepEqual(r.Nodes, s.Nodes) || len(r.Trimmed) != 2 {
		t.Fatal("translated shared relationship reopened at different nodes")
	}
	checkConicalDocPaint(t, r, appearanceSelected, 0, magenta)
	checkConicalDocPaint(t, r, appearanceSibling, 0, cyan)
	checkConicalDocPaint(t, r, appearanceRightCore+"/occ-443", 3, orange)
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_APPEARANCE"); oracle != "" {
		checkConicalDocumentAppearanceOracle(t, oracle, out.Bytes())
	}
}

func TestMixedConicalAP242DAGAppearanceDocumentReject(t *testing.T) {
	slowtest.Skip(t)
	raw, d := openMixedConicalDAGAppearanceDocument(t)
	ctx, l, o := context.Background(), mixedConicalAppearanceLimits(), geom.SurfacePatchOptions{}
	for _, tc := range []struct {
		name string
		edit func(State)
	}{
		{"source digest", func(s State) {
			s.Labels[mixedConicalAppearanceSourceLabel].Attributes["step.source.original-sha256"] = conicalStrAttr("unproved")
		}},
		{"retargeted SHUO", func(s State) {
			id := mixedConicalDAGStyleKey(appearanceSelected)
			label := s.Labels[id]
			label.TargetID = appearanceSibling
			s.Labels[id] = label
		}},
		{"forged original face", func(s State) {
			s.Labels[appearanceFace].Attributes["step.source.face"] = conicalIntAttr(337)
		}},
		{"partial paint", func(s State) {
			delete(s.Labels[appearanceFace].Attributes, "step.srgb.red")
		}},
		{"new override", func(s State) {
			s.Labels[mixedConicalDAGStyleKey(appearanceSibling)] = Label{Target: NodeLabel, TargetID: appearanceSibling,
				Attributes: colorAttributes(step.RGB{R: 1})}
		}},
		{"conflicting shared use", func(s State) {
			n := s.Nodes[appearanceSelected]
			n.Transform[0][3] += 10
			s.Nodes[appearanceSelected] = n
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := d.Snapshot()
			tc.edit(s)
			var out bytes.Buffer
			if err := s.WriteMixedConicalAP242DAGAppearanceSTEP(ctx, &out, bytes.NewReader(raw), l, o); !errors.Is(err, geom.ErrUnrepresentable) || out.Len() != 0 {
				t.Fatal("changed style ownership or ambiguous edits published bytes", err)
			}
		})
	}
}

func openMixedConicalDAGAppearanceDocument(t *testing.T) ([]byte, *Document) {
	t.Helper()
	raw, err := os.ReadFile("../exchange/step/testdata/occt-dag-mixed-conical-ap242-appearance.step")
	if err != nil {
		t.Fatal(err)
	}
	d, err := ReadMixedConicalAP242DAGAppearanceSTEP(context.Background(), bytes.NewReader(raw), mixedConicalAppearanceLimits(), geom.SurfacePatchOptions{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	return raw, d
}

func mixedConicalAppearanceLimits() MixedConicalAP242DAGDocumentLimits {
	return MixedConicalAP242DAGDocumentLimits{Assembly: step.MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}, MaxNodes: 12}
}

func checkConicalDocPaint(t *testing.T, s State, path string, face int, want step.RGB) {
	t.Helper()
	if c, present, err := s.MixedConicalAP242DAGSurfaceColor(path, face); err != nil || !present || c != want {
		t.Fatalf("wrong paint for path %q and face %d: %v, %v", path, face, c, err)
	}
}

func repaintConicalDoc(t *testing.T, s State, label string, c step.RGB) {
	t.Helper()
	owner, ok := s.Labels[label]
	if !ok {
		t.Fatal("missing paint owner", label)
	}
	for key, value := range colorAttributes(c) {
		owner.Attributes[key] = value
	}
}

func checkConicalDocumentEditedEntities(t *testing.T, original []byte, edited *step.File) {
	t.Helper()
	base, err := step.Parse(bytes.NewReader(original), step.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(base.Order, edited.Order) {
		t.Fatal("transferring paint and private translations changed source entity identities")
	}
	required := map[int64]bool{16: true, 69: true, 251: true, 491: true, 499: true, 516: true}
	for _, id := range base.Order {
		if reflect.DeepEqual(base.Entities[id], edited.Entities[id]) {
			continue
		}
		// The unchanged exact frame directions can use normalized zero tokens.
		if !required[id] && !slices.Contains([]int64{17, 18, 70, 71}, id) {
			t.Fatal("edit changed an unrelated original STEP entity", id)
		}
		delete(required, id)
	}
	if len(required) != 0 {
		t.Fatal("a requested edit did not reach its original STEP record", required)
	}
}

func checkConicalDocumentAppearanceOracle(t *testing.T, oracle string, raw []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "edited.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(oracle, "read-dag-appearance", path).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned XCAF could not reopen edited document: %v\n%s", err, out)
	}
	var report struct {
		StyledPaths []struct {
			Path     []int
			Product  [3]float64
			Override *[3]float64
			Faces    []struct {
				PlaneZ float64
				Color  [3]float64
			}
		}
	}
	if err := json.Unmarshal(out, &report); err != nil || len(report.StyledPaths) != 7 {
		t.Fatalf("invalid pinned XCAF appearance report: %v\n%s", err, out)
	}
	near := func(a, b [3]float64) bool {
		return math.Abs(a[0]-b[0])+math.Abs(a[1]-b[1])+math.Abs(a[2]-b[2]) <= 1e-7
	}
	for _, hit := range report.StyledPaths {
		if slices.Equal(hit.Path, []int{1, 1, 2}) {
			if hit.Override == nil || !near(*hit.Override, [3]float64{1, 0, 1}) {
				t.Fatal("selected XCAF SHUO lost the edited magenta", string(out))
			}
		} else if hit.Override != nil {
			t.Fatal("contextual paint leaked to another XCAF occurrence", hit.Path)
		}
		if len(hit.Path) == 3 && hit.Path[2] < 3 {
			if len(hit.Faces) != 0 || !near(hit.Product, [3]float64{0, 1, 1}) {
				t.Fatal("rounded XCAF product lost shared cyan paint", hit.Path)
			}
		} else if !near(hit.Product, [3]float64{0, 1, 0}) || len(hit.Faces) != 1 ||
			hit.Faces[0].PlaneZ != 2 || !near(hit.Faces[0].Color, [3]float64{1, .5, 0}) {
			t.Fatal("XCAF chamfer lost its original independently painted cap", hit.Path)
		}
	}
	t.Logf("pinned XCAF edited appearance: %s", out)
}
