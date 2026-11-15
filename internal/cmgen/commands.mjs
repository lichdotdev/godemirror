// commands.mjs writes what CodeMirror's commands (@codemirror/search's
// occurrence selection among them) and input handling make of
// random documents and selections, for the Go port's tests to match
// (testdata/commands.golden.json). A dev tool, run like state.mjs, with
// @codemirror/state, view, commands, language and search installed:
//
//	node internal/cmgen/commands.mjs "$CODEMIRROR_NODE_MODULES" testdata/commands.golden.json
//
// The commands that need only a state run as @codemirror/commands exports
// them. The rest need an EditorView, which needs a browser; for those, the
// functions marked "from" below are copied from the named source file,
// unchanged but for taking the state where they took a view, and run on the
// real @codemirror/state: a line is one left-to-right bidi span, as in
// godemirror. Positions are UTF-16 code units, as CodeMirror counts them.

import { writeFileSync } from "node:fs";
import { load, packageJSON } from "../lezer/gen/load.mjs";

const [modulesDir, out] = process.argv.slice(2);
const { EditorState, EditorSelection, CharCategory, findClusterBreak, findColumn, countColumn, Text } = await load(modulesDir, "@codemirror/state");
const { BidiSpan, Direction } = await load(modulesDir, "@codemirror/view");
const commands = await load(modulesDir, "@codemirror/commands");
const { indentUnit } = await load(modulesDir, "@codemirror/language");
const search = await load(modulesDir, "@codemirror/search");

// from @codemirror/view src/bidi.ts
let movedOver = "";
function moveVisually(line, order, dir, start, forward) {
  if (!line.length) return null
  let startIndex = start.head - line.from, spanI
  if (start.head == line.from && start.assoc < 0) {
    if (!forward) return null
    startIndex = order[spanI = 0].side(false, dir)
  } else if (start.head == line.to && start.assoc > 0) {
    if (forward) return null
    startIndex = order[spanI = order.length - 1].side(true, dir)
  } else {
    spanI = BidiSpan.find(order, startIndex, start.bidiLevel ?? -1, start.assoc)
  }
  let span = order[spanI], spanEnd = span.side(forward, dir)
  if (startIndex == spanEnd) {
    let nextI = spanI += forward ? 1 : -1
    if (nextI < 0 || nextI >= order.length) return null
    span = order[spanI = nextI]
    startIndex = span.side(!forward, dir)
    spanEnd = span.side(forward, dir)
  }
  let nextIndex = findClusterBreak(line.text, startIndex, span.forward(forward, dir))
  if (nextIndex < span.from || nextIndex > span.to) nextIndex = spanEnd
  movedOver = line.text.slice(Math.min(startIndex, nextIndex), Math.max(startIndex, nextIndex))
  let nextSpan = spanI == (forward ? order.length - 1 : 0) ? null : order[spanI + (forward ? 1 : -1)]
  if (nextIndex == spanEnd) {
    if (!nextSpan) return forward ? EditorSelection.cursor(line.to, 1) : EditorSelection.cursor(line.from, -1)
    if (nextSpan.level + (forward ? 0 : 1) < span.level)
      return EditorSelection.cursor(nextSpan.side(!forward, dir) + line.from,
                                    nextSpan.forward(forward, dir) ? 1 : -1, nextSpan.level)
  }
  return EditorSelection.cursor(nextIndex + line.from, span.forward(forward, dir) ? -1 : 1, span.level)
}

const ltr = (line) => [new BidiSpan(0, line.length, 0)];

// from @codemirror/view src/cursor.ts
function moveByChar(state, start, forward, by) {
  let line = state.doc.lineAt(start.head), spans = ltr(line)
  let direction = Direction.LTR
  for (let cur = start, check = null;;) {
    let next = moveVisually(line, spans, direction, cur, forward), char = movedOver
    if (!next) {
      if (line.number == (forward ? state.doc.lines : 1)) return cur
      char = "\n"
      line = state.doc.line(line.number + (forward ? 1 : -1))
      spans = ltr(line)
      next = forward ? EditorSelection.cursor(line.from, -1) : EditorSelection.cursor(line.to, 1)
    }
    if (!check) {
      if (!by) return next
      check = by(char)
    } else if (!check(char)) {
      return cur
    }
    cur = next
  }
}

// from @codemirror/view src/cursor.ts
function byGroup(state, pos, start) {
  let categorize = state.charCategorizer(pos)
  let cat = categorize(start)
  return (next) => {
    let nextCat = categorize(next)
    if (cat == CharCategory.Space) cat = nextCat
    return cat == nextCat
  }
}

