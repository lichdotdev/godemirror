// gen.mjs writes lsp/servers' catalog from nvim-lspconfig's lsp/*.lua
// configs (evaluated by dump.lua under nvim -l) and mason-registry's
// registry.json, both at the versions pinned below. Run it, from anywhere,
// with git, unzip, nvim and gofmt on PATH (or go generate ./lsp/servers):
//
//	node internal/serversgen/gen.mjs
//
// Moving a pin is editing LSPCONFIG_COMMIT or MASON_TAG and running it again.

import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const LSPCONFIG_COMMIT = "76f4cafc49677a34dc8ad8142b773d78ab4a041e";
const MASON_TAG = "2026-11-26-scared-trunk";
const SUPPORTED = new Set(["npm", "pypi", "golang", "cargo", "gem", "github"]);

const here = dirname(fileURLToPath(import.meta.url));
const OUT = join(here, "../../lsp/servers");
function fetchLspconfig(dir) {
  const git = (...args) => execFileSync("git", ["-C", dir, ...args], { stdio: "inherit" });
  execFileSync("git", ["init", "-q", dir]);
  git("fetch", "-q", "--depth", "1", "https://github.com/neovim/nvim-lspconfig", LSPCONFIG_COMMIT);
  git("checkout", "-q", "FETCH_HEAD");
  return dir;
}

// nvim runs twice, each time with a bare environment and its own $HOME,
// XDG dirs and cwd inside work. dump.lua catches what came from those; a
// field the two runs disagree on (a pid, a clock) is computed as well.
function dumpConfigs(lspconfig, work) {
  const [a, b] = ["a", "b"].map((run) => JSON.parse(runDump(lspconfig, join(work, run))));
  b.configs.forEach((other, i) => {
    const c = a.configs[i];
    for (const field of Object.keys(c)) {
      if (field === "dynamic" || same(c[field], other[field])) continue;
      delete c[field];
      c.dynamic = [...c.dynamic, field].sort();
    }
  });
  return a;
}

function same(a, b) {
  return JSON.stringify(sortKeys(a)) === JSON.stringify(sortKeys(b));
}

function runDump(lspconfig, dir) {
  const home = join(dir, "home");
  mkdirSync(home, { recursive: true });
  const env = { PATH: process.env.PATH, HOME: home };
  for (const d of ["CONFIG", "DATA", "CACHE", "STATE"]) env[`XDG_${d}_HOME`] = join(home, d.toLowerCase());
  return execFileSync("nvim", ["-l", join(here, "dump.lua"), lspconfig], {
    cwd: dir,
    env,
    maxBuffer: 64 << 20,
    encoding: "utf8",
  });
}

