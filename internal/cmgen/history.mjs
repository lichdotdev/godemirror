// history.mjs writes what @codemirror/commands' history makes of random
// sequences of edits, edits kept out of the history, isolation, selections,
// undo and redo, for the Go port's tests to match
// (testdata/history.golden.json). A dev tool, run like state.mjs:
//
//	node internal/cmgen/history.mjs "$CODEMIRROR_NODE_MODULES" testdata/history.golden.json
//
// Each step has an explicit time, so grouping by newGroupDelay is the same
// in both. Documents are ASCII: positions are bytes and UTF-16 code units
// alike.

import { writeFileSync } from "node:fs";
import { load, packageJSON } from "../lezer/gen/load.mjs";

const [modulesDir, out] = process.argv.slice(2);
const { EditorState, EditorSelection, Transaction } = await load(modulesDir, "@codemirror/state");
const { history, undo, redo, undoSelection, redoSelection, isolateHistory, undoDepth, redoDepth } =
  await load(modulesDir, "@codemirror/commands");

let seed = 0x415;
function random() {
  seed = (seed + 0x6d2b79f5) | 0;
  let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
}
const r = (n) => Math.floor(random() * n);
const pick = (l) => l[r(l.length)];
const rText = () => pick(["a", "bc", " ", "\n", "xyz"]);

const commands = { undo, redo, undoSelection, redoSelection };
let cases = [];
for (let i = 0; i < 60; i++) {
  let doc = pick(["", "abc", "one\ntwo"]);
  let state = EditorState.create({ doc, extensions: [history(), EditorState.allowMultipleSelections.of(true)] });
  let time = 1000, steps = [];
  for (let j = 0; j < 25; j++) {
    time += pick([10, 100, 400, 600, 2000]);
    let len = state.doc.length, step = { time };
    let kind = pick(["type", "type", "type", "delete", "receive", "select", "isolate", "undo", "undo", "redo", "undoSelection", "redoSelection"]);
    step.kind = kind;
    if (kind == "type" || kind == "receive" || kind == "delete") {
      let from = r(len + 1), to = kind == "delete" ? Math.min(len, from + r(3) + 1) : from;
      step.change = { from, to, insert: kind == "delete" ? "" : rText() };
      step.event = pick(["", "input.type", "delete.backward", "input.paste"]);
      step.isolate = r(5) == 0 ? pick(["before", "after", "full"]) : "";
      let annotations = [Transaction.time.of(time)];
      if (kind == "receive") annotations.push(Transaction.addToHistory.of(false));
      if (step.isolate) annotations.push(isolateHistory.of(step.isolate));
      state = state.update({ changes: step.change, annotations, userEvent: step.event || undefined }).state;
    } else if (kind == "select") {
      let a = r(len + 1), h = r(3) ? a : r(len + 1);
      step.sel = [a, h];
      step.event = pick(["", "select", "select.pointer"]);
      state = state.update({ selection: EditorSelection.single(a, h), annotations: Transaction.time.of(time), userEvent: step.event || undefined }).state;
    } else if (kind == "isolate") {
      step.isolate = pick(["before", "after", "full"]);
      state = state.update({ annotations: [Transaction.time.of(time), isolateHistory.of(step.isolate)] }).state;
    } else {
      let got = null;
      step.ok = commands[kind]({ state, dispatch: (tr) => (got = tr) });
      if (got) state = got.state;
    }
    step.doc = state.doc.toString();
    step.sel2 = state.selection.ranges.map((r) => [r.anchor, r.head]);
    step.main = state.selection.mainIndex;
    step.depths = [undoDepth(state), redoDepth(state)];
    steps.push(step);
  }
  cases.push({ doc, steps });
}

const version = packageJSON(modulesDir, "@codemirror/commands").version;
writeFileSync(out, `{"version": ${JSON.stringify(version)}, "cases": [\n${cases.map((c) => JSON.stringify(c)).join(",\n")}\n]}\n`);
