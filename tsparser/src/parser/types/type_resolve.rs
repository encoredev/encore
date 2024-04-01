use anyhow::Result;
use std::rc::Rc;
use swc_common::sync::Lrc;
use swc_ecma_ast as ast;

use crate::parser::module_loader;
use crate::parser::types::object::{Ctx, ObjectKind};
use crate::parser::types::Object;

use super::typ::*;

#[derive(Debug)]
pub struct TypeChecker<'a> {
    ctx: Ctx<'a>,
}

impl<'a> TypeChecker<'a> {
    pub fn new(loader: Lrc<module_loader::ModuleLoader<'a>>) -> Self {
        Self {
            ctx: Ctx::new(loader),
        }
    }

    pub fn ctx(&self) -> &Ctx<'a> {
        &self.ctx
    }

    pub fn resolve(&self, module: Lrc<module_loader::Module>, expr: &ast::TsType) -> Result<Type> {
        self.ctx.resolve(module, expr)
    }

    pub fn resolve_obj(
        &self,
        module: Lrc<module_loader::Module>,
        expr: &ast::Expr,
    ) -> Result<Option<Rc<Object>>> {
        self.ctx.resolve_obj(module, expr)
    }
}

pub(super) fn resolve_type(ctx: &Ctx, typ: &ast::TsType) -> Result<Type> {
    match typ {
            ast::TsType::TsKeywordType(tt) => keyword(ctx, tt),
            ast::TsType::TsThisType(_) => Ok(Type::This),
            ast::TsType::TsArrayType(tt) => array(ctx, tt),
            ast::TsType::TsTupleType(tt) => tuple(ctx, tt),
            ast::TsType::TsUnionOrIntersectionType(ast::TsUnionOrIntersectionType::TsUnionType(tt)) => union(ctx, tt),
            ast::TsType::TsUnionOrIntersectionType(ast::TsUnionOrIntersectionType::TsIntersectionType(tt)) => intersection(ctx, tt),
            ast::TsType::TsParenthesizedType(tt) => resolve_type(ctx, &tt.type_ann),
            ast::TsType::TsTypeLit(tt) => type_lit(ctx, &tt),
            ast::TsType::TsTypeRef(tt) => type_ref(ctx, &tt),
            ast::TsType::TsOptionalType(tt) => optional(ctx, tt),
            ast::TsType::TsTypeQuery(tt) => type_query(ctx, tt),

            ast::TsType::TsConditionalType(tt) => conditional(ctx, tt),
            ast::TsType::TsLitType(tt) => lit_type(ctx, &tt), // https://www.typescriptlang.org/docs/handbook/2/template-literal-types.html

            ast::TsType::TsFnOrConstructorType(_)
            | ast::TsType::TsRestType(_) // same?
            | ast::TsType::TsTypeOperator(_) // keyof, etc
            | ast::TsType::TsIndexedAccessType(_) // https://www.typescriptlang.org/docs/handbook/2/indexed-access-types.html#handbook-content
            | ast::TsType::TsMappedType(_) // https://www.typescriptlang.org/docs/handbook/2/mapped-types.html
            | ast::TsType::TsTypePredicate(_) // https://www.typescriptlang.org/docs/handbook/2/narrowing.html#using-type-predicates, https://www.typescriptlang.org/docs/handbook/2/classes.html#this-based-type-guards
            | ast::TsType::TsImportType(_) // ??
            | ast::TsType::TsInferType(_) => {
                anyhow::bail!("unsupported: {:#?}", typ)
            }, // typeof
        }
}

fn array(ctx: &Ctx, tt: &ast::TsArrayType) -> Result<Type> {
    Ok(Type::Array(Box::new(resolve_type(ctx, &tt.elem_type)?)))
}

fn optional(ctx: &Ctx, tt: &ast::TsOptionalType) -> Result<Type> {
    Ok(Type::Optional(Box::new(resolve_type(ctx, &tt.type_ann)?)))
}

fn tuple(ctx: &Ctx, tuple: &ast::TsTupleType) -> Result<Type> {
    let mut typs = Vec::with_capacity(tuple.elem_types.len());
    for elem in &tuple.elem_types {
        if elem.label.is_some() {
            // As far as I can tell labels don't actually impact type-checking
            // at all, so we can ignore them.
            // See https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-0.html.
        }

        typs.push(resolve_type(ctx, &elem.ty)?);
    }

    Ok(Type::Tuple(typs))
}

