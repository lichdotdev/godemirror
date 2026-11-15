// decorations.mjs writes where @codemirror/state's RangeSet.map moves
// @codemirror/view's line and widget decorations through random changes,
// for the Go port's tests to match (testdata/decorations.golden.json). A dev
// tool, run like state.mjs:
//
//	node internal/cmgen/decorations.mjs "$CODEMIRROR_NODE_MODULES" testdata/decorations.golden.json
//
// Documents are ASCII: positions are bytes and UTF-16 code units alike.

import { writeFileSync } from "node:fs";
import { load, packageJSON } from "../lezer/gen/load.mjs";

const [modulesDir, out] = process.argv.slice(2);
const { ChangeSet, RangeSet, Text } = await load(modulesDir, "@codemirror/state");
const { Decoration, WidgetType } = await load(modulesDir, "@codemirror/view");

class Widget extends WidgetType {
  constructor(id) { super(); this.id = id; }
  eq(other) { return other.id == this.id; }
  toDOM() { return null; }
}

let seed = 0xdec0;
function random() {
  seed = (seed + 0x6d2b79f5) | 0;
  let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
}
const r = (n) => Math.floor(random() * n);
const pick = (l) => l[r(l.length)];
const rText = (n) => Array.from({ length: n }, () => pick(["a", "b", "\n", " "])).join("");

function rChange(len) {
  if (len == 0 || r(3) == 0) return { from: r(len + 1), insert: rText(r(3) + 1) };
  let from = r(len), to = Math.min(from + r(4) + 1, len);
  return r(2) == 0 ? { from, to, insert: rText(r(3)) } : { from, to };
}

let cases = [];
for (let i = 0; i < 200; i++) {
  let doc = rText(r(20) + 1), text = Text.of(doc.split("\n"));
  let lines = [], widgets = [], ranges = [];
  for (let n = 1; n <= text.lines; n++) if (r(2)) {
    let id = "l" + n;
    lines.push({ id, pos: text.line(n).from });
    ranges.push(Decoration.line({ attributes: { id } }).range(text.line(n).from));
  }
  for (let k = r(4); k > 0; k--) {
    let id = "w" + k, pos = r(doc.length + 1), side = pick([-1, 0, 1]);
    widgets.push({ id, pos, side });
    ranges.push(Decoration.widget({ widget: new Widget(id), side }).range(pos));
  }
  let set = RangeSet.of(ranges, true);
  let specs = Array.from({ length: r(3) + 1 }, () => rChange(doc.length));
  let changes = ChangeSet.of(specs, doc.length);
  let mapped = [];
  set.map(changes).between(0, changes.newLength, (from, _to, value) => {
    mapped.push([value.spec.attributes ? value.spec.attributes.id : value.spec.widget.id, from]);
  });
  mapped.sort((a, b) => a[1] - b[1] || (a[0] < b[0] ? -1 : 1));
  cases.push({ doc, lines, widgets, changes: specs.map((c) => ({ from: c.from, to: c.to ?? c.from, insert: c.insert ?? "" })), mapped });
}

const versions = { "@codemirror/state": packageJSON(modulesDir, "@codemirror/state").version,
  "@codemirror/view": packageJSON(modulesDir, "@codemirror/view").version };
writeFileSync(out, `{"versions": ${JSON.stringify(versions)}, "cases": [\n${cases.map((c) => JSON.stringify(c)).join(",\n")}\n]}\n`);
