// The JavaScript side of toy.grammar's externals, which toy_test.go ports.

import { ExternalTokenizer, ContextTracker } from "@lezer/lr";
import { insertedSemi, Spread, TypeName, ShallowMark, DeepMark, kwLet, kwIf, kwElse, kwAs, Block } from "./parser.terms.js";

const newline = 10, closeBrace = 125, dot = 46, at = 64, openBrace = 123, bang = 33;

// insertSemicolon ends a statement at a line break, a closing brace or the
// end of the input, as JavaScript's automatic semicolons do.
export const insertSemicolon = new ExternalTokenizer((input, stack) => {
  let next = input.next;
  if ((next == closeBrace || next == -1 || next == newline) && stack.canShift(insertedSemi))
    input.acceptToken(insertedSemi);
}, { contextual: true, fallback: true });

// spread reads "...".
export const spread = new ExternalTokenizer((input) => {
  if (input.next == dot && input.peek(1) == dot && input.peek(2) == dot)
    input.acceptToken(Spread, 3);
});

const upperA = 65, upperZ = 90, lowerA = 97, lowerZ = 122;

const isLetter = (ch) => (ch >= upperA && ch <= upperZ) || (ch >= lowerA && ch <= lowerZ);

// typeName reads a capitalized word, which may also be an identifier: the
// parser tries both.
export const typeName = new ExternalTokenizer((input) => {
  if (input.next < upperA || input.next > upperZ) return;
  let length = 1;
  while (isLetter(input.peek(length))) length++;
  input.acceptToken(TypeName, length);
}, { extend: true });

// marker reads "@", told apart by how deep in blocks it is, and only in the
// strict dialect when it is followed by "!".
export const marker = new ExternalTokenizer((input, stack) => {
  if (input.next != at) return;
  let strict = input.peek(1) == bang;
  if (strict && !stack.dialectEnabled(0)) return;
  input.acceptToken(stack.context > 1 ? DeepMark : ShallowMark, strict ? 2 : 1);
});

const keywordTerms = { let: kwLet, if: kwIf, else: kwElse };

export const keywords = (value) => keywordTerms[value] ?? -1;

export const contextualKeywords = (value) => (value == "as" ? kwAs : -1);

// depth counts the blocks a position is in.
export const depth = new ContextTracker({
  start: 0,
  shift: (context, term, stack, input) => (input.next == openBrace ? context + 1 : context),
  reduce: (context, term) => (term == Block ? context - 1 : context),
  hash: (context) => context,
});
