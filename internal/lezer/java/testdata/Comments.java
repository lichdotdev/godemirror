// A file that is mostly comments.
/* block comment */
/*
 * Multi-line block
 * comment with `code` and <b>html</b>.
 */
module com.example.app {
    requires transitive java.sql; // inline
    exports com.example.api to com.example.client;
    opens com.example.internal;
    uses com.example.spi.Plugin;
    provides com.example.spi.Plugin with com.example.DefaultPlugin;
}
/** Javadoc at the end, unterminated? no. */
