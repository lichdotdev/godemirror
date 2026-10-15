fn main() {
    let mut v: Vec<i32> = Vec::new();
    let (a, mut b) = (1, 2);
    let Point { x, y: ref why, .. } = p;
    let [first, .., last] = arr;
    let x = if a > b && b < c || !d { a } else if a == b { b } else { -c };
    let y = a as u64 + (b as u64) * 2 - 3 / 4 % 5;
    let z = a << 2 >> 1 & 0xff | 0b1010 ^ 0o17;
    let r = &mut v;
    let rr = &&v;
    let deref = **rr;
    let cmp = a <= b && a >= b && a != b;
    a += 1; b -= 1; a *= 2; a /= 2; a %= 3; a <<= 1; a >>= 1; a &= 1; a |= 2; a ^= 3;
    let range = 0..10;
    let incl = 0..=10;
    let open = ..;
    let from = 5..;
    let to = ..=5;
    let closure = |x: i32, y| -> i32 { x + y };
    let empty = || 42;
    let moved = move |s| s.len();
    let or = a | b;
    let call = foo::<i32>(1, 2).bar().baz::<Vec<_>>()?;
    let field = self.inner.0.value;
    let idx = v[0][i + 1];
    let tup = (1,);
    let unit = ();
    let arr = [0u8; 16];
    let s = Struct { a, b: 2, ..Default::default() };
    let gen = Vec::<u8>::with_capacity(10);
    let lt = a < b;
    let gt = a > b;
    let shifted = 1 << b;
    'outer: loop {
        while let Some(x) = it.next() {
            if x == 0 { continue 'outer; }
            for i in 0..x { if i > 3 { break 'outer; } }
        }
        break;
    }
    let value = loop { break 5; };
    let m = match opt {
        Some(0) | None => "zero",
        Some(n) if n > 10 => "big",
        Some(1..=9) => "small",
        Some(ref x @ 10) => "ten",
        Some(_) => return,
    };
    if let Some(v) = map.get(&key) && v > 0 {}
    let fut = async { compute().await };
    let fut2 = async move { 1 };
    unsafe { COUNTER += 1; }
    let block = { let t = 1; t * 2 };
    let neg = -x;
    let not = !flag;
    let typed: &dyn Fn(i32) -> i32 = &|x| x;
    let path = <Vec<u8> as IntoIterator>::into_iter(v);
    let q = std::mem::size_of::<u64>();
    let t: (i32, &str) = (1, "a");
    let p: *const u8 = ptr as *const u8;
    let res: Result<(), ()> = Ok(());
    let ok = a&&b;
    let and = a & &b;
    x.0 .1;
    return;
}
