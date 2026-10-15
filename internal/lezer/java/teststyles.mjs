// teststyles.mjs adds styleTags of every kind @lezer/java's own styles leave
// out (inherit, opaque, several tags, wildcard and deeper paths) for
// TestExtraStylesMatchLezer. go generate writes teststyles_test.go and
// testdata/styled/*.golden from it.
export default ({ styleTags, tags: t }) => [
  styleTags({
    "Block/...": t.emphasis,
    "ClassBody/...": t.strong,
    "LambdaExpression!": t.special(t.variableName),
    "Annotation!": t.meta,
    "StringLiteral": [t.string, t.special(t.string)],
    "ClassBody/MethodDeclaration/Definition": t.function(t.definition(t.variableName)),
    "*/ArgumentList/Identifier": [t.local(t.variableName), t.constant(t.name)],
    "SwitchBlock/...": t.processingInstruction,
  }),
];
