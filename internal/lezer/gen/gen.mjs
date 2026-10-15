// gen.mjs writes a Lezer grammar's parse tables and styleTags rules as a Go
// lezer.Spec, for internal/lezer to parse and highlight with. It is a dev
// tool: nothing runs JavaScript at runtime, and the generated file is
// committed.
//
// It needs node and the grammar's npm package, installed outside the
// repository (npm writes node_modules where it runs):
//
//	cd "$(mktemp -d)" && npm init -y && npm install @lezer/java@1.1.5 @lezer/highlight@1.2.5 @lezer/lr@1.4.11
//	LEZER_NODE_MODULES=<that dir>/node_modules go generate ./internal/lezer/java
//
// which runs, in the grammar's package,
//
//	node ../gen/gen.mjs "$LEZER_NODE_MODULES" @lezer/java java | gofmt > spec_generated.go
//
// Arguments: the node_modules directory, the grammar's package and the Go
// package name to write. With a fourth, a module of extra styleTags (see
// withStyles in load.mjs), it writes only the rules the grammar has with
// them added, as testStyles, for a test of the rule kinds the grammar's own
// styles do not use. The grammar's version is read from its
// package.json and written into the file's header.
//
// The tables are captured by wrapping LRParser.deserialize before the
// grammar loads, so they are written exactly as lezer-generator encoded
// them. External tokenizers, external specializers and the @context
// tracker are JavaScript code: the file names them, and the Go package
// passes their ports in lezer.Externals.

import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { entryPath, load, packageJSON, withStyles } from "./load.mjs";

const [modulesDir, pkg, goPackage, stylesModule] = process.argv.slice(2);
if (!goPackage) {
  console.error("usage: node gen.mjs <node_modules dir> <grammar package> <go package> [<styles module>]");
  process.exit(2);
}
const lr = await load(modulesDir, "@lezer/lr");
const { NodeProp } = await load(modulesDir, "@lezer/common");

let captured = null;
const deserialize = lr.LRParser.deserialize;
lr.LRParser.deserialize = (spec) => {
  if (captured) throw new Error("the grammar package builds more than one parser");
  captured = spec;
  return deserialize(spec);
};
const entry = entryPath(modulesDir, pkg);
const parser = await withStyles(modulesDir, (await load(modulesDir, pkg)).parser, stylesModule);
if (!captured) throw new Error(`${pkg} did not build its parser with LRParser.deserialize`);
const source = readFileSync(entry, "utf8");
const { version } = packageJSON(modulesDir, pkg);

const go = (v) => JSON.stringify(v);
const goInts = (xs) => `[]int{${xs.join(", ")}}`;

// splitTop splits a JavaScript list at its top-level commas.
function splitTop(text) {
  const parts = [];
  let depth = 0, quote = null, cur = "";
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quote) {
      if (c == "\\") cur += c + text[++i];
      else { if (c == quote) quote = null; cur += c; }
      continue;
    }
    if (c == '"' || c == "'") quote = c;
    else if ("([{".includes(c)) depth++;
    else if (")]}".includes(c)) depth--;
    else if (c == "," && depth == 0) { parts.push(cur.trim()); cur = ""; continue; }
    cur += c;
  }
  if (cur.trim()) parts.push(cur.trim());
  return parts;
}

