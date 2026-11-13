package document

import (
	"bytes"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestLabelsTransactionsAndPersistence(t *testing.T) {
	state := State{LengthUnit: 1, Nodes: map[string]Node{"node": {Transform: geom.Identity()}}, Labels: map[string]Label{
		"root": {Name: "Project", Target: DocumentLabel, Attributes: map[string]Attribute{"released": {Kind: "boolean"}}},
		"part": {Name: "Part", Target: NodeLabel, TargetID: "node", Parent: "root", Attributes: map[string]Attribute{
			"quantity": {Kind: "integer", Integer: 2}, "finish": {Kind: "string", Text: "matte"}, "scale": {Kind: "number", Number: .5},
		}},
	}}
	d, e := New(state, 2)
	if e != nil {
		t.Fatal(e)
	}
	state.Labels["part"].Attributes["quantity"] = Attribute{Kind: "integer", Integer: 55}
	s, _ := d.Snapshot()
	if s.Labels["part"].Attributes["quantity"].Integer != 2 {
		t.Fatal("caller mutation escaped")
	}
	tx := d.Begin()
	tx.State.Labels["part"].Attributes["quantity"] = Attribute{Kind: "integer", Integer: 4}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = d.Write(&b); e != nil {
		t.Fatal(e)
	}
	copy, e := Read(&b, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = copy.Snapshot()
	if s.Labels["part"].Attributes["quantity"].Integer != 4 {
		t.Fatal("label persistence lost value")
	}
	if e = d.Undo(); e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	if s.Labels["part"].Attributes["quantity"].Integer != 2 {
		t.Fatal("undo lost label")
	}
	tx = d.Begin()
	x := tx.State.Labels["root"]
	x.Parent = "part"
	tx.State.Labels["root"] = x
	if e = tx.Commit(); e == nil {
		t.Fatal("accepted label cycle")
	}
	tx = d.Begin()
	delete(tx.State.Nodes, "node")
	if e = tx.Commit(); e == nil {
		t.Fatal("accepted dangling label")
	}
	tx = d.Begin()
	tx.State.Labels["part"].Attributes["scale"] = Attribute{Kind: "number", Number: math.NaN()}
	if e = tx.Commit(); e == nil {
		t.Fatal("accepted nonfinite attribute")
	}
}
