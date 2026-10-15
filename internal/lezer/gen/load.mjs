// load.mjs imports npm packages from a node_modules directory outside the
// repository, by their ES module entry, for gen.mjs and golden.mjs.

import { readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

// packageDir is the directory of pkg inside modulesDir.
export function packageDir(modulesDir, pkg) {
  return join(modulesDir, ...pkg.split("/"));
}

// packageJSON is pkg's parsed package.json.
export function packageJSON(modulesDir, pkg) {
  return JSON.parse(readFileSync(join(packageDir(modulesDir, pkg), "package.json"), "utf8"));
}

// entryPath is the file pkg's ES module entry point resolves to.
export function entryPath(modulesDir, pkg) {
  const meta = packageJSON(modulesDir, pkg);
  const entry = meta.exports?.import ?? meta.module ?? meta.main;
  if (typeof entry != "string") throw new Error(`cannot find the ES module entry of ${pkg}`);
  return join(packageDir(modulesDir, pkg), entry);
}

// load imports pkg's ES module entry.
export function load(modulesDir, pkg) {
  return import(pathToFileURL(entryPath(modulesDir, pkg)).href);
}

// withStyles is parser with the extra styleTags a styles module adds, for
// tests of rule kinds a grammar's own styles do not use. The module's
// default export takes @lezer/highlight and returns the props to add. No
// module leaves the parser as it is.
export async function withStyles(modulesDir, parser, stylesModule) {
  if (!stylesModule) return parser;
  const highlight = await load(modulesDir, "@lezer/highlight");
  const { default: props } = await import(pathToFileURL(resolve(stylesModule)).href);
  return parser.configure({ props: props(highlight) });
}
