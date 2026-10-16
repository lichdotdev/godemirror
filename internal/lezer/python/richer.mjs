// richer.mjs is godemirror's RicherPythonTags written as the styleTags a CodeMirror
// caller would configure @lezer/python with, for the goldens under testdata/richer
// that TestRicherPythonTagsMatchLezer (in the root package) checks the Go rules
// against. Keep the two the same, rule for rule.
export default ({ styleTags, tags: t }) => [
  styleTags({
    "ParamList/VariableName": t.definition(t.local(t.variableName)),
  }),
];