function tokenizers() {
  const m = /tokenizers: \[(.*)\],\n/.exec(source);
  if (!m) throw new Error("no tokenizers list in " + entry);
  const names = splitTop(m[1]);
  return captured.tokenizers.map((t, i) => {
    if (typeof t == "number") return `{Group: ${t}}`;
    if (t instanceof lr.LocalTokenGroup) {
      const data = /^new LocalTokenGroup\((".*?(?<!\\)")/.exec(names[i]);
      if (!data) throw new Error("cannot read local token group " + names[i]);
      const elseToken = t.elseToken == null ? -1 : t.elseToken;
      return `{Local: &lezer.LocalGroupSpec{Data: ${go(JSON.parse(data[1]))}, PrecTable: ${t.precTable}, ElseToken: ${elseToken}}}`;
    }
    if (!/^\w+$/.test(names[i])) throw new Error("cannot name external tokenizer " + names[i]);
    return `{External: ${go(names[i])}}`;
  });
}

// specTables evaluates the module's spec_* object literals, which back its
// @specialize and @extend lookups.
function specTables() {
  const tables = {};
  for (const m of source.matchAll(/const (spec_\w+) = (\{__proto__:null,.*?\});?$/gm))
    tables[m[1]] = new Function("return " + m[2])();
  return tables;
}

function specializers() {
  const tables = specTables();
  return (captured.specialized || []).map((s) => {
    if (s.external) {
      return `{Term: ${s.term}, External: ${go(s.external.name)}, Extend: ${!!s.extend}}`;
    }
    const name = /spec_\w+/.exec(String(s.get));
    if (!name || !tables[name[0]]) throw new Error("cannot find the table of specializer " + s.get);
    const entries = Object.entries(tables[name[0]]).map(([k, v]) => `${go(k)}: ${v}`);
    return `{Term: ${s.term}, Table: map[string]int{${entries.join(", ")}}}`;
  });
}

// nodeProps decodes the spec's compressed nodeProps into prop -> id -> value.
function nodeProps() {
  const out = {};
  for (const propSpec of captured.nodeProps || []) {
    const name = typeof propSpec[0] == "string" ? propSpec[0]
      : Object.keys(NodeProp).find((k) => NodeProp[k] === propSpec[0]);
    const values = (out[name] = out[name] || {});
    for (let i = 1; i < propSpec.length;) {
      const next = propSpec[i++];
      if (next >= 0) { values[next] = String(propSpec[i++]); continue; }
      const value = String(propSpec[i + -next]);
      for (let j = -next; j > 0; j--) values[propSpec[i++]] = value;
      i++;
    }
  }
  return Object.entries(out).map(([name, values]) =>
    `${go(name)}: {${Object.entries(values).map(([id, v]) => `${id}: ${go(v)}`).join(", ")}},`);
}

// modeNames are the Go names of @lezer/highlight's rule modes, by value.
const modeNames = ["ModeOpaque", "ModeInherit", "ModeNormal"];

// styles lists each node type's combined styleTags rules, read back from
// the node set the grammar's propSources extended.
function styles() {
  const lines = [];
  for (const type of parser.nodeSet.types) {
    const rule = Object.values(type.props).find((v) => v && v.constructor && v.constructor.name == "Rule");
    if (!rule) continue;
    const rules = [];
    for (let r = rule; r; r = r.next) {
      const context = r.context ? `, Context: []string{${r.context.map(go).join(", ")}}` : "";
      rules.push(`{Tags: []string{${r.tags.map((t) => go(t.toString())).join(", ")}}, Mode: lezer.${modeNames[r.mode]}${context}}`);
    }
    lines.push(`${type.id}: {${rules.join(", ")}}, // ${type.name}`);
  }
  return lines;
}

// termsBlock is the body of the const declaration lezer-generator writes as
// parser.terms.js: in the module itself where a bundler inlined it, or in a
// parser.terms.js next to it. A grammar with no JavaScript of its own has
// no need for it, and its bundle may leave it out.
function termsBlock() {
  const decl = /generated by lezer-generator\. You probably shouldn't edit it\.\n(?:export )?const\s+([^;]*)/g;
  const isTerms = (body) => /^\s*[\w$]+ = \d+(\s*,\s*[\w$]+ = \d+)*\s*$/.test(body);
  for (const m of source.matchAll(decl)) if (isTerms(m[1])) return m[1];
  const file = join(dirname(entry), "parser.terms.js");
  if (existsSync(file)) {
    for (const m of readFileSync(file, "utf8").matchAll(decl)) if (isTerms(m[1])) return m[1];
  }
  if (!/new ExternalTokenizer|new ContextTracker|external: /.test(source)) return "";
  throw new Error("cannot find the term ids lezer-generator wrote for " + pkg);
}

// goTermName names a parser.terms.js entry in Go: a token (lowercase, as
// lezer-generator names them) as termX, a node as nodeX and a dialect
// (Dialect_x) as dialectX, so a token and a node spelled alike, such as
// HTML's scriptText and ScriptText, stay apart. A bundler suffixes a name
// that clashes with one of the module's own ($1), and lezer-generator
// prefixes a keyword token (_return), neither part of the name.
function goTermName(name) {
  const bare = name.replace(/\$\d+$/, "").replace(/^_+/, "");
  const capitalized = (s) => s.replace(/^./, (c) => c.toUpperCase());
  if (bare.startsWith("Dialect_")) return "dialect" + capitalized(bare.slice("Dialect_".length));
  return (/^[A-Z]/.test(bare) ? "node" : "term") + capitalized(bare);
}

// terms are the grammar's term ids as Go constants.
function terms() {
  const block = termsBlock();
  if (!block) return [];
  return block.split(",").map((entry) => {
    const [name, id] = entry.trim().split(/\s*=\s*/);
    return `${goTermName(name)} = ${id}`;
  });
}

const termConsts = terms();
const termsDecl = termConsts.length == 0 ? "" : `
// Term ids, as lezer-generator numbered them, for the ported external
// tokenizers, specializers and context tracker.
const (
	${termConsts.join("\n\t")}
)
`;

function contextName() {
  if (!captured.context) return "";
  const m = /context: (\w+)/.exec(source);
  if (!m) throw new Error("cannot name the context tracker");
  return m[1];
}

const topRules = Object.entries(captured.topRules).map(([name, [state, term]]) => `{Name: ${go(name)}, State: ${state}, Term: ${term}}`);
const dialects = Object.entries(captured.dialects || {}).map(([name, offset]) => `{Name: ${go(name)}, Offset: ${offset}}`);
const dynamicPrec = Object.entries(captured.dynamicPrecedences || {}).map(([term, prec]) => `${term}: ${prec}`);
for (const key of ["states", "stateData", "goto", "tokenData"])
  if (typeof captured[key] != "string") throw new Error(`${key} is not an encoded string`);

const header = `// Code generated by internal/lezer/gen/gen.mjs from ${pkg} ${version}${stylesModule ? " with " + stylesModule : ""}; DO NOT EDIT.
${pkg.startsWith("@lezer/") ? `// ${pkg} is MIT licensed, Copyright (C) 2018 by Marijn Haverbeke and others.\n` : ""}
package ${goPackage}

import "github.com/lichdotdev/godemirror/internal/lezer"
`;

const testStyles = `${header}
var testStyles = map[int][]lezer.RuleSpec{
	${styles().join("\n\t")}
}
`;

const out = `${header}${termsDecl}
var spec = lezer.Spec{
	Version:         ${captured.version},
	States:          ${go(captured.states)},
	StateData:       ${go(captured.stateData)},
	Goto:            ${go(captured.goto)},
	NodeNames:       ${go(captured.nodeNames)},
	MaxTerm:         ${captured.maxTerm},
	RepeatNodeCount: ${captured.repeatNodeCount},
	NodeProps: map[string]map[int]string{
		${nodeProps().join("\n\t\t")}
	},
	SkippedNodes: ${goInts(captured.skippedNodes || [])},
	TokenData:    ${go(captured.tokenData)},
	Tokenizers:   []lezer.TokenizerSpec{${tokenizers().join(", ")}},
	TopRules:     []lezer.TopRule{${topRules.join(", ")}},
	Dialects:     []lezer.DialectSpec{${dialects.join(", ")}},
	DynamicPrec:  map[int]int{${dynamicPrec.join(", ")}},
	Specialized:  []lezer.SpecializerSpec{${specializers().join(", ")}},
	TokenPrec:    ${captured.tokenPrec},
	Context:      ${go(contextName())},
	Styles: map[int][]lezer.RuleSpec{
		${styles().join("\n\t\t")}
	},
}
`;
process.stdout.write(stylesModule ? testStyles : out);
