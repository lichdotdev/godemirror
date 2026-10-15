package org.example.web;

import javax.annotation.processing.Generated;

@Generated(value = "tool", date = "2024-01-01")
@Target({ElementType.TYPE, ElementType.METHOD})
@Retention(RetentionPolicy.RUNTIME)
public @interface Route {
    String value() default "/";
    String[] methods() default {"GET"};
    int priority() default -1;
}

enum Level implements Comparable<Level> {
    LOW(1), MEDIUM(5) {
        @Override String label() { return "mid"; }
    }, HIGH(10);

    private final int weight;
    Level(int weight) { this.weight = weight; }
    String label() { return name().toLowerCase(); }
}

abstract class Base<T extends Number & Comparable<T>> {
    protected abstract T zero();
    public <U> U convert(java.util.function.Function<? super T, U> f) { return f.apply(zero()); }
}

class Impl extends Base<Integer> {
    @Override protected Integer zero() { return Integer.valueOf(0); }
    static { System.loadLibrary("native"); }
    { int instanceInit = 42; }
    native void nativeMethod();
    transient int t;
    strictfp double fp() { return 1.0e10d + 3.14F + 0x1.8p1 + 1_000_000L; }
    void arrays(String @NonNull [] args, int... rest) {
        Object o = (Comparable<String> & java.io.Serializable) s -> 0;
        int len = args.length;
        synchronized (this) { len >>= 1; len >>>= 2; len ^= 3; len %= 4; }
    }
}
