import type { Foo } from "./foo";
import { type Bar, baz } from "./bar";
export type { Foo };
declare module "lib" {
  export function f(x: number): string;
}
declare global { interface Window { app: App } }
namespace NS.Inner { export const v = 1; }
enum Color { Red = 1, Green, Blue = "b" }
const enum Flags { A = 1 << 0, B = 1 << 1 }
interface Shape<T = unknown> extends Base, Other<T> {
  readonly kind: "circle" | "square";
  area?(): number;
  [key: string]: unknown;
  new (x: number): Shape;
  <U>(arg: U): U;
}
type Partial2<T> = { readonly [K in keyof T]?: T[K] };
type Minus<T> = { -readonly [K in keyof T]-?: T[K] };
type Cond<T> = T extends string ? "s" : T extends infer U extends number ? U : never;
type Tmpl = `prefix-${string}-suffix`;
type Fn = (a: number, ...rest: string[]) => void;
type Ctor = abstract new () => object;
type Tuple = [a: string, b?: number, ...c: boolean[]];
type Q = typeof import("./mod");
type U1 = unique symbol;
type Idx = Foo["bar"][number];
abstract class Animal<T> implements Walker {
  private readonly legs: number = 4;
  protected abstract name: string;
  declare field: string;
  public static override create?(): void;
  constructor(public x: number, private y?: string) { super(); }
  abstract speak(): void;
  get value(): T { return this._v!; }
  @decorate() method<U extends keyof T>(this: Animal<T>, u: U): asserts u is U {}
}
function isStr(x: unknown): x is string { return typeof x === "string"; }
function assertDefined<T>(x: T): asserts x {}
const cfg = { a: 1 } satisfies Config as const;
const el = <HTMLElement>document.body;
const gen = <T>(x: T): T => x;
const g2 = <T extends object>(x: T) => x;
let v!: number;
let opt = obj!.prop!;
function over(a: string): void;
function over(a: number): void;
function over(a: any) {}
export default interface Def {}
let a: Array<Map<string, number>> = [];
const r = a < b > c;
for (const k in obj as Record<string, any>) {}
accessor;
using res = getResource();
await using ares = getAsync();
