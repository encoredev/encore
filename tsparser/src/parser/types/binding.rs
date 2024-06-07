use swc_ecma_ast as ast;

pub(super) struct BindingPat {
    /// The ast id, for id tracking.
    pub id: ast::Id,

    /// The variable name this is bound to.
    pub name: String,

    /// The type annotation, if any.
    pub type_ann: Option<ast::TsTypeAnn>,

    /// The default value if the destructuring expression fails to match.
    #[allow(dead_code)]
    pub default: Option<ast::Expr>,

    /// The destructuring expression to evaluate the RHS against
    /// to arrive at the value.
    pub destructure_path: Vec<DestructuringExpr>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub(super) enum DestructuringExpr {
    /// The whole expression.
    Full,
    /// A single entry in an array with the given index.
    ArrayIndex(usize),
    /// The remainder of an array; e.g. `...xs`, starting from the given index (inclusive).
    ArrayRest(usize),
    /// A single entry in an object with the given key.
    ObjectKey(DestructuringObjectKey),

    /// The remainder of an object; e.g. `...xs`.
    ObjectRest {
        /// The keys that have already been destructured.
        except: Vec<String>,
    },
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub(super) enum DestructuringObjectKey {
    /// A literal key.
    Ident(String),
    /// A computed key.
    Computed(ast::Expr),
}

pub(super) fn bindings(pat: &ast::Pat) -> Vec<BindingPat> {
    /// Compute sub_bindings for a pattern, stripping the DestructuringExpr::Full
    /// from the head of the destructure_path. This is used so that
    /// we don't end up with "full" expressions other than for "let x = ...".
    fn sub_bindings(pat: &ast::Pat) -> Vec<BindingPat> {
        let mut result = bindings(pat);
        for b in &mut result {
            if matches!(b.destructure_path.first(), Some(DestructuringExpr::Full)) {
                b.destructure_path.remove(0);
            }
        }
        result
    }

    match pat {
        ast::Pat::Ident(ident) => {
            // A basic identifier, e.g. "let x = ..."
            vec![BindingPat {
                id: ident.id.to_id(),
                name: ident.id.sym.as_ref().to_string(),
                type_ann: ident.type_ann.as_ref().map(|ann| *ann.clone()),
                default: None,
                destructure_path: vec![DestructuringExpr::Full],
            }]
        }

        ast::Pat::Array(arr) => {
            // An array destructuring expression, e.g. "let [x, y] = ..."
            let mut result = Vec::with_capacity(arr.elems.len());
            for (idx, elem) in arr.elems.iter().enumerate() {
                // Skip over None elems (e.g. "let [x, , y] = ...")
                let Some(elem) = elem else { continue };

                match elem {
                    // Handle the rest expression here as we know we're dealing
                    // with an array destructuring.
                    ast::Pat::Rest(rest) => {
                        // A rest expression, e.g. "...xs"
                        let rest_bindings = sub_bindings(&rest.arg);
                        for mut rb in rest_bindings {
                            // Add the rest expression.
                            rb.destructure_path
                                .insert(0, DestructuringExpr::ArrayRest(idx));
                            result.push(rb);
                        }
                    }

                    _ => {
                        // For every child binding, add it to the result
                        // while wrapping it in an array index expression.
                        let elem_bindings = sub_bindings(elem);
                        for mut eb in elem_bindings {
                            eb.destructure_path
                                .insert(0, DestructuringExpr::ArrayIndex(idx));
                            result.push(eb);
                        }
                    }
                }
            }

            result
        }

        ast::Pat::Object(obj) => {
            // An object destructuring expression, e.g. "let {x, y} = ..."
            let mut result = Vec::with_capacity(obj.props.len());
            let mut rest: Option<&ast::RestPat> = None;
            for prop in obj.props.iter() {
                match prop {
                    ast::ObjectPatProp::KeyValue(kv) => {
                        // E.g. "let {x: y} = ...", indicates a nested destructuring
                        // where x is not actually bound.
                        let bindings = sub_bindings(&kv.value);

                        // Figure out what the prop key is.
                        let obj_key: DestructuringObjectKey = match &kv.key {
                            ast::PropName::Ident(id) => {
                                DestructuringObjectKey::Ident(id.sym.to_string())
                            }
                            ast::PropName::Str(str) => {
                                DestructuringObjectKey::Ident(str.value.to_string())
                            }
                            ast::PropName::Num(num) => {
                                DestructuringObjectKey::Ident(num.value.to_string())
                            }
                            ast::PropName::BigInt(big) => {
                                DestructuringObjectKey::Ident(big.value.to_string())
                            }
                            ast::PropName::Computed(computed) => {
                                DestructuringObjectKey::Computed(*computed.expr.clone())
                            }
                        };

                        for mut b in bindings {
                            b.destructure_path
                                .insert(0, DestructuringExpr::ObjectKey(obj_key.clone()));
                            result.push(b);
                        }
                    }

                    ast::ObjectPatProp::Assign(assign) => {
                        // E.g. "let {x} = ..." or "let { x = default_value } = ...".
                        // Indicates a new bind named x, optionally with a default value.
                        let key = assign.key.sym.to_string();
                        result.push(BindingPat {
                            id: assign.key.to_id(),
                            name: key.clone(),
                            type_ann: None,
                            default: assign.value.as_ref().map(|v| *v.clone()),
                            destructure_path: vec![DestructuringExpr::ObjectKey(
                                DestructuringObjectKey::Ident(key),
                            )],
                        });
                    }

                    ast::ObjectPatProp::Rest(r) => {
                        // E.g. "let {x, ...xs} = ...", indicates a rest expression.
                        rest = Some(r);
                    }
                }
            }

            // If we have a rest expression, compute the except list.
            if let Some(rest) = rest {
                // Determine the keys that have already been destructured.
                let mut except = Vec::with_capacity(result.len());
                for b in &result {
                    if let Some(DestructuringExpr::ObjectKey(DestructuringObjectKey::Ident(id))) =
                        b.destructure_path.first()
                    {
                        except.push(id.clone());
                    }
                }

                let rest_bindings = sub_bindings(&rest.arg);
                for mut b in rest_bindings {
                    b.destructure_path.insert(
                        0,
                        DestructuringExpr::ObjectRest {
                            except: except.clone(),
                        },
                    );
                    result.push(b);
                }
            }

            result
        }

        ast::Pat::Assign(_assign) => {
            // TODO what does this even mean?
            todo!("assign pattern")
        }

        ast::Pat::Rest(_) => {
            // This shouldn't happen here as we handle it in the array and object cases directly.
            vec![]
        }
        ast::Pat::Invalid(_) | ast::Pat::Expr(_) => {
            // These shouldn't happen; ignore them.
            vec![]
        }
    }
}

#[cfg(test)]
mod tests {
    use crate::parser::types::binding::{DestructuringExpr, DestructuringObjectKey};
    use crate::testutil::testparse::test_parse;

    #[test]
    fn test_bindings() {
        let tests = vec![
            ("x", vec![("x", vec![DestructuringExpr::Full])]),
            (
                "{x}",
                vec![(
                    "x",
                    vec![DestructuringExpr::ObjectKey(DestructuringObjectKey::Ident(
                        "x".into(),
                    ))],
                )],
            ),
            (
                "{x: y}",
                vec![(
                    "y",
                    vec![DestructuringExpr::ObjectKey(DestructuringObjectKey::Ident(
                        "x".into(),
                    ))],
                )],
            ),
            (
                "{a, ...rest, b: c}",
                vec![
                    (
                        "a",
                        vec![DestructuringExpr::ObjectKey(DestructuringObjectKey::Ident(
                            "a".into(),
                        ))],
                    ),
                    (
                        "c",
                        vec![DestructuringExpr::ObjectKey(DestructuringObjectKey::Ident(
                            "b".into(),
                        ))],
                    ),
                    (
                        "rest",
                        vec![DestructuringExpr::ObjectRest {
                            except: vec!["a".into(), "b".into()],
                        }],
                    ),
                ],
            ),
            (
                "[, a, , b, ...rest]",
                vec![
                    ("a", vec![DestructuringExpr::ArrayIndex(1)]),
                    ("b", vec![DestructuringExpr::ArrayIndex(3)]),
                    ("rest", vec![DestructuringExpr::ArrayRest(4)]),
                ],
            ),
        ];

        for (expr, want) in tests {
            let stmt = format!("let {} = 1;", expr);
            let module = test_parse(&stmt);
            let var = module.ast.body[0]
                .as_stmt()
                .unwrap()
                .as_decl()
                .unwrap()
                .as_var()
                .unwrap();
            let got = super::bindings(&var.decls[0].name);
            assert_eq!(got.len(), want.len());
            for (got, want) in got.iter().zip(want.iter()) {
                assert_eq!(got.name, want.0, "expr: {}", expr);
                assert_eq!(got.destructure_path, want.1, "expr: {}", expr);
            }
        }
    }
}
