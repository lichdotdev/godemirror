// golden.mjs writes, for every fixture of a stream mode, what CodeMirror
// makes of it: the mode run through @codemirror/language's StreamLanguage
// and the tree that parse builds highlighted by @lezer/highlight's
// highlightTree, for the Go port's tests to match. A dev tool, like
// internal/lezer/gen, run against a node_modules directory outside the
// repository holding @codemirror/language, @codemirror/legacy-modes and
// @lezer/highlight:
//
//	node internal/stream/gen/golden.mjs "$NODE_MODULES" clike#kotlin internal/stream/kotlin/testdata
//
// The mode is <module>#<export>: a module of @codemirror/legacy-modes/mode
// by name, or a path ending in .mjs to a mode of one's own. With a fourth
// argument, an output directory, the goldens go there instead of next to
// the fixtures (for a corpus kept outside the repository).
//
// Every file directly in the directory not ending in .golden is a fixture,
// and gets a <name>.golden: one line per highlighted span, "from to" and
// its tags as Tag.toString prints them. Positions are UTF-16 code units.
// The parse runs with no editor around it, so it covers the whole file, as
// CodeMirror's does once it has parsed past the viewport.

import { mkdirSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { load, packageDir } from "../../lezer/gen/load.mjs";

const [modulesDir, modeSpec, dir, outDir = dir] = process.argv.slice(2);
if (!dir || !modeSpec?.includes("#")) {
  console.error("usage: node golden.mjs <node_modules dir> <module>#<export> <fixtures dir> [<goldens dir>]");
  process.exit(2);
}
const [modeModule, modeName] = modeSpec.split("#");
const modePath = modeModule.endsWith(".mjs")
  ? resolve(modeModule)
  : join(packageDir(modulesDir, "@codemirror/legacy-modes"), "mode", modeModule + ".js");
const mode = (await import(pathToFileURL(modePath).href))[modeName];
if (!mode) throw new Error(`${modePath} exports no ${modeName}`);

const { StreamLanguage } = await load(modulesDir, "@codemirror/language");
const { highlightTree } = await load(modulesDir, "@lezer/highlight");
const parser = StreamLanguage.define(mode).parser;
const everyTag = { style: (tags) => (tags.length ? tags.map(String).join(" ") : null) };

mkdirSync(outDir, { recursive: true });
for (const name of readdirSync(dir).sort()) {
  if (name.endsWith(".golden") || !statSync(join(dir, name)).isFile()) continue;
  const tree = parser.parse(readFileSync(join(dir, name), "utf8"));
  const lines = [];
  highlightTree(tree, everyTag, (from, to, classes) => lines.push(`${from} ${to} ${classes}`));
  writeFileSync(join(outDir, name + ".golden"), lines.map((l) => l + "\n").join(""));
}