fn union(ctx: &Ctx, union_type: &ast::TsUnionType) -> Result<Type> {
    // TODO handle unifying e.g. "string | 'foo'" into "string"
    Ok(Type::Union(
        union_type
            .types
            .iter()
            .map(|t| resolve_type(ctx, t))
            .collect::<Result<Vec<_>>>()?,
    ))
}

fn type_lit(ctx: &Ctx, type_lit: &ast::TsTypeLit) -> Result<Type> {
    let mut fields: Vec<InterfaceField> = Vec::with_capacity(type_lit.members.len());
    for m in &type_lit.members {
        match m {
            ast::TsTypeElement::TsPropertySignature(p) => {
                let name = match *p.key {
                    ast::Expr::Ident(ref i) => i.sym.as_ref().to_string(),
                    _ => anyhow::bail!("unsupported property key: {:#?}", type_lit),
                };

                if p.type_params.is_some() {
                    anyhow::bail!("unsupported type params: {:#?}", p);
                }
                if p.type_ann.is_none() {
                    anyhow::bail!("unsupported missing type: {:#?}", p);
                }

                fields.push(InterfaceField {
                    name,
                    typ: resolve_type(ctx, p.type_ann.as_ref().unwrap().type_ann.as_ref())?,
                    optional: p.optional,
                });
            }
            ast::TsTypeElement::TsMethodSignature(_) => {
                anyhow::bail!("unsupported: {:#?}", type_lit);
            }
            ast::TsTypeElement::TsIndexSignature(_) => {
                anyhow::bail!("unsupported: {:#?}", type_lit);
            }
            ast::TsTypeElement::TsCallSignatureDecl(_) => {
                anyhow::bail!("unsupported: {:#?}", type_lit);
            }
            ast::TsTypeElement::TsConstructSignatureDecl(_) => {
                anyhow::bail!("unsupported: {:#?}", type_lit);
            }
            ast::TsTypeElement::TsGetterSignature(_) => {
                anyhow::bail!("unsupported: {:#?}", type_lit);
            }
            ast::TsTypeElement::TsSetterSignature(_) => {
                anyhow::bail!("unsupported: {:#?}", type_lit);
            }
        }
    }

    Ok(Type::Interface(Interface { fields }))
}

fn lit_type(_ctx: &Ctx, lit_type: &ast::TsLitType) -> Result<Type> {
    Ok(Type::Literal(match &lit_type.lit {
        ast::TsLit::Str(val) => Literal::String(val.value.to_string()),
        ast::TsLit::Number(val) => Literal::Number(val.value),
        ast::TsLit::Bool(val) => Literal::Boolean(val.value),
        ast::TsLit::BigInt(val) => Literal::BigInt(val.value.to_string()),
        ast::TsLit::Tpl(_) => {
            anyhow::bail!("unsupported: template literal expression {:#?}", lit_type)
        }
    }))
}

fn type_ref(ctx: &Ctx, typ: &ast::TsTypeRef) -> Result<Type> {
    let ident: &ast::Ident = match typ.type_name {
        ast::TsEntityName::Ident(ref i) => i,
        ast::TsEntityName::TsQualifiedName(_) => anyhow::bail!("unsupported: {:#?}", typ),
    };

    let mut type_arguments =
        Vec::with_capacity(typ.type_params.as_ref().map_or(0, |p| p.params.len()));
    if let Some(params) = &typ.type_params {
        for p in &params.params {
            type_arguments.push(resolve_type(ctx, p)?);
        }
    }

    let obj = ctx.resolve_ident(ident)?;
    Ok(match &obj.kind {
        ObjectKind::TypeName(_) | ObjectKind::Enum(_) | ObjectKind::Class(_) => {
            Type::Named(Named {
                obj,
                type_arguments,
            })
        }
        ObjectKind::Var(_) | ObjectKind::Using(_) | ObjectKind::Func(_) => {
            anyhow::bail!("value used as type")
        }
        ObjectKind::Module(_) => anyhow::bail!("module used as type"),
        ObjectKind::Namespace(_) => anyhow::bail!("namespace used as type"),
    })
}

