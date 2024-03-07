

pub mod serde_client;
pub mod serde_server;
mod type_decoder;

struct Generator {
    buf: String,
    at_linestart: bool,
}

impl Generator {
    fn new() -> Self {
        Self {
            buf: "".into(),
            at_linestart: true,
        }
    }

    fn writer(&mut self) -> Writer {
        Writer {
            buf: &mut self.buf,
            at_linestart: &mut self.at_linestart,
            indent: 0,
        }
    }
}

struct Writer<'a> {
    buf: &'a mut String,
    at_linestart: &'a mut bool,
    indent: i32,
}

impl<'a> Writer<'a> {
    fn indent(&mut self) -> Writer {
        Writer {
            buf: &mut self.buf,
            at_linestart: &mut self.at_linestart,
            indent: self.indent + 1,
        }
    }

    fn with_indent<Fn: FnMut(Writer)>(&mut self, mut f: Fn) {
        let w = self.indent();
        f(w)
    }

    fn newline(&mut self) {
        self.buf.push('\n');
        *self.at_linestart = true;
    }

    fn write(&mut self, str: &str) {
        if *self.at_linestart {
            for _i in 0..self.indent {
                self.buf.push_str("  ");
            }
        }
        self.buf.push_str(str);

        *self.at_linestart = str.ends_with('\n');
    }

    fn writeln(&mut self, str: &str) {
        self.write(str);
        self.newline();
    }

    fn write_string(&mut self, s: String) {
        self.write(&s);
    }
}
