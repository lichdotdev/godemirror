// toy.grammar's styleTags, using every kind of rule @lezer/highlight has.

import { styleTags, tags as t } from "@lezer/highlight";

export const highlighting = styleTags({
  "let if else for of as assert": t.keyword,
  BooleanLiteral: t.bool,
  Number: t.number,
  String: t.string,
  Escape: t.escape,
  "Interpolation/...": t.special(t.string),
  "Interpolation!": t.meta,
  "LineComment BlockComment": t.comment,
  VariableName: t.variableName,
  PropertyName: t.propertyName,
  TypeName: t.typeName,
  "CallExpression/VariableName": t.function(t.variableName),
  Definition: t.definition(t.variableName),
  "ParamList/Definition": [t.local(t.variableName), t.definition(t.variableName)],
  "Block/ExpressionStatement/VariableName": t.special(t.variableName),
  "*/ArrowFunction/VariableName": t.local(t.variableName),
  "Block/...": t.content,
  "ShallowMark DeepMark": t.annotation,
  Spread: t.operator,
  '"+" "-" "*" "=" "=>" "|>"': t.operator,
  ".": t.derefOperator,
  "( )": t.paren,
  "{ }": t.brace,
  "; ,": t.separator,
  '"\\""': t.punctuation,
});
