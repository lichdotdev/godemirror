#!/usr/bin/env python3
"""Module docstring.

Spans lines.
"""
from __future__ import annotations

import os, sys
from collections import (
    OrderedDict,
    defaultdict as dd,
)
from . import sibling
from ..pkg.mod import *

__all__ = ["Point", 'main']

CONSTANT: int = 0x1F + 0o17 + 0b1010 + 1_000_000 + 3.14e-10 + 2j


class Point(Base, metaclass=Meta):
    """A point."""

    x: float
    y: float = 0.0

    def __init__(self, x, y=0, *args, key=None, **kwargs) -> None:
        self.x = x
        self.y = y

    @property
    def norm(self):
        return (self.x ** 2 + self.y ** 2) ** 0.5

    @staticmethod
    async def fetch(url: str, /, timeout: float = 1.5) -> bytes:
        async with session.get(url) as resp:
            async for chunk in resp.content:
                yield chunk
        await asyncio.sleep(0)


def generic[T: (int, str), *Ts, **P](items: list[T]) -> dict[str, T]:
    return {str(i): i for i in items if i is not None}


type Alias[K] = dict[K, list[K]]


def control(n):
    global counter
    nonlocal_value = 1
    if n < 0 and not n == -1 or n in (1, 2):
        pass
    elif n >= 10:
        raise ValueError("too big") from None
    else:
        while True:
            n -= 1
            if n % 2:
                continue
            break
        else:
            print("done")
    for i, (a, b) in enumerate(zip(xs, ys)):
        del xs[i]
    try:
        x = 1 / n
    except (ZeroDivisionError, TypeError) as e:
        assert False, e
    except* OSError:
        pass
    finally:
        cleanup()
    with open("a") as f, open("b") as g:
        data = f.read() + g.read()
    lam = lambda a, *b, c=3: a if b else c
    walrus = [y for x in data if (y := x * 2) > 3]
    s = {1, 2, 3}
    sl = data[1:2:3], data[::], data[...]
    neg = ~n << 2 >> 1 & 3 | 4 ^ 5
    m @= other
    return (yield from gen()), *rest


def matching(command):
    match command.split():
        case [action]:
            pass
        case ["go", direction] | ["move", direction]:
            go(direction)
        case Point(x=0, y=0):
            print("Origin")
        case {"key": value, **rest}:
            pass
        case [1, 2, *others] if others:
            pass
        case -1 | 1.5 | "str" | None | True:
            pass
        case _:
            pass


if __name__ == "__main__":
    main(*sys.argv[1:], **{"verbose": True}); exit(0)
