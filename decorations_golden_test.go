package godemirror

//go:generate node internal/cmgen/decorations.mjs $CODEMIRROR_NODE_MODULES testdata/decorations.golden.json

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"testing"
)

func TestDecorationsMapAsCodeMirrorsRangeSet(t *testing.T) {
	data, err := os.ReadFile("testdata/decorations.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Versions map[string]string
		Cases    []struct {
			Doc   string
			Lines []struct {
				ID  string
				Pos int
			}
			Widgets []struct {
				ID        string
				Pos, Side int
			}
			Changes []ChangeSpec
			Mapped  [][2]any
		}
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Versions["@codemirror/state"] != "6.7.6" || golden.Versions["@codemirror/view"] != "6.43.15" {
		t.Fatalf("goldens from %v", golden.Versions)
	}
	for i, c := range golden.Cases {
		// The test tells decorations apart by a number: a line's hover
		// group, a widget's placement.
		var d Decorations
		for _, l := range c.Lines {
			n, _ := strconv.Atoi(l.ID[1:])
			d.Lines = append(d.Lines, LineDecorationAt{Pos: l.Pos, Decoration: LineDecoration{HoverGroup: n}})
		}
		for _, w := range c.Widgets {
			n, _ := strconv.Atoi(w.ID[1:])
			d.Widgets = append(d.Widgets, LineWidgetAt{Pos: w.Pos, Side: w.Side, Placement: Placement(n)})
		}
		changes := mustChanges(t, c.Changes, len(c.Doc))
		mapped := d.sorted().mapThrough(changes)
		var got [][2]any
		for _, l := range mapped.Lines {
			got = append(got, [2]any{"l" + strconv.Itoa(l.Decoration.HoverGroup), float64(l.Pos)})
		}
		for _, w := range mapped.Widgets {
			got = append(got, [2]any{"w" + strconv.Itoa(int(w.Placement)), float64(w.Pos)})
		}
		slices.SortFunc(got, func(a, b [2]any) int {
			return cmp.Or(cmp.Compare(a[1].(float64), b[1].(float64)), cmp.Compare(a[0].(string), b[0].(string)))
		})
		if fmt.Sprint(got) != fmt.Sprint(c.Mapped) {
			t.Errorf("case %d (%q through %v): %v, CodeMirror %v", i, c.Doc, c.Changes, got, c.Mapped)
		}
	}
}