/// Resolves the typeof operator.
fn type_query(ctx: &Ctx, typ: &ast::TsTypeQuery) -> Result<Type> {
    if typ.type_args.is_some() {
        anyhow::bail!("unsupported: typeof with type args: {:#?}", typ);
    }

    match &typ.expr_name {
        ast::TsTypeQueryExpr::TsEntityName(ast::TsEntityName::Ident(ident)) => {
            let obj = ctx.resolve_ident(ident)?;
            anyhow::bail!("typeof not yet implemented: {:#?}", obj);
            // Ok(match &*obj {
            //     Object::TypeName(tt) | Object::Enum(_) | Object::Class(_) => Type::Named(Named {
            //         obj,
            //         type_arguments,
            //     }),
            //     Object::Var(_) | Object::Using(_) | Object::Func(_) => {
            //         anyhow::bail!("value used as type")
            //     }
            //     Object::Module(_) => anyhow::bail!("module used as type"),
            //     Object::Namespace(_) => anyhow::bail!("namespace used as type"),
            // })
        }
        _ => anyhow::bail!("unsupported: typeof with non-ident: {:#?}", typ),
    }
}

// https://www.typescriptlang.org/docs/handbook/2/conditional-types.html
fn conditional(ctx: &Ctx, typ: &ast::TsConditionalType) -> Result<Type> {
    // TODO For now just return the true branch.
    resolve_type(ctx, &typ.true_type)
}

fn intersection(_ctx: &Ctx, _typ: &ast::TsIntersectionType) -> Result<Type> {
    todo!()
    // let types = typ
    //     .types
    //     .iter()
    //     .map(|t| self.typ(t))
    //     .collect::<Result<Vec<_>>>()?
}

fn keyword(_ctx: &Ctx, typ: &ast::TsKeywordType) -> Result<Type> {
    let basic: Basic = match typ.kind {
        ast::TsKeywordTypeKind::TsAnyKeyword => Basic::Any,
        ast::TsKeywordTypeKind::TsUnknownKeyword => Basic::Unknown,
        ast::TsKeywordTypeKind::TsNumberKeyword => Basic::Number,
        ast::TsKeywordTypeKind::TsObjectKeyword => Basic::Object,
        ast::TsKeywordTypeKind::TsBooleanKeyword => Basic::Boolean,
        ast::TsKeywordTypeKind::TsBigIntKeyword => Basic::BigInt,
        ast::TsKeywordTypeKind::TsStringKeyword => Basic::String,
        ast::TsKeywordTypeKind::TsSymbolKeyword => Basic::Symbol,
        ast::TsKeywordTypeKind::TsVoidKeyword => Basic::Void,
        ast::TsKeywordTypeKind::TsUndefinedKeyword => Basic::Undefined,
        ast::TsKeywordTypeKind::TsNullKeyword => Basic::Null,
        ast::TsKeywordTypeKind::TsNeverKeyword => Basic::Never,
        ast::TsKeywordTypeKind::TsIntrinsicKeyword => {
            anyhow::bail!("unimplemented: {:#?}", typ);
        }
    };
    Ok(Type::Basic(basic))
}

pub(super) fn interface_decl(ctx: &Ctx, decl: &ast::TsInterfaceDecl) -> Result<Type> {
    if decl.extends.len() > 0 {
        anyhow::bail!("extends not yet supported");
    } else if decl.type_params.is_some() {
        anyhow::bail!("type params not yet supported");
    }
    resolve_type(
        ctx,
        &ast::TsType::TsTypeLit(ast::TsTypeLit {
            span: decl.span,
            members: decl.body.body.clone(),
        }),
    )
}

