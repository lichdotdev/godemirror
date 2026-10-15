// names.mjs is a mode for the token table's tests: a line is tokens ended
// by "/" or the end of the line, each token's text (less its "/") its
// style, so two tokens can touch.
export const names = {
  name: "names",
  token(stream) {
    let style = "";
    for (let ch; (ch = stream.next()) != null && ch != "/"; ) style += ch;
    return style;
  },
};
