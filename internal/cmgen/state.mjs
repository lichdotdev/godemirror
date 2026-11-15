// state.mjs writes what the real @codemirror/state makes of random changes and
// selections, for the Go port's tests to match (testdata/state.golden.json):
// a dev tool, run with a node_modules directory outside the repository that
// holds @codemirror/state at the version the port names:
//
//	node internal/cmgen/state.mjs "$CODEMIRROR_NODE_MODULES" testdata/state.golden.json
//
// The cases come from a seeded generator, so a run writes the same file. The
// documents are ASCII, so CodeMirror's UTF-16 positions are the port's byte
// offsets. Every position is mapped with assoc -1 and 1 and in each MapMode;
// null stands for a position MapMode reports deleted.

import { writeFileSync } from "node:fs";
import { load, packageJSON } from "../lezer/gen/load.mjs";

const [modulesDir, out] = process.argv.slice(2);
const { ChangeSet, EditorSelection, EditorState, MapMode, Text } = await load(modulesDir, "@codemirror/state");

// mulberry32, a small seeded generator.
let seed = 0x5eed;
function random() {
  seed = (seed + 0x6d2b79f5) | 0;
  let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
}
const r = (n) => Math.floor(random() * n);
const pieces = ["a", "b", "c", "x", "y", "z", "\n", "\r\n", "\r"];
const rText = (n) => Array.from({ length: n }, () => pieces[r(pieces.length)]).join("");
const rDoc = (n) => Array.from({ length: n }, () => (r(6) == 0 ? "\n" : "abcdef"[r(6)])).join("");

function rChange(len) {
  if (len == 0 || r(3) == 0) return { from: r(len + 1), insert: rText(r(4) + 1) };
  let from = r(len), to = Math.min(from + r(5) + 1, len);
  return r(2) == 0 ? { from, to, insert: rText(r(3) + 1) } : { from, to };
}
const rChanges = (len, count) => Array.from({ length: count }, () => rChange(len));
const spec = (c) => ({ from: c.from, to: c.to ?? c.from, insert: c.insert ?? "" });

function rRanges(len, count) {
  return Array.from({ length: count }, () => {
    let a = r(len + 1);
    return r(3) == 0 ? { anchor: a, head: a } : { anchor: a, head: r(len + 1) };
  });
}

function mapAll(set) {
  let maps = [];
  for (let pos = 0; pos <= set.length; pos++)
    for (let assoc of [-1, 1])
      maps.push([MapMode.Simple, MapMode.TrackDel, MapMode.TrackBefore, MapMode.TrackAfter].map((mode) => set.mapPos(pos, assoc, mode)));
  return maps;
}

const ranges = (sel) => ({ ranges: sel.ranges.map((r) => [r.anchor, r.head]), main: sel.mainIndex });

let cases = [];
for (let i = 0; i < 150; i++) {
  let size = r(20), doc = rDoc(size), text = Text.of(doc.split("\n"));
  let specsA = rChanges(size, r(4) + 1), specsB = rChanges(size, r(4) + 1);
  let a = ChangeSet.of(specsA, size), b = ChangeSet.of(specsB, size);
  let specsC = rChanges(a.newLength, r(4) + 1), c = ChangeSet.of(specsC, a.newLength);
  let selRanges = rRanges(size, r(4) + 1), main = r(selRanges.length);
  let sel = EditorSelection.create(selRanges.map((s) => EditorSelection.range(s.anchor, s.head)), main);
  let state = EditorState.create({ doc, selection: sel, extensions: EditorState.allowMultipleSelections.of(true) });
  let insert = rText(r(3));
  let replaced = state.update(state.replaceSelection(insert)).state;
  cases.push({
    doc, a: specsA.map(spec), b: specsB.map(spec), c: specsC.map(spec),
    aSections: a.desc.toString(), applied: a.apply(text).toString(),
    composed: a.compose(c).desc.toString(), composedApplied: a.compose(c).apply(text).toString(),
    mapped: a.map(b).desc.toString(), mappedBefore: a.map(b, true).desc.toString(),
    inverted: a.invert(text).desc.toString(), restored: a.invert(text).apply(a.apply(text)).toString(),
    mapPos: mapAll(a),
    selection: selRanges.map((s) => [s.anchor, s.head]), main,
    created: ranges(sel), selectionMapped: ranges(sel.map(a)),
    insert, replacedDoc: replaced.doc.toString(), replacedSelection: ranges(replaced.selection),
  });
}

const version = packageJSON(modulesDir, "@codemirror/state").version;
writeFileSync(out, `{"version": ${JSON.stringify(version)}, "cases": [\n${cases.map((c) => JSON.stringify(c)).join(",\n")}\n]}\n`);
