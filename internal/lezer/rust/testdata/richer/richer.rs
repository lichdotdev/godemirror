impl Ring {
    fn push(&mut self, item: u32, mut count: usize, (a, b): (u8, u8), &c: &u8) -> usize {
        let each = |x, y: u32| x + y;
        let local = item;
        count + each(local, item) as usize
    }
}