// from @codemirror/view src/cursor.ts
function groupAt(state, pos, bias = 1) {
  let categorize = state.charCategorizer(pos)
  let line = state.doc.lineAt(pos), linePos = pos - line.from
  if (line.length == 0) return EditorSelection.cursor(pos)
  if (linePos == 0) bias = 1
  else if (linePos == line.length) bias = -1
  let from = linePos, to = linePos
  if (bias < 0) from = findClusterBreak(line.text, linePos, false)
  else to = findClusterBreak(line.text, linePos)
  let cat = categorize(line.text.slice(from, to))
  while (from > 0) {
    let prev = findClusterBreak(line.text, from, false)
    if (categorize(line.text.slice(prev, from)) != cat) break
    from = prev
  }
  while (to < line.length) {
    let next = findClusterBreak(line.text, to)
    if (categorize(line.text.slice(to, next)) != cat) break
    to = next
  }
  return EditorSelection.undirectionalRange(from + line.from, to + line.from)
}

// from @codemirror/view src/rectangular-selection.ts
const MaxOff = 2000
function rectangleFor(state, a, b) {
  let startLine = Math.min(a.line, b.line), endLine = Math.max(a.line, b.line)
  let ranges = []
  if (a.off > MaxOff || b.off > MaxOff || a.col < 0 || b.col < 0) {
    let startOff = Math.min(a.off, b.off), endOff = Math.max(a.off, b.off)
    for (let i = startLine; i <= endLine; i++) {
      let line = state.doc.line(i)
      if (line.length <= endOff)
        ranges.push(EditorSelection.range(line.from + startOff, line.to + endOff))
    }
  } else {
    let startCol = Math.min(a.col, b.col), endCol = Math.max(a.col, b.col)
    for (let i = startLine; i <= endLine; i++) {
      let line = state.doc.line(i)
      let start = findColumn(line.text, startCol, state.tabSize, true)
      if (start < 0) {
        ranges.push(EditorSelection.cursor(line.to))
      } else {
        let end = findColumn(line.text, endCol, state.tabSize)
        ranges.push(EditorSelection.range(line.from + start, line.from + end))
      }
    }
  }
  return ranges
}

// from @codemirror/view src/input.ts, without the clipboard filters
function copiedRange(state) {
  let content = [], ranges = [], linewise = false
  for (let range of state.selection.ranges) if (!range.empty) {
    content.push(state.sliceDoc(range.from, range.to))
    ranges.push(range)
  }
  if (!content.length) {
    let upto = -1
    for (let {from} of state.selection.ranges) {
      let line = state.doc.lineAt(from)
      if (line.number > upto) {
        content.push(line.text)
        ranges.push({from: line.from, to: Math.min(state.doc.length, line.to + 1)})
      }
      upto = line.number
    }
    linewise = true
  }
  return {text: content.join(state.lineBreak), ranges, linewise}
}

// from @codemirror/view src/input.ts, returning the spec it dispatched
function doPaste(state, input, lastLinewiseCopy) {
  let changes, i = 1, text = state.toText(input)
  let byLine = text.lines == state.selection.ranges.length
  let linewise = lastLinewiseCopy != null && state.selection.ranges.every(r => r.empty) && lastLinewiseCopy == text.toString()
  if (linewise) {
    let lastLine = -1
    changes = state.changeByRange(range => {
      let line = state.doc.lineAt(range.from)
      if (line.from == lastLine) return {range}
      lastLine = line.from
      let insert = state.toText((byLine ? text.line(i++).text : input) + state.lineBreak)
      return {changes: {from: line.from, insert},
              range: EditorSelection.cursor(range.from + insert.length, -1)}
    })
  } else if (byLine) {
    changes = state.changeByRange(range => {
      let line = text.line(i++)
      return {changes: {from: range.from, to: range.to, insert: line.text},
              range: EditorSelection.cursor(range.from + line.length, -1)}
    })
  } else {
    changes = state.replaceSelection(text)
  }
  return state.update(changes, {userEvent: "input.paste", scrollIntoView: true})
}