pub(super) fn resolve_expr_type(ctx: &Ctx, expr: &ast::Expr) -> Result<Type> {
    Ok(match expr {
        ast::Expr::This(_) => Type::This,
        ast::Expr::Array(lit) => resolve_array_lit_type(ctx, lit)?,
        ast::Expr::Object(lit) => resolve_object_lit_type(ctx, lit)?,
        ast::Expr::Fn(_) => anyhow::bail!("unsupported fn expr: {:#?}", expr),
        ast::Expr::Unary(expr) => match expr.op {
            // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/void
            ast::UnaryOp::Void => Type::Basic(Basic::Undefined),

            // This is the JavaScript typeof operator, not the TypeScript typeof operator.
            // See https://www.typescriptlang.org/docs/handbook/2/typeof-types.html
            ast::UnaryOp::TypeOf => Type::Basic(Basic::String),

            ast::UnaryOp::Minus
            | ast::UnaryOp::Plus
            | ast::UnaryOp::Bang
            | ast::UnaryOp::Tilde
            | ast::UnaryOp::Delete => resolve_expr_type(ctx, &expr.arg)?,
        },
        ast::Expr::Update(expr) => resolve_expr_type(ctx, &expr.arg)?,
        ast::Expr::Bin(expr) => {
            // TODO handle this correctly.
            let left = resolve_expr_type(ctx, &expr.left)?;
            let right = resolve_expr_type(ctx, &expr.right)?;
            let unified = left.unify(&right);
            unified.unwrap_or(left)
        }
        ast::Expr::Assign(expr) => resolve_expr_type(ctx, &expr.right)?,
        ast::Expr::Member(expr) => resolve_member_expr_type(ctx, expr)?,
        ast::Expr::SuperProp(_) => anyhow::bail!("unsupported super expr: {:#?}", expr),
        ast::Expr::Cond(cond) => {
            let left = resolve_expr_type(ctx, &cond.cons)?;
            let right = resolve_expr_type(ctx, &cond.alt)?;
            left.unify_or_union(&right)
        }
        ast::Expr::Call(expr) => anyhow::bail!("unsupported call expr: {:#?}", expr),
        ast::Expr::New(expr) => {
            // The type of a class instance is the same as the class itself.
            // TODO type args
            resolve_expr_type(ctx, &expr.callee)?
        }
        ast::Expr::Seq(expr) => match expr.exprs.last() {
            Some(expr) => resolve_expr_type(ctx, expr)?,
            None => Type::Basic(Basic::Never),
        },
        ast::Expr::Ident(expr) => {
            let obj = ctx.resolve_ident(expr)?;
            // ctx.obj_type(obj)?
            Type::Named(Named {
                obj,
                type_arguments: vec![],
            })
        }
        ast::Expr::PrivateName(expr) => {
            let obj = ctx.resolve_ident(&expr.id)?;
            // ctx.obj_type(obj)?
            Type::Named(Named {
                obj,
                type_arguments: vec![],
            })
        }
        ast::Expr::Lit(expr) => match &expr {
            ast::Lit::Str(_) => Type::Basic(Basic::String),
            ast::Lit::Bool(_) => Type::Basic(Basic::Boolean),
            ast::Lit::Null(_) => Type::Basic(Basic::Null),
            ast::Lit::Num(_) => Type::Basic(Basic::Number),
            ast::Lit::BigInt(_) => Type::Basic(Basic::BigInt),
            ast::Lit::Regex(_) => anyhow::bail!("unsupported regex: {:#?}", expr),
            ast::Lit::JSXText(_) => anyhow::bail!("unsupported jsx text: {:#?}", expr),
        },
        ast::Expr::Tpl(_) => Type::Basic(Basic::String),
        ast::Expr::TaggedTpl(_) => anyhow::bail!("unsupported tagged tpl: {:#?}", expr),
        ast::Expr::Arrow(_) => anyhow::bail!("unsupported arrow expr: {:#?}", expr),
        ast::Expr::Class(_) => anyhow::bail!("unsupported class expr: {:#?}", expr),
        ast::Expr::Yield(expr) => match &expr.arg {
            Some(arg) => resolve_expr_type(ctx, arg)?,
            None => Type::Basic(Basic::Undefined),
        },
        ast::Expr::MetaProp(expr) => match expr.kind {
            // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target
            ast::MetaPropKind::NewTarget => Type::Union(vec![
                Type::Basic(Basic::Undefined),
                Type::Signature(Signature {}),
            ]),
            // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/import.meta
            ast::MetaPropKind::ImportMeta => Type::Basic(Basic::Object),
        },
        ast::Expr::Await(expr) => {
            let prom = resolve_expr_type(ctx, &expr.arg)?;
            if let Type::Named(named) = &prom {
                if named.obj.name.as_deref() == Some("Promise")
                    && ctx.is_universe(named.obj.module_id)
                {
                    if let Some(t) = named.type_arguments.get(0) {
                        return Ok(t.clone());
                    }
                }
            }
            Type::Basic(Basic::Unknown)
        }

        ast::Expr::Paren(expr) => resolve_expr_type(ctx, &expr.expr)?,

        ast::Expr::JSXMember(_)
        | ast::Expr::JSXNamespacedName(_)
        | ast::Expr::JSXEmpty(_)
        | ast::Expr::JSXElement(_)
        | ast::Expr::JSXFragment(_) => Type::Basic(Basic::Never),

        // <T>foo
        ast::Expr::TsTypeAssertion(expr) => resolve_type(ctx, &expr.type_ann)?,
        // foo as T
        ast::Expr::TsAs(expr) => resolve_type(ctx, &expr.type_ann)?,

        ast::Expr::TsConstAssertion(expr) => resolve_expr_type(ctx, &expr.expr)?,

        // https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-9.html
        ast::Expr::TsSatisfies(expr) => resolve_expr_type(ctx, &expr.expr)?,

        ast::Expr::TsInstantiation(expr) => {
            // TODO handle type args
            resolve_expr_type(ctx, &expr.expr)?
        }

        // The "foo!" operator
        ast::Expr::TsNonNull(expr) => {
            let base = resolve_expr_type(ctx, &expr.expr)?;
            match base {
                Type::Optional(typ) => *typ,
                Type::Union(types) => {
                    let non_null = types
                        .into_iter()
                        .filter(|t| {
                            !matches!(t, Type::Basic(Basic::Undefined) | Type::Basic(Basic::Null))
                        })
                        .collect::<Vec<_>>();
                    match &non_null.len() {
                        0 => Type::Basic(Basic::Never),
                        1 => non_null[0].clone(),
                        _ => Type::Union(non_null),
                    }
                }
                _ => base,
            }
        }

        ast::Expr::OptChain(_) => anyhow::bail!("opt chain not yet supported: {:#?}", expr),
        ast::Expr::Invalid(_) => Type::Basic(Basic::Never),
    })
}

