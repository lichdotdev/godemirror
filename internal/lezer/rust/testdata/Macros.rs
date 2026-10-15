#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    #[should_panic(expected = "boom")]
    fn it_works() {
        assert_eq!(2 + 2, 4, "math is {}", "broken");
        println!("{:?} {name}", vec![1, 2, 3], name = "x");
        let v = vec![0; 10];
        my_crate::my_macro!(a b c);
        format_args!["{}", 1];
        thread_local! { static X: u8 = 0; }
        panic!("boom");
    }
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct Config {
    #[serde(default = "default_port")]
    port: u16,
    /** block doc */
    name: String, /* trailing /* nested */ comment */
}

#[doc = r#"raw attr"#]
fn attributed(#[allow(unused)] x: i32) {}
