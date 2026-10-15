// corpus.mjs writes random edit sessions for incremental.mjs to write the
// goldens of: a dev tool, like gen.mjs, for the randomized corpus the tests
// read from lezertest.CorpusEnv, and for the sessions under the grammars'
// testdata/incremental. Each session's document is the fixtures given,
// picked at random and joined until it is 4500 code units long plus up to
// --spread more (default 14000); then come 3 to 8 steps of edits, the same
// seed always giving the same sessions:
//
//	node internal/lezer/gen/corpus.mjs [--spread=<units>] <out dir> <count> <seed> <fixture files...>
//
// writes <out dir>/session-NNN.json. An edit inserts, deletes or replaces,
// at the start, the end, near a quote, comment, brace or line break, or
// anywhere, with tokens of many languages or a piece of the document
// itself, so edits land inside strings and comments and break the syntax
// as often as they keep it. A step is sometimes two or three edits, and
// sometimes undoes the step before it, repairing what it broke.

import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

let args = process.argv.slice(2), spread = 14000;
if (args[0]?.startsWith("--spread=")) spread = +args.shift().slice("--spread=".length);
const [outDir, countArg, seedArg, ...fixtures] = args;
if (!fixtures.length) {
  console.error("usage: node corpus.mjs [--spread=<units>] <out dir> <count> <seed> <fixture files...>");
  process.exit(2);
}

// xorshift32, for sessions that do not change with the JavaScript engine.
let seed = +seedArg >>> 0 || 1;
const rnd = () => {
  seed ^= seed << 13; seed >>>= 0;
  seed ^= seed >>> 17;
  seed ^= seed << 5; seed >>>= 0;
  return seed / 4294967296;
};
const int = (n) => Math.floor(rnd() * n);
const pick = (a) => a[int(a.length)];
const surrogate = /[\ud800-\udfff]/;

const texts = fixtures.map((f) => readFileSync(f, "utf8")).filter((t) => t.length > 0);
const pool = ["{", "}", "(", ")", "[", "]", ";", "\n", "\n    ", "\n\n", " ", "  ", "\t", "\"", "'", "`", "/*", "*/", "//", "#",
  "<", ">", "</", "/>", "=", ",", ".", ":", "x", "foo", "bar1", "42", "3.14", "0x1F", "if", "else", "return", "class",
  "function", "def", "fn", "let", "var", "const", "import", "public", "static", "void", "int", "@", "$", "\\", "<!--",
  "-->", "<script>", "</script>", "<style>", "</style>", "<div class=\"a\">", "</div>", "color: red;", "@media",
  "{{", "\"\"\"", "r#\"", "'a'", "->", "=>", "::", " = ", "() ", "f\"", "${", "*", "+", "-", "!", "?", "&&", "||",
  "élan", "日本", "async", "await", "yield", "match", "impl", "struct", "pub", "go", "chan", "select", "lambda", "pass",
  "@Override", "new ", "this.", "self.", "  # c\n", "// c\n", "/* c */", "<!-- c -->", "<p>", "</p>", "<b>", "</b>", "&amp;"];

// snippet is text to insert: a piece of the document, or a few tokens. It
// never holds half a surrogate pair, which JSON could not carry to Go.
function snippet(text) {
  if (rnd() < 0.3 && text.length > 2) {
    const from = int(text.length - 1), len = 1 + int(rnd() < 0.8 ? 40 : 400);
    return text.slice(from, from + len).replace(new RegExp(surrogate, "g"), "");
  }
  let s = "";
  for (let n = 1 + int(rnd() < 0.7 ? 2 : 8); n > 0; n--) s += pick(pool);
  return s;
}

function position(text) {
  const r = rnd();
  if (r < 0.1) return Math.min(text.length, int(10));
  if (r < 0.2) return Math.max(0, text.length - int(10));
  if (r < 0.45) {
    const re = /["'`#]|\/\*|\/\/|<!--|\{|\n/g, hits = [];
    for (let m; (m = re.exec(text)) && hits.length < 2000;) hits.push(m.index);
    if (hits.length) return Math.min(text.length, pick(hits) + int(3));
  }
  return int(text.length + 1);
}

function edit(text) {
  const from = position(text), r = rnd();
  const deleted = 1 + int(rnd() < 0.7 ? 5 : rnd() < 0.8 ? 80 : 800);
  if (r < 0.4) return { from, to: from, insert: snippet(text) };
  const to = Math.min(text.length, from + deleted);
  if (r < 0.7) return { from, to, insert: "" };
  return { from, to, insert: snippet(text) };
}

function apply(text, edits) {
  let out = "", at = 0;
  for (const e of edits) {
    out += text.slice(at, e.from) + e.insert;
    at = e.to;
  }
  return out + text.slice(at);
}

// step is the edits of the next step: the undoing of a one-edit step
// before it, or one to three edits that do not overlap, in order.
function step(text, prev, prevText) {
  if (prev && prev.length == 1 && rnd() < 0.2) {
    const e = prev[0], removed = prevText.slice(e.from, e.to);
    if (!surrogate.test(removed)) return [{ from: e.from, to: e.from + e.insert.length, insert: removed }];
  }
  const n = rnd() < 0.8 ? 1 : 2 + int(2), edits = [];
  for (let i = 0; i < n; i++) edits.push(edit(text));
  edits.sort((a, b) => a.from - b.from || a.to - b.to);
  const kept = [];
  for (const e of edits) {
    const last = kept[kept.length - 1];
    if (!last || (e.from >= last.to && e.from > last.from)) kept.push(e);
  }
  return kept;
}

mkdirSync(outDir, { recursive: true });
for (let s = 0; s < +countArg; s++) {
  const target = 4500 + int(spread);
  let text = "";
  while (text.length < target) text += pick(texts) + (rnd() < 0.5 ? "\n" : "");
  const session = { text, steps: [] };
  let prev = null, prevText = null;
  for (let n = 3 + int(6); n > 0; n--) {
    const edits = step(text, prev, prevText);
    session.steps.push(edits);
    prevText = text;
    text = apply(text, edits);
    prev = edits;
  }
  writeFileSync(join(outDir, `session-${String(s).padStart(3, "0")}.json`), JSON.stringify(session));
}
