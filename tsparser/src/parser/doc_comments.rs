/// Compute the doc comment on the line(s) immediately preceding the given position.
/// It returns None if there are no comments.
pub fn doc_comments_before(
    source_map: &swc_common::SourceMap,
    comments: &dyn swc_common::comments::Comments,
    pos: swc_common::BytePos,
) -> Option<String> {
    // Get the file and line number of the position.
    // It returns Err(file) if there is no line number information, in which
    // case we won't be able to find any comments.
    let Ok(res) = source_map.lookup_line(pos) else {
        return None;
    };

    let candidates = comments.get_leading(pos).or_else(|| {
        // If there are no comments at the pos, look up the start of the line
        // and try there.
        let start_pos = res.sf.lines.get(res.line)?;
        comments.get_leading(*start_pos)
    })?;

    // The list of comments includes all consecutive comments in the AST, even if
    // there are lines without comments in-between. We only want to include the comments that
    // are attached to each other.

    let mut comments = Vec::new();

    // Iterate over the comments in reverse. For every comment, ensure the line number
    for (i, c) in candidates.iter().rev().enumerate() {
        if i > 0 && c.kind == swc_common::comments::CommentKind::Block {
            // If we have a block comment that's not the first comment, ignore it.
            // We only ever combine multiple line comments.
            break;
        }

        // Ensure we don't have any gaps in lines between the AST node and the comment in question.
        let Some(start_pos_ith_line) = res.sf.lines.get(res.line - (i + 1)) else {
            break;
        };
        if c.span.hi >= *start_pos_ith_line {
            comments.push(c);
        } else {
            break;
        }

        // If this was a block comment, don't consider any additional comments.
        if c.kind == swc_common::comments::CommentKind::Block {
            break;
        }
    }

    if comments.len() > 0 {
        let mut result = String::new();
        for comment in comments.iter().rev() {
            let is_jsdoc = comment.kind == swc_common::comments::CommentKind::Block
                && comment.text.starts_with("*");

            for line in comment.text.lines() {
                let mut trimmed = line.trim();
                if is_jsdoc {
                    if trimmed.starts_with("/**") {
                        trimmed = trimmed[3..].trim_start();
                    } else if trimmed.starts_with("*/") {
                        trimmed = trimmed[2..].trim_start();
                    } else if trimmed.starts_with("*") {
                        trimmed = trimmed[1..].trim_start();
                    }
                }
                result.push_str(trimmed);
                result.push('\n');
            }
        }

        let trimmed = result.trim();
        if trimmed.len() > 0 {
            return Some(trimmed.to_string());
        }
    }

    None
}

#[cfg(test)]
mod tests {
    use super::*;
    use swc_common::comments::SingleThreadedComments;
    use swc_common::input::StringInput;
    use swc_common::{FileName, SourceMap, Spanned};
    use swc_ecma_ast as ast;
    use swc_ecma_parser::lexer::Lexer;
    use swc_ecma_parser::{Parser, Syntax};

    fn decl_comments(src: &str) -> Vec<Option<String>> {
        let source_map: SourceMap = Default::default();
        let file = source_map.new_source_file(FileName::Custom("test.ts".into()), src.into());
        let comments: Box<SingleThreadedComments> = Box::new(Default::default());
        let lexer = Lexer::new(
            Syntax::Typescript(Default::default()),
            ast::EsVersion::Es2022,
            StringInput::from(file.as_ref()),
            Some(&comments),
        );

        let mut parser = Parser::new_from(lexer);
        let ast = parser.parse_module().unwrap();

        let mut result = Vec::new();
        for it in ast.body {
            if let ast::ModuleItem::Stmt(stmt) = it {
                if let ast::Stmt::Decl(decl) = stmt {
                    let c = doc_comments_before(&source_map, &comments, decl.span_lo());
                    result.push(c);
                }
            }
        }

        result
    }

    #[test]
    fn parse_comments() {
        let comments = decl_comments(
            r#"
let a = 0;

// one-line
let b = 1;

// ignored due to line-break

// one
// two
let c = 2;
// ignored due to trailing comment

/* line block
comment */
let d = 3;

/* encroaching on decl
*/ let e = 4;

/* same line as decl */ let f = 5;

// line comment
/* followed by block
comment */
let g = 6;

/* block */
// line one
// line two
let h = 7;

// line one
/* block */
// line two
let i = 8;

/**
 * JSDoc comment
 * multiple lines
 */
 let j = 9;
            "#,
        );
        assert_eq!(
            comments,
            vec![
                None,
                Some("one-line".into()),
                Some("one\ntwo".into()),
                Some("line block\ncomment".into()),
                Some("encroaching on decl".into()),
                Some("same line as decl".into()),
                Some("followed by block\ncomment".into()),
                Some("line one\nline two".into()),
                Some("line two".into()),
                Some("JSDoc comment\nmultiple lines".into()),
            ]
        );
    }

    #[test]
    fn parse_jsdoc() {
        let comments = decl_comments(
            r#"
/**
 * JSDoc comment
 * multiple lines
 */
let i = 0;
            "#,
        );
        assert_eq!(
            comments,
            vec![Some("JSDoc comment\nmultiple lines".into()),]
        );
    }
}