// from @codemirror/view src/domchange.ts, without atomic ranges and with no
// DOM node to find a composition in
function applyDefaultInsert(startState, change, newSel, composing) {
  let tr, sel = startState.selection.main
  if (change.from >= sel.from && change.to <= sel.to && change.to - change.from >= (sel.to - sel.from) / 3 &&
      (!newSel || newSel.main.empty && newSel.main.from == change.from + change.insert.length) &&
      composing < 0) {
    let before = sel.from < change.from ? startState.sliceDoc(sel.from, change.from) : ""
    let after = sel.to > change.to ? startState.sliceDoc(change.to, sel.to) : ""
    tr = startState.replaceSelection(startState.toText(
      before + change.insert.sliceString(0, undefined, startState.lineBreak) + after))
  } else {
    let changes = startState.changes(change)
    let mainSel = newSel && newSel.main.to <= changes.newLength ? newSel.main : undefined
    if (startState.selection.ranges.length > 1 && composing >= 0 &&
        change.to <= sel.to + 10 && change.to >= sel.to - 10) {
      let replaced = startState.sliceDoc(change.from, change.to)
      let compositionRange = startState.doc.lineAt(sel.head)
      let offset = sel.to - change.to
      tr = startState.changeByRange(range => {
        if (range.from == sel.from && range.to == sel.to)
          return {changes, range: mainSel || range.map(changes)}
        let to = range.to - offset, from = to - replaced.length
        if (startState.sliceDoc(from, to) != replaced ||
            to >= compositionRange.from && from <= compositionRange.to)
          return {range}
        let rangeChanges = startState.changes({from, to, insert: change.insert}), selOff = range.to - sel.to
        return {
          changes: rangeChanges,
          range: !mainSel ? range.map(rangeChanges) :
            EditorSelection.range(Math.max(0, mainSel.anchor + selOff), Math.max(0, mainSel.head + selOff))
        }
      })
    } else {
      tr = {changes, selection: mainSel && startState.selection.replaceRange(mainSel)}
    }
  }
  let userEvent = "input.type"
  if (composing >= 0) userEvent += ".compose"
  return startState.update(tr, {userEvent, scrollIntoView: true})
}

let seed = 0xc0de;
function random() {
  seed = (seed + 0x6d2b79f5) | 0;
  let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
}
const r = (n) => Math.floor(random() * n);
const pick = (list) => list[r(list.length)];
const pieces = ["a", "b", "x", "Z", "_", "1", " ", " ", "  ", "\t", ".", ",", "(", ")", "{", "}", "[", "]",
  "é", "é", "世界", "👍", "🇧🇷", "👨‍👩", " ", "❤️", "\n", "\n", "\n  ", "\n\t"];
const rDoc = () => Array.from({ length: r(14) }, () => pick(pieces)).join("");

// boundaries are the positions between code points, where Go can point.
function boundaries(doc) {
  let out = [0];
  for (let i = 0; i < doc.length; ) {
    i += doc.codePointAt(i) > 0xffff ? 2 : 1;
    out.push(i);
  }
  return out;
}

function rSelection(doc, multiple) {
  let b = boundaries(doc), n = multiple ? r(3) + 1 : 1;
  let ranges = Array.from({ length: n }, () => {
    let a = pick(b);
    return r(2) ? EditorSelection.cursor(a, pick([-1, 0, 1])) : EditorSelection.range(a, pick(b));
  });
  return EditorSelection.create(ranges, r(ranges.length));
}

const sel = (s) => ({ ranges: s.ranges.map((r) => [r.anchor, r.head, r.assoc, r.goalColumn ?? -1]), main: s.mainIndex });
const result = (tr) => tr ? { doc: tr.state.doc.toString(), sel: sel(tr.state.selection), event: tr.annotation(tr.constructor.userEvent) ?? "" } : null;

function run(state, command) {
  let got = null;
  let ok = command({ state, dispatch: (tr) => (got = tr) });
  return ok ? result(got) : null;
}

const stateCommands = ["deleteCharBackward", "deleteCharForward", "deleteGroupBackward", "deleteGroupForward",
  "insertNewlineAndIndent", "selectAll", "cursorDocStart", "cursorDocEnd", "selectDocStart", "selectDocEnd",
  "simplifySelection"];

function moveCommand(state, forward, group, select) {
  let move = (range) => group
    ? moveByChar(state, range, forward, (initial) => byGroup(state, range.head, initial))
    : moveByChar(state, range, forward);
  let how = select ? move : (range) => range.empty ? move(range) : EditorSelection.cursor(forward ? range.to : range.from);
  let selection = EditorSelection.create(state.selection.ranges.map((range) => {
    if (!select) return how(range);
    let head = how(range);
    return EditorSelection.range(range.anchor, head.head, head.goalColumn, head.bidiLevel || undefined, head.assoc);
  }), state.selection.mainIndex);
  if (selection.eq(state.selection, !select)) return null;
  return result(state.update({ selection, scrollIntoView: true, userEvent: "select" }));
}

