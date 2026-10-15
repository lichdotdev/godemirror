#!/usr/bin/env node
"use strict";
import fs, { readFile as rf, writeFile } from "node:fs";
import * as path from 'node:path';
import json from "./data.json" with { type: "json" };
import defer * as lazy from "./lazy.js";
export { rf, writeFile as wf };
export * from "./util.js";
export * as ns from "./ns.js";
export default class Store extends Base {
  #items = new Map();
  static #count = 0;
  static { Store.#count = 1; }
  @observable accessor value = 0;
  constructor(name, ...rest) {
    super(name);
    this.name = name ?? "anon";
    this.rest = [...rest];
  }
  get size() { return this.#items.size; }
  set size(v) { throw new RangeError(`cannot set ${v}`); }
  static async *stream(source) {
    for await (const chunk of source) yield* chunk;
  }
  #secret() { return #items in this; }
}
const re = /ab+c[\/\]]?/giu, half = a / 2 / b;
let x = 0x1F + 0o17 + 0b101 + 1_000_000n + .5e-3 + 3.14;
x ||= 1; x &&= 2; x ??= 3; x **= 2; x >>>= 1;
const obj = { a, b: 2, [key]: 3, 'q': 4, async *gen() {}, get g() { return 1 }, ...rest };
const { a: alias = 5, ...others } = obj, [first, , third = 3] = arr;
label: for (let i = 0; i < 10; i++) {
  if (i % 2) continue label;
  else break label;
}
switch (typeof x) {
  case "number": x--; break;
  default: void 0;
}
try { risky?.(); } catch { } finally { debugger; }
try { risky(); } catch ({ message }) { console.error(message); }
do x++; while (x < 5)
while (false) {}
with (obj) { a; }
const fn = async (a, b = 2) => await a + b;
const tag = html`<p>${user.name}\n</p>`;
const nested = `outer ${`inner ${deep} \u{1F600}`} done`;
new Foo;
new.target;
import.meta.url;
const mod = await import("./mod.js");
delete obj.a, delete obj?.b;
a?.b?.[c]?.(d);
x = a ? b : c ? d : e;
x = a < b > c;
y = !a && ~b || -c;
z = a instanceof B && "k" in o;
function* gen() { const v = yield 1; return v; }
async function af() { await null; }
