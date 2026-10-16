// richer.mjs is godemirror's RicherGoTags written as the styleTags a CodeMirror
// caller would configure @lezer/go with, for the goldens under testdata/richer
// that TestRicherGoTagsMatchLezer (in the root package) checks the Go rules
// against. Keep the two the same, rule for rule.
export default ({ styleTags, tags: t }) => [
  styleTags({
    "Parameter/DefName": t.definition(t.local(t.variableName)),
    make: t.function(t.standard(t.variableName)),
  }),
];
