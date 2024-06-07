use std::option;

use litparser_derive::LitParser;
use swc_common::errors::{ColorConfig, Handler};
use swc_common::input::StringInput;
use swc_common::sync::Lrc;
use swc_common::{FileName, SourceMap};
use swc_ecma_parser::lexer::Lexer;
use swc_ecma_parser::{Parser, Syntax};

use litparser::LitParser;

#[test]
fn test_parse() {
    #[derive(LitParser)]
    struct Foo {
        foo: String,
        bar: std::time::Duration,
    }

    let expr = parse(r#"{ foo: "foo", bar: "1h" }"#);
    let foo = Foo::parse_lit(&expr).expect("failed to parse lit");
    assert_eq!(foo.foo, "foo");
    assert_eq!(foo.bar, std::time::Duration::from_secs(3600));
}

#[test]
fn test_parse_str_keys() {
    #[derive(LitParser)]
    struct Foo {
        foo: String,
        bar: i32,
    }

    let expr = parse(r#"{ "foo": "foo", "bar": 3 }"#);
    let foo = Foo::parse_lit(&expr).expect("failed to parse lit");
    assert_eq!(foo.foo, "foo");
    assert_eq!(foo.bar, 3);
}

#[test]
fn test_parse_refs() {
    struct Dummy<'a> {
        foo: Option<&'a str>,
    }
    impl LitParser for Dummy<'_> {
        fn parse_lit(input: &swc_ecma_ast::Expr) -> anyhow::Result<Self> {
            Ok(Self { foo: None })
        }
    }

    #[derive(LitParser)]
    struct Foo<'a> {
        foo: String,
        dummy: Dummy<'a>,
    }

    let expr = parse(r#"{ foo: "foo", "dummy": null }"#);
    let foo = Foo::parse_lit(&expr).expect("failed to parse lit");
    assert_eq!(foo.foo, "foo");
}

#[test]
fn test_parse_option() {
    // let foo: Option<Option<T>> = None;

    #[derive(LitParser)]
    struct Foo {
        // Try with different ways of writing Option, since we parse the syntax tree.
        foo: Option<String>,
        bar: option::Option<String>,
        baz: std::option::Option<String>,
        boo: ::std::option::Option<String>,
    }

    // The empty case
    {
        let expr = parse(r#"{ }"#);
        let foo = Foo::parse_lit(&expr).expect("failed to parse lit");
        assert_eq!(foo.foo, None);
        assert_eq!(foo.bar, None);
        assert_eq!(foo.baz, None);
        assert_eq!(foo.boo, None);
    }

    // The non-empty case
    {
        let expr = parse(r#"{ foo: "foo", bar: "bar", baz: "baz", boo: "boo" }"#);
        let foo = Foo::parse_lit(&expr).expect("failed to parse lit");
        assert_eq!(foo.foo, Some("foo".to_string()));
        assert_eq!(foo.bar, Some("bar".to_string()));
        assert_eq!(foo.baz, Some("baz".to_string()));
        assert_eq!(foo.boo, Some("boo".to_string()));
    }
}

fn parse(src: &str) -> Box<swc_ecma_ast::Expr> {
    let cm: Lrc<SourceMap> = Default::default();
    let handler = Handler::with_tty_emitter(ColorConfig::Auto, true, false, Some(cm.clone()));

    let fm = cm.new_source_file(FileName::Custom("test.ts".into()), src.into());
    let lexer = Lexer::new(
        Syntax::Es(Default::default()),
        Default::default(),
        StringInput::from(&*fm),
        None,
    );

    let mut parser = Parser::new_from(lexer);

    for e in parser.take_errors() {
        e.into_diagnostic(&handler).emit();
    }

    parser.parse_expr().expect("failed to parse expr")
}
