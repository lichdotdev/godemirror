fn literals() {
    let ints = (0, 42, 1_000_000, 0xdead_BEEF, 0o777, 0b1010_1010, 7u8, 9i64, 3usize);
    let floats = (1.0, 1.5e10, 2E-3, 3e+7, 1_0.0_1, 2.5f32, 1f64, 1e3f64, 0.1_f32, 1.);
    let not_floats = (1..2, 1.max(2), 1.e3, 2.f32, 3._x);
    let tricky = (1.0.max(2.0), x.0.1, 5f3, 7f6x, 1e, 1e+, 2ef32);
    let strings = ("plain", "esc \" \\ \n \t \u{1F600} \x7f", "multi
line");
    let bytes = (b"bytes", b'c', b'\n', b'\x00');
    let chars = ('a', '\'', '\\', '\u{10FFFF}', 'é', '\n');
    let raw = (r"raw \n", r#"has "quotes""#, r##"has "# inside"##, br"bytes", br#"b"#, r"", r#""#);
    let lifetimes: &'static str = "s";
    let bools = (true, false);
    let unicode = "héllo 世界 🎉";
    let emoji = '🎉';
    let c_str = c"hello";
    let ident_r = r;
    let ident_b = b;
    let ident_br = br;
    let rx = r#x;
}
