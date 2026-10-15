package com.example.stock;

import java.math.BigDecimal;
import static java.util.Objects.requireNonNull;
import lombok.*;

@Getter
@AllArgsConstructor
@lombok.experimental.Accessors(chain = true)
@SuppressWarnings({"unchecked", "rawtypes"})
public enum Movement implements Comparable<Movement> {
    OUTFLOW("B", -1),
    INFLOW("C", 1) {
        @Override
        int sign() { return 1; }
    },
    NONE;

    private final String value;
    private int weight = 0, order;
    static final BigDecimal LIMIT = BigDecimal.ONE;

    Movement() { this("", 0); }

    Movement(String value, int weight) {
        this.value = requireNonNull(value);
        this.weight = weight;
    }

    int sign() {
        int local = weight;
        @Deprecated int marked = 0;
        StockUpdate stockUpdate = new StockUpdate();
        return stockUpdate.value.compareTo(LIMIT) + local + marked;
    }

    @interface Tagged { String name() default "x"; }
}
