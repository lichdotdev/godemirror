import * as React from "react";

type Props<T> = { items: T[]; render: (item: T) => React.ReactNode };

export function List<T extends { id: string }>({ items, render }: Props<T>) {
  const identity = <T,>(x: T) => x;
  const constrained = <T extends unknown>(x: T) => x;
  return (
    <ul>
      {items.map((it) => <li key={it.id}>{render(identity(it))}</li>)}
    </ul>
  );
}

export const Button: React.FC<{ label: string }> = ({ label }) => (
  <button type="button" onClick={(e: React.MouseEvent) => e.preventDefault()}>
    {label as string}
  </button>
);

const value = obj as unknown as Props<string>;
const notCast = <div>{value!}</div>;
const generic = <Generic<string> prop="x" />;
enum E { A }
