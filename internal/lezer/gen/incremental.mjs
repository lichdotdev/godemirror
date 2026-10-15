// incremental.mjs writes, for every edit session of a grammar, what the real
// Lezer makes of it parsing incrementally, for the Go port's tests to match:
// a dev tool, like golden.mjs, run with the same node_modules and the same
// options (--dialect, --top, --wrap, and a styles module):
//
//	node internal/lezer/gen/incremental.mjs "$LEZER_NODE_MODULES" @lezer/java internal/lezer/java/testdata/incremental
//
// Every <name>.json directly in the directory is a session, {"text": ...,
// "steps": [[{"from", "to", "insert"}, ...], ...]}: a document and the
// edits made to it, each step a list of edits in order that do not
// overlap, their positions UTF-16 offsets into the document as it was
// before the step. It gets a <name>.json.golden next to it with a line per
// parse, the first of the document as it is and then one per step: the
// step's number and the SHA-256, in hex, of what the parse gave, the tree as
// Tree.toString prints it, its layout (see layout below) and its spans,
// written as golden.mjs writes them, one per line, then of the spans of a
// fresh parse of the same text, which Lezer does not promise are the
// incremental parse's (a reused node keeps what was nested in it, and error
// recovery may go another way). A step parses the edited
// document with the fragments TreeFragment.applyChanges leaves of the
// previous tree's TreeFragment.addTree, as an editor would.

import { createHash } from "node:crypto";
import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { load, withStyles } from "./load.mjs";

let args = process.argv.slice(2), dialect = null, top = null, wrapModule = null;
for (;;) {
  if (args[0]?.startsWith("--dialect=")) dialect = args.shift().slice("--dialect=".length);
  else if (args[0]?.startsWith("--top=")) top = args.shift().slice("--top=".length);
  else if (args[0]?.startsWith("--wrap=")) wrapModule = args.shift().slice("--wrap=".length);
  else break;
}
const [modulesDir, pkg, dir, stylesModule] = args;
if (!dir) {
  console.error("usage: node incremental.mjs [--dialect=<dialects>] [--top=<top rule>] [--wrap=<module>] <node_modules dir> <grammar package> <sessions dir>");
  process.exit(2);
}
const { highlightTree } = await load(modulesDir, "@lezer/highlight");
const { Tree, NodeProp, TreeFragment } = await load(modulesDir, "@lezer/common");
let parser = await withStyles(modulesDir, (await load(modulesDir, pkg)).parser, stylesModule);
if (dialect != null) parser = parser.configure({ dialect });
if (top != null) parser = parser.configure({ top });
if (wrapModule != null) {
  const { default: wrap } = await import(pathToFileURL(resolve(wrapModule)).href);
  parser = parser.configure({ wrap: await wrap((name) => load(modulesDir, name)) });
}

const everyTag = { style: (tags) => (tags.length ? tags.map(String).join(" ") : null) };

const sha = (s) => createHash("sha256").update(s, "utf8").digest("hex");

// layout is how the tree is stored, which a reused node keeps from the tree
// it came from: every Tree as its type id and length, its contextHash (#)
// and lookAhead (~) props, a mounted tree in braces after its overlay
// ranges, and its children in brackets, each after its position, a
// TreeBuffer as B<length>/<buffer size>.
function layout(tree) {
  const out = [];
  const walk = (t) => {
    out.push(t.type.id + ":" + t.length);
    const cx = t.prop(NodeProp.contextHash), la = t.prop(NodeProp.lookAhead);
    if (cx) out.push("#" + cx);
    if (la) out.push("~" + la);
    const m = t.prop(NodeProp.mounted);
    if (m) {
      out.push("{" + (m.overlay ? m.overlay.map((r) => r.from + "-" + r.to).join(",") : "") + "|");
      walk(m.tree);
      out.push("}");
    }
    out.push("[");
    t.children.forEach((c, i) => {
      out.push(t.positions[i] + "=");
      if (c instanceof Tree) walk(c);
      else out.push("B" + c.length + "/" + c.buffer.length);
      out.push(";");
    });
    out.push("]");
  };
  walk(tree);
  return out.join("");
}

function spans(tree) {
  const lines = [];
  highlightTree(tree, everyTag, (from, to, classes) => lines.push(`${from} ${to} ${classes}`));
  return sha(lines.join("\n"));
}

function describe(step, tree, text) {
  return `${step} ${sha(tree.toString())} ${sha(layout(tree))} ${spans(tree)} ${spans(parser.parse(text))}`;
}

// apply is text with the step's edits made, and the ChangedRanges they are.
function apply(text, edits) {
  let out = "", at = 0, shift = 0;
  const changes = [];
  for (const { from, to, insert } of edits) {
    out += text.slice(at, from) + insert;
    changes.push({ fromA: from, toA: to, fromB: from + shift, toB: from + shift + insert.length });
    shift += insert.length - (to - from);
    at = to;
  }
  return { text: out + text.slice(at), changes };
}

for (const name of readdirSync(dir).sort()) {
  if (!name.endsWith(".json")) continue;
  const session = JSON.parse(readFileSync(join(dir, name), "utf8"));
  let text = session.text, tree = parser.parse(text);
  const lines = [describe(0, tree, text)];
  session.steps.forEach((edits, i) => {
    const next = apply(text, edits);
    const fragments = TreeFragment.applyChanges(TreeFragment.addTree(tree), next.changes);
    text = next.text;
    tree = parser.parse(text, fragments);
    lines.push(describe(i + 1, tree, text));
  });
  writeFileSync(join(dir, name + ".golden"), lines.join("\n") + "\n");
}
