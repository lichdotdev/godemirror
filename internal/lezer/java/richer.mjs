// richer.mjs is godemirror's RicherJavaTags written as the styleTags a
// CodeMirror caller would configure @lezer/java with, for the goldens under
// testdata/richer that TestRicherJavaTagsMatchLezer (in the root package)
// checks the Go rules against. Keep the two the same, rule for rule.
export default ({ styleTags, tags: t }) => [
  styleTags({
    "EnumConstant/Definition": t.constant(t.variableName),
    "MarkerAnnotation Annotation MarkerAnnotation/Identifier Annotation/Identifier": t.annotation,
    "MarkerAnnotation/ScopedIdentifier! Annotation/ScopedIdentifier!": t.annotation,
    "FieldDeclaration/VariableDeclarator/Definition": t.definition(t.propertyName),
    "PackageDeclaration/Identifier ImportDeclaration/Identifier": t.namespace,
    "PackageDeclaration/ScopedIdentifier! ImportDeclaration/ScopedIdentifier!": t.namespace,
  }),
];
