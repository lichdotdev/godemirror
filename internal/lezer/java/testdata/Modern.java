package demo;

import java.util.List;
import java.util.stream.Collectors;

@FunctionalInterface
interface Shape { double area(); }

public sealed interface Expr permits Num, Add, Mul {}

record Num(int value) implements Expr {}
record Add(Expr left, Expr right) implements Expr {}
record Mul(Expr left, Expr right) implements Expr {
    Mul {
        if (left == null || right == null) throw new NullPointerException();
    }
}

@SuppressWarnings({"unchecked", "rawtypes"})
@Deprecated(since = "9", forRemoval = true)
public class Modern {
    static int eval(Expr e) {
        return switch (e) {
            case Num n -> n.value();
            case Add(Expr l, Expr r) -> eval(l) + eval(r);
            case Mul m when m.left() instanceof Num n && n.value() == 0 -> 0;
            case Mul m -> eval(m.left()) * eval(m.right());
        };
    }

    static String day(int d) {
        String name = switch (d) {
            case 1, 7 -> "weekend";
            case 2, 3, 4, 5, 6 -> {
                var s = "week" + "day";
                yield s;
            }
            default -> throw new IllegalStateException("bad day " + d);
        };
        return name;
    }

    static final String QUERY = """
        SELECT id, name
          FROM users
         WHERE name LIKE '%\"x\"%' \
           AND active = true
        """;

    public static void main(String... args) throws Exception {
        List<String> words = List.of("alpha", "beta", "gamma");
        var upper = words.stream()
            .filter(w -> w.length() > 4)
            .map(String::toUpperCase)
            .collect(Collectors.joining(", ", "[", "]"));
        Runnable r = () -> System.out.println(upper);
        r.run();
        Shape unit = () -> Math.PI;
        char c = '\n', q = '\'', u = 'é';
        label:
        for (String w : words) {
            if (w.isEmpty()) continue label;
            else break label;
        }
        Object o = words;
        if (o instanceof List<?> list && !list.isEmpty()) {
            System.out.printf("%d words%n", list.size());
        }
        try (var in = new java.io.StringReader("x")) {
            in.read();
        } catch (java.io.IOException | RuntimeException ex) {
            assert ex != null : "never";
        } finally {
            System.gc();
        }
        int[] arr = {1, 2, 3};
        arr[0] += arr[1]--;
        boolean b = true ^ false;
        do { b = !b; } while (b);
        new Thread(() -> {}).start();
        String café = "naïve ☕ — 🎉 emoji";
    }
}
