// richer.mjs is godemirror's RicherJavaScriptTags written as the styleTags a CodeMirror
// caller would configure @lezer/javascript with, for the goldens under testdata/richer
// that TestRicherJavaScriptTagsMatchLezer (in the root package) checks the Go rules
// against. Keep the two the same, rule for rule.
export default ({ styleTags, tags: t }) => [
  styleTags({
    "ParamList/VariableDefinition ParamList/ArrayPattern/VariableDefinition ParamList/ObjectPattern/PatternProperty/VariableDefinition":
      t.definition(t.local(t.variableName)),
  }),
];
