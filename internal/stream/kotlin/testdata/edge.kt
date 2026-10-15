/* level 1 /* level 2 /* level 3 */ back to 2 */ back to 1
still comment */ val after = 1
/* unclosed /* nested
  across lines */ still open
done */ fun f() = 1
val a = "unterminated
val b = 'also unterminated
continues' + 1
val c = """triple ${ open
  and $more "still" ""
end""" + `back tick` + `weird name`
val d = "escaped \" quote \\" + "dollar $ alone" + "brace ${a.b{c}}"
val e = list.* + a * b + a.*b + a.  *b
val f = a/*inline*/b // trailing
val g = a//b
val h = a +/* c */ b -//c
val i = a **= b *// c
val class fun object interface var ident
fun foo bar baz(qux)
fun /* comment */ name() @meta name2
val (p, q) = pair
x.let { val inner = it }
0XFFl 0B101 1__2 1.e5 1.5e 1.5E+_3 2UL 3uL 4f 5F .5f ..5 1..2 0x 0b2 12abc 0xg
@ann @ann.sub @Ann(x = 1) @`a` @$x
#!shebang? # hash ` lone \ back
x?.y!!.z ?: w; a?.let(::f); a::class.java
🎉 val emoji = "🎉 in string ${"🎉"}" // 🎉 comment
日本語 = 1; Ωmega αβγ ¡x
a	tab	separated
""
"""
