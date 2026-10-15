a = 'single' + "double" + '''long
single''' + """long
double"""
b = r'raw\d' + R"raw\"quote" + rb'bytes\n' + Br'''raw
long\'''' + rf"{x}\n" + fR'{y!r}'
c = b'\x41\u1234\U0001F600\N{BULLET}\o17\'\\' "implicit" 'concat'
d = f"plain {x} {y!r:>10} {z=} {{escaped}} {a + b:{width}.{prec}f}"
e = f'''multi
{line!s}
  {nested["key"]:{fmt}}'''
g = f"{f'{inner}'}" + f"{x:%Y-%m-%d}" + F"""{
    value
}"""
h = u'unicode' + U"also" + 'unterminated
i = "next line ok"
j = f"unterminated {x
k = "\N{unterminated
l = 'trailing backslash\
continues'
m = f"{'{'}" + f"{x!a}" + rf'\{x}'