fn resolve_array_lit_type(ctx: &Ctx, lit: &ast::ArrayLit) -> Result<Type> {
    let elem_types = Vec::with_capacity(lit.elems.len());

    // Track the current element type.
    let mut elem_type: Option<Type> = None;

    for elem in &lit.elems {
        if let Some(elem) = elem {
            let mut base = resolve_expr_type(ctx, &elem.expr)?;
            if elem.spread.is_some() {
                // The type of [...["a"]] is string[].
                match base {
                    Type::Array(arr) => {
                        base = *arr;
                    }
                    _ => {}
                }
            }

            match &elem_type {
                Some(Type::Union(_elem_types)) => {}
                Some(typ) => {
                    elem_type = Some(Type::Union(vec![typ.clone(), base]));
                }
                None => {
                    elem_type = Some(base);
                }
            }
        }
    }

    Ok(Type::Union(elem_types))
}

fn resolve_object_lit_type(ctx: &Ctx, lit: &ast::ObjectLit) -> Result<Type> {
    let mut fields = Vec::with_capacity(lit.props.len());

    for prop in &lit.props {
        match prop {
            ast::PropOrSpread::Prop(prop) => {
                let kv = match prop.as_ref() {
                    ast::Prop::Shorthand(id) => {
                        let obj = ctx.resolve_ident(&id)?;
                        let obj_type = ctx.obj_type(obj)?;
                        (id.sym.as_ref().to_string(), obj_type)
                    }
                    ast::Prop::KeyValue(kv) => {
                        let key = propname_to_string(ctx, &kv.key)?;
                        let val_typ = resolve_expr_type(ctx, &kv.value)?;
                        (key, val_typ)
                    }
                    ast::Prop::Assign(_) => {
                        anyhow::bail!("unsupported assign in object literal")
                    }
                    ast::Prop::Getter(prop) => {
                        let key = propname_to_string(ctx, &prop.key)?;
                        // We can't figure out the value type here as it relies on
                        // doing type analysis on the function body.
                        (key, Type::Basic(Basic::Unknown))
                    }
                    ast::Prop::Setter(prop) => {
                        let key = propname_to_string(ctx, &prop.key)?;
                        // We can't figure out the value type here as it relies on
                        // doing type analysis on the function body.
                        (key, Type::Basic(Basic::Unknown))
                    }
                    ast::Prop::Method(prop) => {
                        let key = propname_to_string(ctx, &prop.key)?;
                        // We can't figure out the value type here as it relies on
                        // doing type analysis on the function body.
                        (key, Type::Basic(Basic::Unknown))
                    }
                };
                fields.push(InterfaceField {
                    name: kv.0,
                    typ: kv.1,
                    optional: false,
                });
            }
            ast::PropOrSpread::Spread(spread) => {
                let typ = resolve_expr_type(ctx, &spread.expr)?;
                match typ {
                    Type::Interface(interface) => {
                        fields.extend(interface.fields);
                    }
                    _ => anyhow::bail!("unsupported spread type: {:#?}", typ),
                }
            }
        }
    }

    Ok(Type::Interface(Interface { fields }))
}

