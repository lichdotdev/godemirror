package com.example.cache;

import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.Function;
import static java.util.Objects.requireNonNull;

/**
 * A bounded cache that evicts the least recently used entry.
 *
 * @param <K> the key type
 * @param <V> the value type
 */
public final class LruCache<K extends Comparable<? super K>, V> implements Iterable<Map.Entry<K, V>> {
    private static final int DEFAULT_CAPACITY = 0x100;
    private final Map<K, V> entries = new LinkedHashMap<>(16, 0.75f, true);
    private final int capacity;
    private volatile long hits = 0L, misses;

    public LruCache() { this(DEFAULT_CAPACITY); }

    public LruCache(int capacity) {
        if (capacity <= 0) {
            throw new IllegalArgumentException("capacity must be positive: " + capacity);
        }
        this.capacity = capacity;
    }

    public synchronized V get(K key, Function<? super K, ? extends V> loader) {
        requireNonNull(key, "key");
        V value = entries.get(key);
        if (value != null) {
            hits++;
            return value;
        }
        misses += 1;
        value = loader.apply(key);
        entries.put(key, value);
        if (entries.size() > capacity) {
            Iterator<K> it = entries.keySet().iterator();
            it.next();
            it.remove();
        }
        return value;
    }

    public <R extends Collection<? super V>> R copyValues(R target) {
        for (V v : entries.values()) target.add(v);
        return target;
    }

    @Override
    public Iterator<Map.Entry<K, V>> iterator() {
        return Collections.unmodifiableMap(entries).entrySet().iterator();
    }

    double hitRate() {
        long total = hits + misses;
        return total == 0 ? 0.0 : (double) hits / total * 100e-2;
    }

    static int[][] grid(int n) {
        int[][] g = new int[n][n];
        for (int i = 0, j = n - 1; i < n; i++, j--) {
            g[i][j] = i << 2 | j >>> 1 ^ ~i & 0b1010;
        }
        return g;
    }
}
