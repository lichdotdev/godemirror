// nesting.mjs is the parse wrapper golden.mjs parses the fixtures with
// (its --wrap option): @lezer/html's configureNesting with the default
// nesting of @codemirror/lang-html 6.4.12 (src/html.ts: defaultNesting,
// defaultAttrs), its languages' parsers taken from @lezer/javascript and
// @lezer/css as @codemirror/lang-javascript and @codemirror/lang-css
// configure them.

export default async function (load) {
  const { configureNesting } = await load("@lezer/html");
  const { parser: js } = await load("@lezer/javascript");
  const { parser: css } = await load("@lezer/css");
  const eventAttributes = ("beforeunload copy cut dragstart dragover dragleave dragenter dragend " +
    "drag paste focus blur change click load mousedown mouseenter mouseleave " +
    "mouseup keydown keyup resize scroll unload").split(" ").map((n) => "on" + n);
  const tags = [
    { tag: "script", attrs: (a) => a.type == "text/typescript" || a.lang == "ts", parser: js.configure({ dialect: "ts" }) },
    { tag: "script", attrs: (a) => a.type == "text/babel" || a.type == "text/jsx", parser: js.configure({ dialect: "jsx" }) },
    { tag: "script", attrs: (a) => a.type == "text/typescript-jsx", parser: js.configure({ dialect: "jsx ts" }) },
    { tag: "script", attrs: (a) => /^(importmap|speculationrules|application\/(.+\+)?json)$/i.test(a.type), parser: js.configure({ top: "SingleExpression" }) },
    { tag: "script", attrs: (a) => !a.type || /^(?:text|application)\/(?:x-)?(?:java|ecma)script$|^module$|^$/i.test(a.type), parser: js },
    { tag: "style", attrs: (a) => (!a.lang || a.lang == "css") && (!a.type || /^(text\/)?(x-)?(stylesheet|css)$/i.test(a.type)), parser: css },
  ];
  const attributes = [{ name: "style", parser: css.configure({ top: "Styles" }) }]
    .concat(eventAttributes.map((name) => ({ name, parser: js })));
  return configureNesting(tags, attributes);
}
