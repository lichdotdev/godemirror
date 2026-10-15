// mixnest.mjs is the parse wrapper mix_test.go's goldens are written with
// (golden.mjs's --wrap option): @lezer/java nesting itself through
// parseMixed, in the ways mix_test.go's nest does.

export default async function (load) {
  const { parseMixed } = await load("@lezer/common");
  const { parser } = await load("@lezer/java");
  return parseMixed((node, input) => {
    if (node.name == "StringLiteral" && input.read(node.from, node.from + 2) == '"!') return { parser };
    if (node.name == "LineComment" && input.read(node.from, node.to).includes("!")) return { parser };
    if (node.name == "BlockComment" && node.to - 2 > node.from + 2)
      return { parser, overlay: [{ from: node.from + 2, to: node.to - 2 }] };
    if (node.name == "Block") {
      const overlay = [];
      for (let c = node.node.firstChild; c; c = c.nextSibling)
        if (c.name == "LineComment" && input.read(c.from, c.from + 3) == "//>") overlay.push({ from: c.from + 3, to: c.to });
      if (overlay.length) return { parser, overlay };
    }
    return null;
  });
}
