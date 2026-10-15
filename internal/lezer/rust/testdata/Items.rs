#!/usr/bin/env rust-script
//! Crate docs.
#![allow(dead_code)]

use std::collections::{HashMap, HashSet};
use std::fmt::{self, Display};
use crate::inner::*;
use super::Thing as Other;
pub(crate) use self::nested::Item;

extern crate alloc;

/// A shape.
#[derive(Debug, Clone, PartialEq)]
pub enum Shape<T: Copy + Default = f64> {
    Circle { radius: T },
    Rect(T, T),
    Unit,
    Tagged = 3,
}

#[repr(C)]
pub struct Point<'a, T> where T: 'a + ?Sized {
    pub x: &'a T,
    y: i32,
}

struct Tuple(pub u8, String);
struct Empty;

union Bits { i: u32, f: f32 }

pub trait Area: Display + Send {
    const SIDES: usize;
    type Output: Into<f64>;
    fn area(&self) -> Self::Output;
    fn name(&self) -> String { String::from("shape") }
}

impl<T> Area for Shape<T> where T: Copy + Default + Into<f64> + Display {
    const SIDES: usize = 0;
    type Output = f64;

    fn area(&self) -> f64 {
        match *self {
            Shape::Circle { radius } => 3.14 * radius.into() * radius.into(),
            Shape::Rect(w, h) => w.into() * h.into(),
            _ => 0.0,
        }
    }
}

impl fmt::Display for Empty {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "empty")
    }
}

unsafe impl Send for Bits {}
impl !Sync for Tuple {}

pub mod nested {
    pub struct Item;
    mod deeper;
}

pub type Map<K> = HashMap<K, Vec<Box<dyn Fn(&K) -> bool + 'static>>>;
static mut COUNTER: u64 = 0;
const LIMIT: usize = 1 << 10;

pub async unsafe fn complicated<'a, 'b: 'a, const N: usize>(
    xs: &'a [u8; N],
    mut ys: impl Iterator<Item = &'b str>,
    (a, b): (i32, i32),
) -> Result<Option<&'a u8>, Box<dyn std::error::Error>> {
    Ok(xs.first())
}

extern "C" {
    fn abs(input: i32) -> i32;
    static errno: i32;
}

pub fn generic<T: for<'x> Fn(&'x str) -> &'x str>(f: T) -> impl Fn() {
    move || { f("x"); }
}

macro_rules! square {
    ($x:expr) => { $x * $x };
    ($($rest:tt)*) => {{ 0 }};
}

fn default() -> fn(i32) -> i32 { |x| x }