async function fetchRegistry(dir) {
  const url = `https://github.com/mason-org/mason-registry/releases/download/${MASON_TAG}/registry.json.zip`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: ${res.status}`);
  mkdirSync(dir, { recursive: true });
  const zip = join(dir, "registry.json.zip");
  writeFileSync(zip, Buffer.from(await res.arrayBuffer()));
  execFileSync("unzip", ["-q", "-o", zip, "registry.json", "-d", dir]);
  return JSON.parse(readFileSync(join(dir, "registry.json"), "utf8"));
}

// A purl is pkg:type/namespace/name@version?qualifiers#subpath. Mason
// leaves a scope's @ unencoded (pkg:npm/@biomejs/biome@2.6.8), so the
// version is after the last @ of the name.
function parsePurl(id) {
  const m = /^pkg:([^/]+)\/([^?#]+)(?:\?([^#]*))?(?:#(.*))?$/.exec(id);
  if (!m) throw new Error(`bad purl ${id}`);
  const [, kind, path, query = "", subpath] = m;
  const at = path.lastIndexOf("@");
  const hasVersion = at > path.lastIndexOf("/");
  let pkg = decodeURIComponent(hasVersion ? path.slice(0, at) : path);
  if (subpath) pkg += "/" + subpath;
  const version = hasVersion ? decodeURIComponent(path.slice(at + 1)) : "";
  return { kind, pkg, version, qualifiers: Object.fromEntries(new URLSearchParams(query)) };
}

// Mason templates seen in assets: {{version}}, {{ version | strip_prefix "v" }}
// and {{source.asset.<key>}}, itself expanded. Anything else leaves the value unresolved (null).
function expand(s, version, asset) {
  let ok = true;
  const out = s.replace(/\{\{\s*([^}]*?)\s*\}\}/g, (_, expr) => {
    const strip = /^version\s*\|\s*strip_prefix\s+"([^"]*)"$/.exec(expr);
    if (expr === "version") return version;
    if (strip) return version.startsWith(strip[1]) ? version.slice(strip[1].length) : version;
    const ref = /^source\.asset\.([\w.]+)$/.exec(expr);
    const v = ref && ref[1].split(".").reduce((o, k) => o?.[k], asset);
    const expanded = typeof v === "string" ? expand(v, version, asset) : null;
    if (expanded != null) return expanded;
    ok = false;
    return "";
  });
  return ok && !out.includes("{{") ? out : null;
}

function githubAssets(pkg, version) {
  const out = [];
  for (const asset of [pkg.source.asset].flat()) {
    if (!asset || asset.target == null) continue;
    const files = [asset.file].flat().map((f) => expand(f, version, asset));
    const bins = Object.entries(pkg.bin ?? {}).map(([name, v]) => [name, expand(v, version, asset)]);
    if (files.includes(null) || bins.some(([, v]) => v == null)) continue;
    for (const target of [asset.target].flat()) out.push({ target, files, bins: Object.fromEntries(bins) });
  }
  return out;
}

function recipe(pkg, unsupported) {
  const p = parsePurl(pkg.source.id);
  const reason = !SUPPORTED.has(p.kind) ? p.kind : p.kind === "github" && !pkg.source.asset ? "github build" : null;
  if (reason) {
    (unsupported[reason] ??= []).push(pkg.name);
    return null;
  }
  const install = {
    mason: pkg.name,
    kind: p.kind,
    pkg: p.pkg,
    version: p.version,
    qualifiers: p.qualifiers,
    extra: pkg.source.extra_packages ?? [],
    bins: Object.keys(pkg.bin ?? {}).sort(),
    assets: [],
  };
  if (p.kind === "github") {
    install.assets = githubAssets(pkg, p.version);
    if (install.assets.length === 0) {
      (unsupported["github templates"] ??= []).push(pkg.name);
      return null;
    }
  }
  return install;
}

function installsByLspconfig(registry, unsupported) {
  const out = {};
  const sorted = [...registry].sort((a, b) => !!a.deprecation - !!b.deprecation || a.name.localeCompare(b.name));
  for (const pkg of sorted) {
    const name = pkg.neovim?.lspconfig;
    if (!name || out[name]) continue;
    const install = recipe(pkg, unsupported);
    if (install) out[name] = install;
  }
  return out;
}

const str = (s) => JSON.stringify(s);
const strs = (a) => `[]string{${a.map(str).join(", ")}}`;
const strMap = (o) => `map[string]string{${Object.entries(o).map(([k, v]) => `${str(k)}: ${str(v)}`).join(", ")}}`;
// Lua tables have no key order, so objects are written with sorted keys
// for a rerun to write the same file.
const sortKeys = (v) =>
  Array.isArray(v)
    ? v.map(sortKeys)
    : v && typeof v === "object"
      ? Object.fromEntries(Object.keys(v).sort().map((k) => [k, sortKeys(v[k])]))
      : v;
const raw = (v) => {
  if (v == null) return null;
  const s = JSON.stringify(sortKeys(v));
  return s === "{}" || s === "[]" ? null : `json.RawMessage(${str(s)})`;
};

function rootMarkers(markers) {
  if (!Array.isArray(markers) || markers.length === 0) return null;
  return `[][]string{${markers.map((m) => strs([m].flat()).slice("[]string".length)).join(", ")}}`;
}

function goInstall(i) {
  const kind = `Kind${i.kind[0].toUpperCase()}${i.kind.slice(1)}`;
  const lines = [`Mason: ${str(i.mason)}`, `Kind: ${kind}`, `Package: ${str(i.pkg)}`, `Version: ${str(i.version)}`];
  if (Object.keys(i.qualifiers).length) lines.push(`Qualifiers: ${strMap(i.qualifiers)}`);
  if (i.extra.length) lines.push(`ExtraPackages: ${strs(i.extra)}`);
  if (i.bins.length) lines.push(`Bins: ${strs(i.bins)}`);
  if (i.assets.length) {
    const assets = i.assets.map((a) => `{Target: ${str(a.target)}, Files: ${strs(a.files)}, Bins: ${strMap(a.bins)}},`);
    lines.push(`Assets: []Asset{\n${assets.join("\n")}\n}`);
  }
  return `&Install{\n${lines.join(",\n")},\n}`;
}

function goServer(c, install) {
  const lines = [`Name: ${str(c.name)}`];
  if (Array.isArray(c.cmd) && c.cmd.length) lines.push(`Command: ${strs(c.cmd)}`);
  if (c.filetypes?.length) lines.push(`FileTypes: ${strs(c.filetypes)}`);
  const markers = rootMarkers(c.root_markers);
  if (markers) lines.push(`RootMarkers: ${markers}`);
  const init = raw(c.init_options);
  if (init) lines.push(`InitOptions: ${init}`);
  const settings = raw(c.settings);
  if (settings) lines.push(`Settings: ${settings}`);
  if (c.dynamic.length) lines.push(`Dynamic: ${strs(c.dynamic)}`);
  if (install) lines.push(`Install: ${goInstall(install)}`);
  return `{\n${lines.join(",\n")},\n},`;
}

function header(nvim) {
  return `// Code generated by internal/serversgen; DO NOT EDIT.

// Derived from, and modified from (converted to Go), two Apache-2.0
// sources: nvim-lspconfig (https://github.com/neovim/nvim-lspconfig) at
// commit ${LSPCONFIG_COMMIT}, and mason-registry
// (https://github.com/mason-org/mason-registry) release ${MASON_TAG};
// evaluated, and filetypes taken, with Neovim ${nvim}.
// See LICENSE.apache and NOTICE.

package servers
`;
}

function writeCatalog(dump, installs) {
  const body = dump.configs.map((c) => goServer(c, installs[c.name])).join("\n");
  const src = `${header(dump.nvim)}
import "encoding/json"

var catalog = []Server{
${body}
}
`;
  writeFileSync(join(OUT, "catalog_generated.go"), src);
}

// Only the filetypes some server claims, and only basenames: Lookup
// matches a file's name, not its directory.
function writeFileTypes(dump) {
  const used = new Set(dump.configs.flatMap((c) => c.filetypes ?? []));
  const table = (o) =>
    Object.entries(o)
      .filter(([k, ft]) => used.has(ft) && !k.includes("/"))
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
      .map(([k, ft]) => `\t${str(k)}: ${str(ft)},`)
      .join("\n");
  const src = `${header(dump.nvim)}
var extensionFileTypes = map[string]string{
${table(dump.extension)}
}

var filenameFileTypes = map[string]string{
${table(dump.filename)}
}
`;
  writeFileSync(join(OUT, "filetypes_generated.go"), src);
}

function summarize(configs, installs, unsupported) {
  const kinds = {};
  for (const c of configs) {
    const k = installs[c.name]?.kind;
    if (k) kinds[k] = (kinds[k] ?? 0) + 1;
  }
  console.log(`servers: ${configs.length}`);
  console.log(`dynamic: ${configs.filter((c) => c.dynamic.length).length}`);
  console.log(`no plain cmd: ${configs.filter((c) => !Array.isArray(c.cmd)).length}`);
  console.log(`install recipes: ${Object.values(kinds).reduce((a, b) => a + b, 0)}`, kinds);
  const orphans = Object.keys(installs).filter((n) => !configs.some((c) => c.name === n));
  if (orphans.length) console.log(`mason names with no lspconfig config: ${orphans.join(", ")}`);
  for (const [reason, names] of Object.entries(unsupported)) {
    console.log(`unsupported ${reason} (${names.length}): ${names.join(", ")}`);
  }
}

async function main() {
  const work = mkdtempSync(join(tmpdir(), "serversgen-"));
  try {
    const lspconfig = fetchLspconfig(join(work, "nvim-lspconfig"));
    const registry = await fetchRegistry(join(work, "mason"));
    const dump = dumpConfigs(lspconfig, work);
    const unsupported = {};
    const installs = installsByLspconfig(registry, unsupported);
    writeCatalog(dump, installs);
    writeFileTypes(dump);
    execFileSync("gofmt", ["-w", join(OUT, "catalog_generated.go"), join(OUT, "filetypes_generated.go")]);
    summarize(dump.configs, installs, unsupported);
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
}

await main();
