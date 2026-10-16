// richer.mjs is godemirror's RicherRustTags written as the styleTags a CodeMirror
// caller would configure @lezer/rust with, for the goldens under testdata/richer
// that TestRicherRustTagsMatchLezer (in the root package) checks the Go rules
// against. Keep the two the same, rule for rule.
export default ({ styleTags, tags: t }) => [
  styleTags({
    "Parameter/BoundIdentifier Parameter/*/BoundIdentifier": t.definition(t.local(t.variableName)),
  }),
];
