import React, { useState } from "react";

export function App({ items, onSelect }) {
  const [open, setOpen] = useState(false);
  const Comp = <T,>(x) => x;
  return (
    <>
      <Header title="Hi &amp; bye" {...props} disabled />
      <ul className={open ? "open" : undefined} data-id='x'>
        {items.map((item) => (
          <li key={item.id} onClick={() => onSelect(item)}>
            {item.label} &mdash; {/* comment */}
            <svg:rect xlink:href="#a" />
            <Foo.Bar.Baz value={1}>text</Foo.Bar.Baz>
          </li>
        ))}
      </ul>
      {open && <Modal onClose={() => setOpen(false)}>Close me</Modal>}
      <div>
        a < b and {"literal"} {`template ${x}`}
      </div>
    </>
  );
}

const lt = a < b, gt = c > d;
const g = <T extends>(x) => x;
const h = < div >spaced</ div >;
