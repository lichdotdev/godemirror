// golden.mjs writes, for every fixture of a grammar, what the real Lezer
// parser and @lezer/highlight's highlightTree make of it, for the Go port's
// tests to match: a dev tool, like gen.mjs, run with the same node_modules
// (which also needs @lezer/highlight):
//
//	node internal/lezer/gen/golden.mjs "$LEZER_NODE_MODULES" @lezer/java internal/lezer/java/testdata
//
// An optional fourth argument is a module of extra styleTags to add to the
// grammar first (see withStyles in load.mjs), the one gen.mjs wrote the
// test grammar with. A leading --dialect=<dialects> parses with those
// dialects enabled, as LRParser.configure's dialect option takes them, and
// a --top=<rule> parses from that top rule instead of the first. A
// --wrap=<module> parses with the parse wrapper the module's default export
// returns, given a function that imports a package from the node_modules
// directory, as LRParser.configure's wrap option takes it.
//
// Every file directly in the directory not ending in .golden is a fixture, and gets
// a <name>.golden next to it: a first line "tree " and the tree as
// Tree.toString prints it, then one line per highlighted span, "from to"
// and its tags as Tag.toString prints them. Positions are UTF-16 code
// units. The highlighter gives every tag its own class, so a span's class
// lists the tags of the node and the ones it inherits, in the order
// highlightTree joins them.

import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
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
  console.error("usage: node golden.mjs [--dialect=<dialects>] [--top=<top rule>] [--wrap=<module>] <node_modules dir> <grammar package> <fixtures dir>");
  process.exit(2);
}
const { highlightTree } = await load(modulesDir, "@lezer/highlight");
let parser = await withStyles(modulesDir, (await load(modulesDir, pkg)).parser, stylesModule);
if (dialect != null) parser = parser.configure({ dialect });
if (top != null) parser = parser.configure({ top });
if (wrapModule != null) {
  const { default: wrap } = await import(pathToFileURL(resolve(wrapModule)).href);
  parser = parser.configure({ wrap: await wrap((name) => load(modulesDir, name)) });
}

const everyTag = { style: (tags) => (tags.length ? tags.map(String).join(" ") : null) };

for (const name of readdirSync(dir).sort()) {
  if (name.endsWith(".golden") || !statSync(join(dir, name)).isFile()) continue;
  const text = readFileSync(join(dir, name), "utf8");
  const tree = parser.parse(text);
  const lines = ["tree " + tree.toString()];
  highlightTree(tree, everyTag, (from, to, classes) => lines.push(`${from} ${to} ${classes}`));
  writeFileSync(join(dir, name + ".golden"), lines.join("\n") + "\n");
}
