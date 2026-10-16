function push<T>(item: T, count?: number, ...rest: T[]): number {
  const each = (x: T): T => x;
  return rest.length + count;
}
class Ring {
  constructor(private capacity: number, readonly name = "ring") {}
  push(item: string): void {}
}
