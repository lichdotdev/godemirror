//! A tiny line-based key/value server.

use std::collections::BTreeMap;
use std::io::{self, BufRead, BufReader, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::{Arc, Mutex};
use std::thread;

#[derive(Debug)]
enum Command<'a> {
    Get(&'a str),
    Set { key: &'a str, value: &'a str },
    Quit,
}

#[derive(Debug, thiserror::Error)]
pub enum ParseError {
    #[error("empty line")]
    Empty,
    #[error("unknown command `{0}`")]
    Unknown(String),
}

impl<'a> TryFrom<&'a str> for Command<'a> {
    type Error = ParseError;

    fn try_from(line: &'a str) -> Result<Self, Self::Error> {
        let mut words = line.split_whitespace();
        match words.next() {
            None => Err(ParseError::Empty),
            Some("GET") => words.next().map(Command::Get).ok_or(ParseError::Empty),
            Some("SET") => {
                let key = words.next().ok_or(ParseError::Empty)?;
                let value = words.next().unwrap_or_default();
                Ok(Command::Set { key, value })
            }
            Some("QUIT") => Ok(Command::Quit),
            Some(other) => Err(ParseError::Unknown(other.to_owned())),
        }
    }
}

type Store = Arc<Mutex<BTreeMap<String, String>>>;

fn handle(stream: TcpStream, store: Store) -> io::Result<()> {
    let mut writer = stream.try_clone()?;
    for line in BufReader::new(stream).lines() {
        let line = line?;
        let reply = match Command::try_from(line.as_str()) {
            Ok(Command::Get(key)) => store.lock().unwrap().get(key).cloned().unwrap_or_else(|| "nil".into()),
            Ok(Command::Set { key, value }) => {
                store.lock().expect("poisoned").insert(key.to_string(), value.to_string());
                String::from("OK")
            }
            Ok(Command::Quit) => break,
            Err(e) => format!("ERR {e}"),
        };
        writeln!(writer, "{}", reply)?;
    }
    Ok(())
}

fn main() -> io::Result<()> {
    let listener = TcpListener::bind("127.0.0.1:7878")?;
    let store: Store = Default::default();
    for stream in listener.incoming().filter_map(Result::ok) {
        let store = Arc::clone(&store);
        thread::spawn(move || {
            if let Err(err) = handle(stream, store) {
                eprintln!("connection failed: {err:?}");
            }
        });
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_set() {
        assert!(matches!(Command::try_from("SET a 1"), Ok(Command::Set { key: "a", value: "1" })));
    }
}