fn propname_to_string(ctx: &Ctx, prop: &ast::PropName) -> Result<String> {
    Ok(match prop {
        ast::PropName::Ident(id) => id.sym.as_ref().to_string(),
        ast::PropName::Str(str) => str.value.to_string(),
        ast::PropName::Num(num) => num.value.to_string(),
        ast::PropName::BigInt(bigint) => bigint.value.to_string(),
        ast::PropName::Computed(expr) => match resolve_expr_type(ctx, &expr.expr)? {
            Type::Literal(lit) => match lit {
                Literal::String(str) => str,
                Literal::Number(num) => num.to_string(),
                _ => anyhow::bail!("unsupported: {:#?}", lit),
            },
            _ => anyhow::bail!("unsupported: {:#?}", expr),
        },
    })
}

fn resolve_member_expr_type(ctx: &Ctx, expr: &ast::MemberExpr) -> Result<Type> {
    let obj_type = resolve_expr_type(ctx, &expr.obj)?;
    resolve_sel_type(ctx, obj_type, &expr.prop)
}

fn resolve_sel_type(ctx: &Ctx, obj_type: Type, prop: &ast::MemberProp) -> Result<Type> {
    Ok(match obj_type {
        Type::Basic(_) => anyhow::bail!("unsupported member on basic type: {:#?}", obj_type),
        Type::Literal(_) => anyhow::bail!("unsupported member on literal type: {:#?}", obj_type),
        Type::Array(_) => anyhow::bail!("unsupported member on array type: {:#?}", obj_type),
        Type::Tuple(_) => anyhow::bail!("unsupported member on tuple type: {:#?}", obj_type),
        Type::Union(_) => anyhow::bail!("unsupported member on union type: {:#?}", obj_type),
        Type::Signature(_) => {
            anyhow::bail!("unsupported member on signature type: {:#?}", obj_type)
        }
        Type::Optional(_) => anyhow::bail!("unsupported member on optional type: {:#?}", obj_type),
        Type::This => anyhow::bail!("unsupported member on 'this' type: {:#?}", obj_type),
        Type::TypeArgument(_) => {
            anyhow::bail!("unsupported member on TypeArgument type: {:#?}", obj_type)
        }
        Type::Class(_) => anyhow::bail!("unsupported member on class type: {:#?}", obj_type),
        Type::Interface(tt) => {
            for field in tt.fields {
                let matches = match prop {
                    ast::MemberProp::Ident(i) => field.name == i.sym.as_ref(),
                    ast::MemberProp::PrivateName(i) => field.name == i.id.sym.as_ref(),
                    ast::MemberProp::Computed(i) => match resolve_expr_type(ctx, &i.expr)? {
                        Type::Literal(lit) => match lit {
                            Literal::String(str) => field.name == str,
                            Literal::Number(num) => num.to_string() == field.name,
                            _ => false,
                        },
                        _ => false,
                    },
                };
                if matches {
                    return Ok(field.typ);
                }
            }
            Type::Basic(Basic::Never)
        }
        Type::Named(named) => {
            let typ = ctx.obj_type(named.obj.clone())?;
            resolve_sel_type(ctx, typ, prop)?
        }
    })
}