let cases = [];
for (let i = 0; i < 100; i++) {
  let doc = rDoc(), multiple = r(3) > 0, unit = pick(["  ", "\t", "    "]);
  let extensions = [indentUnit.of(unit)];
  if (multiple) extensions.push(EditorState.allowMultipleSelections.of(true));
  let state = EditorState.create({ doc, selection: rSelection(doc, multiple), extensions });
  let c = { doc, multiple, indentUnit: unit, sel: sel(state.selection), commands: {} };
  for (let name of stateCommands) c.commands[name] = run(state, commands[name]);
  for (let forward of [false, true]) for (let group of [false, true]) for (let select of [false, true])
    c.commands[`move ${forward} ${group} ${select}`] = moveCommand(state, forward, group, select);
  let b = boundaries(doc), at = pick(b);
  c.groupAt = { pos: at, bias: pick([-1, 1]) };
  let g = groupAt(state, at, c.groupAt.bias);
  c.groupAt.range = [g.from, g.to];
  let corner = () => {
    let line = state.doc.line(r(state.doc.lines) + 1), bs = boundaries(line.text), off = pick(bs);
    let col = r(3) == 0 ? countColumn(line.text, state.tabSize) + r(4) : countColumn(line.text, state.tabSize, off);
    return { line: line.number, col, off };
  };
  let ra = corner(), rb = corner();
  c.rect = { a: ra, b: rb, ranges: rectangleFor(state, ra, rb).map((r) => [r.anchor, r.head]) };
  let copied = copiedRange(state);
  c.copied = { text: copied.text, ranges: copied.ranges.map((r) => [r.from, r.to]), linewise: copied.linewise };
  c.paste = { input: pick(["x", "a\nb", "one\ntwo\nthree", copied.text, "\t"]) };
  c.paste.linewiseCopy = r(2) ? copied.linewise ? copied.text : null : c.paste.input;
  c.paste.result = result(doPaste(state, c.paste.input, c.paste.linewiseCopy));
  let main = state.selection.main, from = r(2) ? main.from : pick(b), to = Math.max(from, r(2) ? main.to : pick(b));
  let insert = pick(["q", "", "é", "\n", "ab", "世"]), composing = -1;
  // A composition replaces what it composed so far, at the main cursor.
  if (r(4) == 0 && state.selection.ranges.every((r) => r.to >= main.to - main.from)) {
    from = main.from, to = main.to, composing = 0;
  }
  let newSel = r(3) ? EditorSelection.single(from + insert.length) : null;
  c.input = { from, to, insert, newSel: newSel ? newSel.main.head : -1, composing };
  try {
    c.input.result = result(applyDefaultInsert(state, { from, to, insert: Text.of(insert.split("\n")) }, newSel, composing));
  } catch (e) {
    // CodeMirror refuses what it made: a selection outside the document.
    c.input.result = { error: String(e.message) };
  }
  cases.push(c);
}

// searchCases select occurrences in documents of few words, repeated. No
// word changes under NFKD, which SearchCursor matches through and the port
// does not (docs/ceilings.md): "e" would select the "e" "é" decomposes to.
let searchCases = [];
const words = ["one", "two", "onetwo", "ne", "x_1", "世界"];
const seps = [" ", " ", "\n", ".", ""];
for (let i = 0; i < 100; i++) {
  let doc = Array.from({ length: r(10) + 1 }, () => pick(words) + pick(seps)).join("");
  let b = boundaries(doc), n = r(3) + 1, ranges = [];
  let start = pick(b);
  let len = pick([0, 1, 2, 3, 6]);
  for (let k = 0; k < n; k++) {
    let at = k == 0 ? start : pick(b);
    let end = b.find((p) => p >= at + len) ?? doc.length;
    ranges.push(EditorSelection.range(at, r(4) == 0 ? at : end));
  }
  let state = EditorState.create({ doc, selection: EditorSelection.create(ranges, r(ranges.length)),
    extensions: EditorState.allowMultipleSelections.of(true) });
  searchCases.push({ doc, sel: sel(state.selection),
    next: run(state, search.selectNextOccurrence), matches: run(state, search.selectSelectionMatches) });
}

const versions = Object.fromEntries(["@codemirror/state", "@codemirror/view", "@codemirror/commands", "@codemirror/language", "@codemirror/search"]
  .map((p) => [p, packageJSON(modulesDir, p).version]));
writeFileSync(out, `{"versions": ${JSON.stringify(versions)}, "cases": [\n${cases.map((c) => JSON.stringify(c)).join(",\n")}\n],
"searchCases": [\n${searchCases.map((c) => JSON.stringify(c)).join(",\n")}\n]}\n`);
