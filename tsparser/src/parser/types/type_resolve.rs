use std::borrow::Cow;
use std::borrow::Cow::{Borrowed, Owned};
use std::cell::OnceCell;
use std::fmt::Debug;
use std::ops::Deref;
use std::rc::Rc;

use swc_common::errors::HANDLER;
use swc_common::sync::Lrc;
use swc_common::{BytePos, Span, Spanned};
use swc_ecma_ast as ast;

use crate::parser::module_loader::ModuleId;
use crate::parser::types::object::{CheckState, ObjectKind, ResolveState, TypeNameDecl};
use crate::parser::types::{typ, Object};
use crate::parser::{module_loader, Range};

use super::resolved::{Resolved, Resolved::*};
use super::typ::*;

#[derive(Debug)]
pub struct TypeChecker {
    ctx: ResolveState,
}

impl TypeChecker {
    pub fn new(loader: Lrc<module_loader::ModuleLoader>) -> Self {
        Self {
            ctx: ResolveState::new(loader),
        }
    }

    pub fn state(&self) -> &ResolveState {
        &self.ctx
    }

    pub fn resolve_type(&self, module: Lrc<module_loader::Module>, expr: &ast::TsType) -> Type {
        // Ensure the module is initialized.
        let module_id = module.id;
        _ = self.ctx.get_or_init_module(module);

        let ctx = Ctx::new(&self.ctx, module_id);
        let typ = ctx.typ(expr);
        match ctx.concrete(&typ) {
            New(typ) => typ,
            Changed(typ) => typ.clone(),
            Same(_) => typ,
        }
    }

    pub fn concrete(&self, module_id: ModuleId, typ: &Type) -> Type {
        // Ensure the module is initialized.
        let ctx = Ctx::new(&self.ctx, module_id);
        ctx.concrete(typ).into_owned()
    }

    pub fn underlying(&self, module_id: ModuleId, typ: &Type) -> Type {
        let ctx = Ctx::new(&self.ctx, module_id);
        ctx.underlying(&typ).into_owned()
    }

    pub fn resolve_obj(
        &self,
        module: Lrc<module_loader::Module>,
        expr: &ast::Expr,
    ) -> Option<Rc<Object>> {
        // Ensure the module is initialized.
        let module_id = module.id;
        _ = self.ctx.get_or_init_module(module);

        let ctx = Ctx::new(&self.ctx, module_id);
        ctx.resolve_obj(expr)
    }

    pub fn resolve_obj_type(&self, obj: &Object) -> Type {
        let ctx = Ctx::new(&self.ctx, obj.module_id);
        ctx.obj_type(obj)
    }
}

#[derive(Debug, Clone)]
pub struct Ctx<'a> {
    state: &'a ResolveState,

    /// The current module being resolved.
    module: ModuleId,

    /// The type parameters in the current type resolution scope.
    type_params: &'a [&'a ast::TsTypeParam],

    /// The type arguments in the current type resolution scope.
    type_args: &'a [Type],

    /// Context for the current mapped type being processed, if any.
    mapped_key_id: Option<ast::Id>,

    /// The mapped key type to substitute when concretising, if any.
    mapped_key_type: Option<&'a Type>,
}

impl<'a> Ctx<'a> {
    pub fn new(state: &'a ResolveState, module: ModuleId) -> Self {
        Self {
            state,
            module,
            type_params: &[],
            type_args: &[],
            mapped_key_id: None,
            mapped_key_type: None,
        }
    }

    pub fn state(&self) -> &ResolveState {
        self.state
    }

    fn with_module(self, module: ModuleId) -> Self {
        Self { module, ..self }
    }

    fn with_type_params(self, type_params: &'a [&'a ast::TsTypeParam]) -> Self {
        Self {
            type_params,
            ..self
        }
    }

    fn with_type_args(self, type_args: &'a [Type]) -> Self {
        Self { type_args, ..self }
    }

    fn with_mapped_key_id(self, mapped_key_id: Option<ast::Id>) -> Self {
        Self {
            mapped_key_id,
            ..self
        }
    }

    fn with_mapped_key_type(self, mapped_key_type: Option<&'a Type>) -> Self {
        Self {
            mapped_key_type,
            ..self
        }
    }
}

impl<'a> Ctx<'a> {
    pub fn typ(&self, typ: &ast::TsType) -> Type {
        match typ {
            ast::TsType::TsKeywordType(tt) => self.keyword(tt),
            ast::TsType::TsThisType(_) => Type::This,
            ast::TsType::TsArrayType(tt) => self.array(tt),
            ast::TsType::TsTupleType(tt) => self.tuple(tt),
            ast::TsType::TsUnionOrIntersectionType(ast::TsUnionOrIntersectionType::TsUnionType(tt)) => self.union(tt),
            ast::TsType::TsUnionOrIntersectionType(ast::TsUnionOrIntersectionType::TsIntersectionType(tt)) => self.intersection(tt),
            ast::TsType::TsParenthesizedType(tt) => self.typ(&tt.type_ann),
            ast::TsType::TsTypeLit(tt) => self.type_lit(&tt),
            ast::TsType::TsTypeRef(tt) => self.type_ref(&tt),
            ast::TsType::TsOptionalType(tt) => self.optional(tt),
            ast::TsType::TsTypeQuery(tt) => self.type_query(tt),

            ast::TsType::TsConditionalType(tt) => self.conditional(tt),
            ast::TsType::TsLitType(tt) => self.lit_type(&tt),
            ast::TsType::TsTypeOperator(tt) => self.type_op(&tt),
            ast::TsType::TsMappedType(tt) => self.mapped(&tt),
            ast::TsType::TsIndexedAccessType(tt) => self.indexed_access(tt),

            ast::TsType::TsFnOrConstructorType(_)
            | ast::TsType::TsRestType(_) // same?
            | ast::TsType::TsTypePredicate(_) // https://www.typescriptlang.org/docs/handbook/2/narrowing.html#using-type-predicates, https://www.typescriptlang.org/docs/handbook/2/classes.html#this-based-type-guards
            | ast::TsType::TsImportType(_) // ??
            | ast::TsType::TsInferType(_) => {
                HANDLER.with(|handler| handler.span_err(typ.span(), &format!("unsupported: {:#?}", typ)));
                Type::Basic(Basic::Never)
            }, // typeof
        }
    }

    pub fn types<'b, I: IntoIterator<Item = &'b ast::TsType>>(&self, types: I) -> Vec<Type> {
        types.into_iter().map(|t| self.typ(t)).collect()
    }

    pub fn btyp(&self, typ: &ast::TsType) -> Box<Type> {
        Box::new(self.typ(typ))
    }

    /// Resolves keyof, unique, readonly, etc.
    fn type_op(&self, tt: &ast::TsTypeOperator) -> Type {
        let underlying = self.typ(&tt.type_ann);
        match tt.op {
            ast::TsTypeOperatorOp::ReadOnly => underlying,
            ast::TsTypeOperatorOp::Unique => underlying,
            ast::TsTypeOperatorOp::KeyOf => self.keyof(&underlying),
        }
    }

    /// Resolves a mapped type, which represents another type being modified.
    /// https://www.typescriptlang.org/docs/handbook/2/mapped-types.html
    fn mapped(&self, tt: &ast::TsMappedType) -> Type {
        // [K in keyof T]: T[K]

        let Some(value_type) = &tt.type_ann else {
            HANDLER.with(|handler| handler.span_err(tt.span, "missing value type annotation"));
            return Type::Basic(Basic::Never);
        };

        let Some(in_type) = &tt.type_param.constraint else {
            HANDLER.with(|handler| handler.span_err(tt.span, "missing 'in' type annotation"));
            return Type::Basic(Basic::Never);
        };

        if let Some(name_type) = &tt.name_type {
            HANDLER.with(|handler| {
                handler.span_err(name_type.span(), "'as' type annotation not yet supported")
            });
            return Type::Basic(Basic::Never);
        };

        // First parse the "in" type.
        let in_type = self.btyp(in_type);

        // Next, introduce a nested ctx that introduces the "K" mapped type parameter.
        let nested = self
            .clone()
            // .with_type_args(&[])
            .with_mapped_key_id(Some(tt.type_param.name.to_id()));

        // Next, parse the value type.
        let value_type = nested.btyp(value_type);

        let optional = match tt.optional {
            None => None,
            Some(ast::TruePlusMinus::Plus | ast::TruePlusMinus::True) => Some(true),
            Some(ast::TruePlusMinus::Minus) => Some(false),
        };

        Type::Generic(Generic::Mapped(Mapped {
            in_type,
            value_type,
            optional,
        }))
    }

    // https://www.typescriptlang.org/docs/handbook/2/indexed-access-types.html#handbook-content
    fn indexed_access(&self, tt: &ast::TsIndexedAccessType) -> Type {
        let obj = self.typ(&tt.obj_type);
        let idx = self.typ(&tt.index_type);
        self.type_index(tt.span, &obj, &idx)
    }

    fn type_index(&self, span: Span, obj: &Type, idx: &Type) -> Type {
        match (obj, idx) {
            // If either obj or index is a generic type, we need to store it as a Generic::Index.
            pair @ ((Type::Generic(_), _) | (_, Type::Generic(_))) => Type::Generic(
                Generic::Index((Box::new(pair.0.clone()), Box::new(pair.1.clone()))),
            ),

            (Type::Named(named), idx) => {
                let underlying = named.underlying(self.state);
                self.type_index(span, &underlying, idx)
            }

            (obj, Type::Named(idx)) => {
                let underlying = idx.underlying(self.state);
                self.type_index(span, obj, &underlying)
            }

            // Otherwise, look up the concrete result.
            (Type::Interface(iface), idx) => match idx {
                Type::Literal(Literal::String(s)) => iface
                    .fields
                    .iter()
                    .find(|f| match &f.name {
                        FieldName::String(name) => *name == *s,
                        FieldName::Symbol(_) => false,
                    })
                    .map_or(Type::Basic(Basic::Never), |f| {
                        let typ = f.typ.clone();
                        // If the field is optional, wrap the type in Optional.
                        if f.optional {
                            Type::Optional(Box::new(typ))
                        } else {
                            typ
                        }
                    }),

                Type::Basic(Basic::String | Basic::Number) => iface
                    .index
                    .as_ref()
                    .map_or(Type::Basic(Basic::Never), |(_, value)| *value.clone()),

                _ => {
                    HANDLER.with(|handler| {
                        handler.span_err(span, "unsupported index access type operation")
                    });
                    Type::Basic(Basic::Never)
                }
            },

            (obj, idx) => {
                HANDLER.with(|handler| {
                    handler.span_err(
                        span,
                        &format!(
                            "unsupported indexed access type operation: obj {:#?} index {:#?}",
                            obj, idx
                        ),
                    )
                });
                Type::Basic(Basic::Never)
            }
        }
    }

    /// Given a type, produces a union type of the underlying keys,
    /// e.g. `keyof {foo: string; bar: number}` yields `"foo" | "bar"`.
    fn keyof(&self, typ: &Type) -> Type {
        match typ {
            Type::Basic(tt) => match tt {
                Basic::Any => Type::Union(vec![
                    Type::Basic(Basic::String),
                    Type::Basic(Basic::Number),
                    Type::Basic(Basic::Symbol),
                ]),

                // These should technically enumerate the built-in properties
                // on these types, but we haven't implemented that yet.
                Basic::String | Basic::Boolean | Basic::Number | Basic::BigInt | Basic::Symbol => {
                    Type::Union(vec![])
                }

                // keyof these yields never.
                Basic::Object
                | Basic::Undefined
                | Basic::Null
                | Basic::Void
                | Basic::Unknown
                | Basic::Never => Type::Basic(Basic::Never),
            },

            Type::Enum(tt) => Type::Union(
                tt.members
                    .iter()
                    .map(|m| Type::Literal(Literal::String(m.name.clone())))
                    .collect(),
            ),

            // These should technically enumerate the built-in properties
            // on these types, but we haven't implemented that yet.
            Type::Array(_) | Type::Tuple(_) => Type::Union(vec![]),

            Type::Interface(interface) => {
                let keys = interface
                    .fields
                    .iter()
                    .filter_map(|f| match &f.name {
                        FieldName::String(name) => {
                            Some(Type::Literal(Literal::String(name.clone())))
                        }
                        FieldName::Symbol(_) => None,
                    })
                    .collect();
                Type::Union(keys)
            }

            Type::Named(_) => {
                let underlying = self.underlying(typ);
                self.keyof(&underlying)
            }

            Type::Class(_) => {
                HANDLER.with(|handler| handler.err("keyof ClassType not yet supported"));
                Type::Basic(Basic::Never)
            }

            Type::Optional(typ) => self.keyof(&typ),
            Type::Union(types) => {
                let res: Vec<_> = types.into_iter().map(|t| self.keyof(t)).collect();
                Type::Union(res)
            }

            // keyof "blah" is the same as keyof string, which should yield all properties.
            Type::Literal(_) => Type::Union(vec![]),

            Type::This => Type::Basic(Basic::Never),

            Type::Generic(generic) => {
                Type::Generic(Generic::Keyof(Box::new(Type::Generic(generic.clone()))))
            }
        }
    }

    /// Resolves the typeof operator.
    fn type_query(&self, typ: &ast::TsTypeQuery) -> Type {
        if typ.type_args.is_some() {
            HANDLER.with(|handler| {
                handler.span_err(typ.span, "typeof with type args not yet supported")
            });
            return Type::Basic(Basic::Never);
        }

        match &typ.expr_name {
            ast::TsTypeQueryExpr::TsEntityName(ast::TsEntityName::Ident(ident)) => {
                let obj = self.ident_obj(ident);
                if let Some(obj) = obj {
                    self.resolve_obj_type(&obj)
                } else {
                    HANDLER.with(|handler| handler.span_err(ident.span, "unknown identifier"));
                    Type::Basic(Basic::Never)
                }
            }
            _ => {
                HANDLER.with(|handler| {
                    handler.span_err(typ.span, "typeof with non-ident not yet supported")
                });
                Type::Basic(Basic::Never)
            }
        }
    }

    fn type_lit(&self, type_lit: &ast::TsTypeLit) -> Type {
        let mut fields: Vec<InterfaceField> = Vec::with_capacity(type_lit.members.len());
        let mut index = None;
        for m in &type_lit.members {
            match m {
                ast::TsTypeElement::TsPropertySignature(p) => {
                    let name = match *p.key {
                        ast::Expr::Ident(ref i) => FieldName::String(i.sym.as_ref().to_string()),
                        _ => {
                            HANDLER.with(|handler| {
                                handler.span_err(p.key.span(), "unsupported property key")
                            });
                            continue;
                        }
                    };

                    if let Some(type_params) = &p.type_params {
                        HANDLER.with(|handler| {
                            handler.span_err(type_params.span(), "unsupported type parameters")
                        });
                        continue;
                    }
                    if p.type_ann.is_none() {
                        HANDLER.with(|handler| {
                            handler.span_err(p.span(), "unsupported missing type annotation")
                        });
                        continue;
                    }

                    let typ = self.typ(p.type_ann.as_ref().unwrap().type_ann.as_ref());
                    fields.push(InterfaceField {
                        range: m.span().into(),
                        name,
                        typ,
                        optional: p.optional,
                    });
                }

                ast::TsTypeElement::TsIndexSignature(idx) => {
                    // [foo: K]: V;
                    let Some(ast::TsFnParam::Ident(ident)) = idx.params.get(0) else {
                        HANDLER.with(|handler| {
                            handler.span_err(idx.span(), "missing index signature key")
                        });
                        continue;
                    };
                    let Some(key_type_ann) = &ident.type_ann else {
                        HANDLER.with(|handler| {
                            handler.span_err(ident.span(), "missing key type annotation")
                        });
                        continue;
                    };

                    let Some(value_type_ann) = &idx.type_ann else {
                        HANDLER.with(|handler| {
                            handler.span_err(idx.span(), "missing value type annotation")
                        });
                        continue;
                    };

                    let key = self.typ(&key_type_ann.type_ann);
                    let value = self.typ(&value_type_ann.type_ann);
                    index = Some((Box::new(key), Box::new(value)))
                }

                ast::TsTypeElement::TsMethodSignature(_)
                | ast::TsTypeElement::TsCallSignatureDecl(_)
                | ast::TsTypeElement::TsConstructSignatureDecl(_)
                | ast::TsTypeElement::TsGetterSignature(_)
                | ast::TsTypeElement::TsSetterSignature(_) => {
                    HANDLER.with(|handler| {
                        handler.span_err(m.span(), &format!("unsupported: {:#?}", type_lit))
                    });
                    continue;
                }
            }
        }

        Type::Interface(Interface {
            fields,

            // TODO should these be set?
            index,
            call: None,
        })
    }

    /// Resolves literals.
    fn lit_type(&self, lit_type: &ast::TsLitType) -> Type {
        Type::Literal(match &lit_type.lit {
            ast::TsLit::Str(val) => Literal::String(val.value.to_string()),
            ast::TsLit::Number(val) => Literal::Number(val.value),
            ast::TsLit::Bool(val) => Literal::Boolean(val.value),
            ast::TsLit::BigInt(val) => Literal::BigInt(val.value.to_string()),
            ast::TsLit::Tpl(_) => {
                // A template literal.
                // https://www.typescriptlang.org/docs/handbook/2/template-literal-types.html
                HANDLER.with(|handler| {
                    handler.span_err(
                        lit_type.span,
                        "template literal expression not yet supported",
                    )
                });
                Literal::String("".into())
            }
        })
    }

    fn type_ref(&self, typ: &ast::TsTypeRef) -> Type {
        let ident: &ast::Ident = match &typ.type_name {
            ast::TsEntityName::Ident(i) => i,
            ast::TsEntityName::TsQualifiedName(qn) => {
                HANDLER
                    .with(|handler| handler.span_err(typ.span, "qualified name not yet supported"));
                return Type::Basic(Basic::Never);
            }
        };

        // Is this a reference to a type parameter?
        let type_param = self
            .type_params
            .iter()
            .enumerate()
            .find(|tp| tp.1.name.to_id() == ident.to_id())
            .map(|tp| (tp.0, *tp.1));
        if let Some((idx, type_param)) = type_param {
            return if let Some(type_arg) = self.type_args.get(idx) {
                type_arg.clone()
            } else {
                let constraint = type_param.constraint.as_ref().map(|c| self.btyp(c));
                Type::Generic(Generic::TypeParam(TypeParam { idx, constraint }))
            };
        }

        // Otherwise, is this a reference to the current mapped 'key' type?
        if let Some(mapped_type_ctx) = &self.mapped_key_id {
            if ident.to_id() == *mapped_type_ctx {
                // Do we have a mapped key type?
                return if let Some(mapped_key_type) = self.mapped_key_type {
                    mapped_key_type.clone()
                } else {
                    Type::Generic(Generic::MappedKeyType)
                };
            }
        }

        let Some(obj) = self.ident_obj(ident) else {
            HANDLER.with(|handler| handler.span_err(ident.span, "unknown identifier"));
            return Type::Basic(Basic::Never);
        };

        let mut type_arguments =
            Vec::with_capacity(typ.type_params.as_ref().map_or(0, |p| p.params.len()));
        if let Some(params) = &typ.type_params {
            for p in &params.params {
                type_arguments.push(self.typ(p));
            }
        }

        match &obj.kind {
            ObjectKind::TypeName(_) => {
                let named = Named::new(obj, type_arguments);

                // Don't reference named types in the universe,
                // otherwise we try to find them on disk.
                // if self.state.is_universe(named.obj.module_id) {
                // named.underlying(self.state).clone()
                // } else {
                Type::Named(named)
                // }
            }
            ObjectKind::Enum(_) | ObjectKind::Class(_) => {
                Type::Named(Named::new(obj, type_arguments))
            }
            ObjectKind::Var(_) | ObjectKind::Using(_) | ObjectKind::Func(_) => {
                HANDLER.with(|handler| handler.span_err(ident.span, "value used as type"));
                Type::Basic(Basic::Never)
            }
            ObjectKind::Module(_) => {
                HANDLER.with(|handler| handler.span_err(ident.span, "module used as type"));
                Type::Basic(Basic::Never)
            }
            ObjectKind::Namespace(_) => {
                HANDLER.with(|handler| handler.span_err(ident.span, "namespace used as type"));
                Type::Basic(Basic::Never)
            }
        }
    }

    fn array(&self, tt: &ast::TsArrayType) -> Type {
        Type::Array(Box::new(self.typ(&tt.elem_type)))
    }

    fn optional(&self, tt: &ast::TsOptionalType) -> Type {
        Type::Optional(Box::new(self.typ(&tt.type_ann)))
    }

    fn tuple(&self, tuple: &ast::TsTupleType) -> Type {
        let types = self.types(tuple.elem_types.iter().filter_map(|t|
            // As far as I can tell labels don't actually impact type-checking
            // at all, so we can ignore them.
            // See https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-0.html.
            if t.label.is_some() {
                None
            } else {
                Some(t.ty.as_ref())
            }));

        Type::Tuple(types)
    }

    fn union(&self, union_type: &ast::TsUnionType) -> Type {
        // TODO handle unifying e.g. "string | 'foo'" into "string"
        let types = self.types(union_type.types.iter().map(|t| t.as_ref()));
        Type::Union(types)
    }

    // https://www.typescriptlang.org/docs/handbook/2/conditional-types.html
    fn conditional(&self, tt: &ast::TsConditionalType) -> Type {
        let check = self.typ(&tt.check_type);
        let extends = self.typ(&tt.extends_type);

        // Do we have a union type in `check`, and the AST is a naked type parameter?
        // If so, we need to treat it as a distributive conditional type.
        // See: https://www.typescriptlang.org/docs/handbook/advanced-types.html#distributive-conditional-types
        if let Type::Union(types) = &check {
            if let ast::TsType::TsTypeRef(ref check) = tt.check_type.as_ref() {
                if check.type_params.is_none() {
                    if let Some(ident) = check.type_name.as_ident() {
                        if self
                            .type_params
                            .iter()
                            .find(|tp| tp.name.to_id() == ident.to_id())
                            .is_some()
                        {
                            // Apply the conditional to each type in the union.
                            let result = types
                                .iter()
                                .map(|t| match t.assignable(self.state, &extends) {
                                    Some(true) => self.typ(&tt.true_type),
                                    Some(false) => self.typ(&tt.false_type),
                                    None => Type::Generic(Generic::Conditional(Conditional {
                                        check_type: Box::new(t.clone()),
                                        extends_type: Box::new(extends.clone()),
                                        true_type: self.btyp(&tt.true_type),
                                        false_type: self.btyp(&tt.false_type),
                                    })),
                                })
                                .collect::<Vec<_>>();
                            return simplify_union(result);
                        }
                    }
                }
            }
        }

        match check.assignable(self.state, &extends) {
            Some(true) => self.typ(&tt.true_type),
            Some(false) => self.typ(&tt.false_type),
            None => Type::Generic(Generic::Conditional(Conditional {
                check_type: Box::new(check),
                extends_type: Box::new(extends),
                true_type: self.btyp(&tt.true_type),
                false_type: self.btyp(&tt.false_type),
            })),
        }
    }

    fn intersection(&self, typ: &ast::TsIntersectionType) -> Type {
        let mut result = Owned(self.typ(&typ.types[0]));
        for t in &typ.types[1..] {
            let t = self.typ(t);
            result = intersect(self, result, Owned(t));
        }
        result.into_owned()
    }

    fn keyword(&self, typ: &ast::TsKeywordType) -> Type {
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
                HANDLER.with(|handler| {
                    handler.span_err(typ.span, "unimplemented: TsIntrinsicKeyword")
                });
                Basic::Never
            }
        };

        Type::Basic(basic)
    }

    fn interface_decl(&self, decl: &ast::TsInterfaceDecl) -> Type {
        if decl.extends.len() > 0 {
            HANDLER.with(|handler| handler.span_err(decl.span, "extends not yet supported"));
            return Type::Basic(Basic::Never);
        } else if decl.type_params.is_some() {
            HANDLER.with(|handler| handler.span_err(decl.span, "type params not yet supported"));
            return Type::Basic(Basic::Never);
        }

        self.typ(&ast::TsType::TsTypeLit(ast::TsTypeLit {
            span: decl.span,
            members: decl.body.body.clone(),
        }))
    }

    fn expr(&self, expr: &ast::Expr) -> Type {
        match expr {
            ast::Expr::This(_) => Type::This,
            ast::Expr::Array(lit) => self.array_lit(lit),
            ast::Expr::Object(lit) => self.object_lit(lit),
            ast::Expr::Fn(_) => {
                HANDLER.with(|handler| handler.span_err(expr.span(), "fn expr not yet supported"));
                Type::Basic(Basic::Never)
            }
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
                | ast::UnaryOp::Delete => self.expr(&expr.arg),
            },
            ast::Expr::Update(expr) => self.expr(&expr.arg),
            ast::Expr::Bin(expr) => {
                let left = self.expr(&expr.left);
                let right = self.expr(&expr.right);

                match left.union_merge(&right) {
                    Some(unified) => unified,
                    // TODO handle this correctly.
                    None => left,
                }
            }
            ast::Expr::Assign(expr) => self.expr(&expr.right),
            ast::Expr::Member(expr) => self.member_expr(expr),
            ast::Expr::SuperProp(_) => {
                HANDLER
                    .with(|handler| handler.span_err(expr.span(), "super prop not yet supported"));
                Type::Basic(Basic::Never)
            }
            ast::Expr::Cond(cond) => {
                let left = self.expr(&cond.cons);
                let right = self.expr(&cond.alt);
                left.simplify_or_union(right)
            }
            ast::Expr::Call(expr) => {
                HANDLER.with(|handler| handler.span_err(expr.span, "call expr not yet supported"));
                Type::Basic(Basic::Never)
            }
            ast::Expr::New(expr) => {
                // The type of a class instance is the same as the class itself.
                // TODO type args
                self.expr(&expr.callee)
            }
            ast::Expr::Seq(expr) => match expr.exprs.last() {
                Some(expr) => self.expr(expr),
                None => Type::Basic(Basic::Never),
            },
            ast::Expr::Ident(expr) => {
                let Some(obj) = self.ident_obj(expr) else {
                    HANDLER.with(|handler| handler.span_err(expr.span, "unknown identifier"));
                    return Type::Basic(Basic::Never);
                };

                let named = Named::new(obj, vec![]);
                Type::Named(named)
            }
            ast::Expr::PrivateName(expr) => {
                let Some(obj) = self.ident_obj(&expr.id) else {
                    HANDLER.with(|handler| handler.span_err(expr.id.span, "unknown identifier"));
                    return Type::Basic(Basic::Never);
                };

                Type::Named(Named::new(obj, vec![]))
            }
            ast::Expr::Lit(expr) => match &expr {
                ast::Lit::Str(val) => Type::Literal(Literal::String(val.value.to_string())),
                ast::Lit::Bool(val) => Type::Literal(Literal::Boolean(val.value)),
                ast::Lit::Num(val) => Type::Literal(Literal::Number(val.value)),
                ast::Lit::Null(_) => Type::Basic(Basic::Null),
                ast::Lit::BigInt(_) => Type::Basic(Basic::BigInt),
                ast::Lit::Regex(_) => {
                    HANDLER
                        .with(|handler| handler.span_err(expr.span(), "regex not yet supported"));
                    Type::Basic(Basic::Never)
                }
                ast::Lit::JSXText(_) => {
                    HANDLER.with(|handler| {
                        handler.span_err(expr.span(), "jsx text not yet supported")
                    });
                    Type::Basic(Basic::Never)
                }
            },
            ast::Expr::Tpl(_) => Type::Basic(Basic::String),
            ast::Expr::TaggedTpl(_) => {
                HANDLER.with(|handler| {
                    handler.span_err(expr.span(), "tagged template not yet supported")
                });
                Type::Basic(Basic::Never)
            }
            ast::Expr::Arrow(_) => {
                HANDLER
                    .with(|handler| handler.span_err(expr.span(), "arrow expr not yet supported"));
                Type::Basic(Basic::Never)
            }
            ast::Expr::Class(_) => {
                HANDLER
                    .with(|handler| handler.span_err(expr.span(), "class expr not yet supported"));
                Type::Basic(Basic::Never)
            }
            ast::Expr::Yield(expr) => match &expr.arg {
                Some(arg) => self.expr(arg),
                None => Type::Basic(Basic::Undefined),
            },
            ast::Expr::MetaProp(expr) => match expr.kind {
                // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/new.target
                ast::MetaPropKind::NewTarget => {
                    HANDLER.with(|handler| {
                        handler.span_err(expr.span, "new.target not yet supported")
                    });
                    Type::Basic(Basic::Never)
                }
                // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/import.meta
                ast::MetaPropKind::ImportMeta => Type::Basic(Basic::Object),
            },
            ast::Expr::Await(expr) => {
                let prom = self.expr(&expr.arg);
                if let Type::Named(mut named) = prom {
                    if named.obj.name.as_deref() == Some("Promise")
                        && self.state.is_universe(named.obj.module_id)
                    {
                        if named.type_arguments.len() > 0 {
                            return named.type_arguments.swap_remove(0);
                        }
                    }
                }
                Type::Basic(Basic::Unknown)
            }

            ast::Expr::Paren(expr) => self.expr(&expr.expr),

            ast::Expr::JSXMember(_)
            | ast::Expr::JSXNamespacedName(_)
            | ast::Expr::JSXEmpty(_)
            | ast::Expr::JSXElement(_)
            | ast::Expr::JSXFragment(_) => Type::Basic(Basic::Never),

            // <T>foo
            ast::Expr::TsTypeAssertion(expr) => self.typ(&expr.type_ann),
            // foo as T
            ast::Expr::TsAs(expr) => self.typ(&expr.type_ann),

            ast::Expr::TsConstAssertion(expr) => self.expr(&expr.expr),

            // https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-9.html
            ast::Expr::TsSatisfies(expr) => self.expr(&expr.expr),

            ast::Expr::TsInstantiation(expr) => {
                // TODO handle type args
                self.expr(&expr.expr)
            }

            // The "foo!" operator
            ast::Expr::TsNonNull(expr) => {
                let base = self.expr(&expr.expr);
                match base {
                    Type::Optional(typ) => *typ,
                    Type::Union(types) => {
                        let non_null = types
                            .into_iter()
                            .filter(|t| {
                                !matches!(
                                    t,
                                    Type::Basic(Basic::Undefined) | Type::Basic(Basic::Null)
                                )
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

            // "foo?.bar"
            ast::Expr::OptChain(expr) => {
                HANDLER.with(|handler| {
                    handler.span_err(expr.span, "optional chaining not yet supported")
                });
                Type::Basic(Basic::Never)
            }

            ast::Expr::Invalid(_) => Type::Basic(Basic::Never),
        }
    }

    fn array_lit(&self, lit: &ast::ArrayLit) -> Type {
        let elem_types = Vec::with_capacity(lit.elems.len());

        // Track the current element type.
        let mut elem_type: Option<Type> = None;

        for elem in &lit.elems {
            if let Some(elem) = elem {
                let mut base = self.expr(&elem.expr);
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

        Type::Union(elem_types)
    }

    fn object_lit(&self, lit: &ast::ObjectLit) -> Type {
        let mut fields = Vec::with_capacity(lit.props.len());

        for prop in &lit.props {
            match prop {
                ast::PropOrSpread::Prop(prop) => {
                    let (name, typ) = match prop.as_ref() {
                        ast::Prop::Shorthand(id) => {
                            let Some(obj) = self.ident_obj(&id) else {
                                HANDLER.with(|handler| {
                                    handler.span_err(id.span, "unknown identifier")
                                });
                                return Type::Basic(Basic::Never);
                            };

                            let obj_type = self.obj_type(&obj);
                            (Cow::Borrowed(id.sym.as_ref()), obj_type)
                        }
                        ast::Prop::KeyValue(kv) => {
                            let key = self.prop_name_to_string(&kv.key);
                            let val_typ = self.expr(&kv.value);
                            (key, val_typ)
                        }
                        ast::Prop::Assign(prop) => {
                            HANDLER.with(|handler| {
                                handler
                                    .span_err(prop.span(), "unsupported assign in object literal")
                            });
                            return Type::Basic(Basic::Never);
                        }
                        ast::Prop::Getter(prop) => {
                            let key = self.prop_name_to_string(&prop.key);
                            // We can't figure out the value type here as it relies on
                            // doing type analysis on the function body.
                            (key, Type::Basic(Basic::Unknown))
                        }
                        ast::Prop::Setter(prop) => {
                            let key = self.prop_name_to_string(&prop.key);
                            // We can't figure out the value type here as it relies on
                            // doing type analysis on the function body.
                            (key, Type::Basic(Basic::Unknown))
                        }
                        ast::Prop::Method(prop) => {
                            let key = self.prop_name_to_string(&prop.key);
                            // We can't figure out the value type here as it relies on
                            // doing type analysis on the function body.
                            (key, Type::Basic(Basic::Unknown))
                        }
                    };
                    fields.push(InterfaceField {
                        range: prop.span().into(),
                        name: FieldName::String(name.into_owned()),
                        typ,
                        optional: false,
                    });
                }
                ast::PropOrSpread::Spread(spread) => {
                    let typ = self.expr(&spread.expr);
                    match typ {
                        Type::Interface(interface) => {
                            fields.extend(interface.fields);
                        }
                        _ => {
                            HANDLER.with(|handler| {
                                handler.span_err(spread.span(), "unsupported spread")
                            });
                        }
                    }
                }
            }
        }

        Type::Interface(Interface {
            fields,

            // TODO should these be set?
            index: None,
            call: None,
        })
    }

    fn member_expr(&self, expr: &ast::MemberExpr) -> Type {
        let obj_type = self.expr(&expr.obj);
        self.resolve_member_prop(&obj_type, &expr.prop)
    }

    fn resolve_member_prop(&self, obj_type: &Type, prop: &ast::MemberProp) -> Type {
        match obj_type {
            Type::Basic(_)
            | Type::Literal(_)
            | Type::Array(_)
            | Type::Tuple(_)
            | Type::Union(_)
            | Type::Optional(_)
            | Type::This
            | Type::Generic(_)
            | Type::Class(_) => {
                HANDLER.with(|handler| handler.span_err(prop.span(), "unsupported member on type"));
                Type::Basic(Basic::Never)
            }
            Type::Enum(tt) => {
                for m in tt.members.iter() {
                    let name = m.name.as_str();
                    let matches = match prop {
                        ast::MemberProp::Ident(i) => name == i.sym.as_ref(),
                        ast::MemberProp::PrivateName(i) => name == i.id.sym.as_ref(),
                        ast::MemberProp::Computed(i) => match self.expr(&i.expr) {
                            Type::Literal(lit) => match lit {
                                Literal::String(str) => name == str,
                                Literal::Number(num) => name == num.to_string().as_str(),
                                _ => false,
                            },
                            _ => false,
                        },
                    };
                    if matches {
                        return m.value.clone().to_type();
                    }
                }
                Type::Basic(Basic::Never)
            }
            Type::Interface(tt) => {
                for field in tt.fields.iter() {
                    let matches = match prop {
                        ast::MemberProp::Ident(i) => field.name.eq_str(i.sym.as_ref()),
                        ast::MemberProp::PrivateName(i) => field.name.eq_str(i.id.sym.as_ref()),
                        ast::MemberProp::Computed(i) => match self.expr(&i.expr) {
                            Type::Literal(lit) => match lit {
                                Literal::String(str) => field.name.eq_str(&str),
                                Literal::Number(num) => field.name.eq_str(num.to_string().as_str()),
                                _ => false,
                            },
                            _ => false,
                        },
                    };
                    if matches {
                        return field.typ.clone();
                    }
                }

                // Otherwise use the index signature's value type, if present.
                if let Some(idx) = &tt.index {
                    *idx.1.clone()
                } else {
                    Type::Basic(Basic::Never)
                }
            }
            Type::Named(_) => {
                let underlying = self.underlying(obj_type);
                self.resolve_member_prop(&underlying, prop)
            }
        }
    }

    /// Resolves a prop name to the underlying string literal.
    fn prop_name_to_string<'b>(&self, prop: &'b ast::PropName) -> Cow<'b, str> {
        match prop {
            ast::PropName::Ident(id) => Cow::Borrowed(id.sym.as_ref()),
            ast::PropName::Str(str) => Cow::Borrowed(str.value.as_ref()),
            ast::PropName::Num(num) => Cow::Owned(num.value.to_string()),
            ast::PropName::BigInt(bigint) => Cow::Owned(bigint.value.to_string()),
            ast::PropName::Computed(expr) => {
                if let Type::Literal(lit) = self.expr(&expr.expr) {
                    match lit {
                        Literal::String(str) => return Cow::Owned(str),
                        Literal::Number(num) => return Cow::Owned(num.to_string()),
                        _ => {}
                    }
                }

                HANDLER.with(|handler| handler.span_err(expr.span, "unsupported computed prop"));
                Cow::Borrowed("")
            }
        }
    }
}

impl<'a> Ctx<'a> {
    pub fn obj_type(&self, obj: &Object) -> Type {
        if matches!(&obj.kind, ObjectKind::Module(_)) {
            // Modules don't have a type.
            return Type::Basic(Basic::Never);
        };

        match obj.state.borrow().deref() {
            CheckState::Completed(typ) => return typ.clone(),
            CheckState::InProgress => {
                // TODO support certain types of circular references.
                HANDLER.with(|handler| {
                    handler.span_err(obj.range.to_span(), "circular type reference");
                });
                return Type::Basic(Basic::Never);
            }
            CheckState::NotStarted => {
                // Fall through below to do actual type-checking.
                // Needs to be handled separately to avoid borrowing issues.
            }
        }
        // Post-condition: state is NotStarted.

        // Mark this object as being checked.
        *obj.state.borrow_mut() = CheckState::InProgress;

        let type_params: Vec<_> = obj.kind.type_params().collect();

        let typ = {
            // Create a nested ctx that uses the object's module.
            let ctx = Ctx::new(self.state, self.module).with_type_params(&type_params);
            ctx.resolve_obj_type(obj)
        };

        *obj.state.borrow_mut() = CheckState::Completed(typ.clone());
        typ
    }

    fn resolve_obj_type(&self, obj: &Object) -> Type {
        match &obj.kind {
            ObjectKind::TypeName(tn) => match &tn.decl {
                TypeNameDecl::Interface(iface) => {
                    // TODO handle type params here
                    self.interface_decl(iface)
                }
                TypeNameDecl::TypeAlias(ta) => {
                    // TODO handle type params here
                    self.typ(&*ta.type_ann)
                }
            },

            ObjectKind::Enum(o) => {
                let mut members = Vec::with_capacity(o.members.len());
                let mut prev_value = None;
                for m in &o.members {
                    // Determine the initializer type, if provided.
                    let init = m.init.as_ref().map(|t| self.expr(&t));
                    let value = match init {
                        None => {
                            // We didn't have an initializer.
                            // Determine the value based on the previous value.
                            match prev_value {
                                // No previous value; this is the first entry.
                                None => EnumValue::Number(0),
                                Some(EnumValue::Number(i)) => EnumValue::Number(i + 1),
                                Some(EnumValue::String(_)) => {
                                    HANDLER.with(|h| {
                                        h.span_err(
                                            m.span(),
                                            "implicit enum value cannot follow string value",
                                        )
                                    });
                                    EnumValue::Number(0)
                                }
                            }
                        }
                        Some(Type::Literal(lit)) => match lit {
                            Literal::String(str) => EnumValue::String(str),
                            Literal::Number(num) => {
                                // Ensure the number is an integer.
                                if num.fract() != 0.0 {
                                    HANDLER.with(|h| {
                                        h.span_err(m.span(), "enum value must be an integer")
                                    });
                                }
                                EnumValue::Number(num as i64)
                            }
                            _ => {
                                HANDLER.with(|h| h.span_err(m.span(), "unsupported enum value"));
                                EnumValue::Number(0)
                            }
                        },
                        _ => {
                            HANDLER.with(|h| h.span_err(m.span(), "unsupported enum value"));
                            EnumValue::Number(0)
                        }
                    };

                    let name = match &m.id {
                        ast::TsEnumMemberId::Ident(id) => id.sym.as_ref().to_string(),
                        ast::TsEnumMemberId::Str(str) => str.value.as_ref().to_string(),
                    };
                    prev_value = Some(value.clone());
                    members.push(EnumMember { name, value });
                }
                Type::Enum(EnumType { members })
            }

            ObjectKind::Var(o) => {
                // Do we have a type annotation? If so, use that.
                if let Some(type_ann) = &o.type_ann {
                    self.typ(&*type_ann.type_ann)
                } else if let Some(expr) = &o.expr {
                    self.expr(&*expr)
                } else {
                    Type::Basic(Basic::Never)
                }
            }

            ObjectKind::Using(o) => {
                // Do we have a type annotation? If so, use that.
                if let Some(type_ann) = &o.type_ann {
                    self.typ(&type_ann.type_ann)
                } else if let Some(expr) = &o.expr {
                    self.expr(&*expr)
                } else {
                    Type::Basic(Basic::Never)
                }
            }

            ObjectKind::Func(_o) => {
                HANDLER.with(|handler| {
                    handler.span_err(obj.range.to_span(), "function types not yet supported");
                });
                Type::Basic(Basic::Never)
            }

            ObjectKind::Class(_o) => Type::Class(ClassType {}),

            ObjectKind::Module(_o) => Type::Basic(Basic::Never),
            ObjectKind::Namespace(_o) => {
                // TODO include namespace objects in interface
                Type::Basic(Basic::Object)
            }
        }
    }

    fn resolve_obj(&self, expr: &ast::Expr) -> Option<Rc<Object>> {
        match self.expr(expr) {
            Type::Named(named) => Some(named.obj.clone()),
            _ => None,
        }
    }

    fn ident_obj(&self, ident: &ast::Ident) -> Option<Rc<Object>> {
        // Does this represent a type parameter?
        self.state.resolve_module_ident(self.module, ident)
    }
}

impl<'a> Ctx<'a> {
    #[tracing::instrument(skip(self), ret, level = "trace")]
    pub fn concrete<'b>(&'b self, typ: &'b Type) -> Resolved<'b, Type> {
        match typ {
            // Basic types that never change.
            Type::Basic(_) | Type::Literal(_) | Type::Enum(_) | Type::This => Same(typ),

            // Nested types that recurse.
            Type::Array(elem) => match self.concrete(elem) {
                New(t) => New(Type::Array(Box::new(t))),
                Changed(t) => New(Type::Array(Box::new(t.clone()))),
                Same(_) => Same(typ),
            },
            Type::Tuple(types) => match self.concrete_list(types) {
                New(t) => New(Type::Tuple(t)),
                Changed(t) => New(Type::Tuple(t.to_owned())),
                Same(_) => Same(typ),
            },
            Type::Union(types) => match self.concrete_list(types) {
                New(t) => New(Type::Union(t)),
                Changed(t) => New(Type::Union(t.to_owned())),
                Same(_) => Same(typ),
            },
            Type::Optional(typ) => match self.concrete(typ) {
                New(t) => New(Type::Optional(Box::new(t))),
                Changed(t) => New(Type::Optional(Box::new(t.to_owned()))),
                Same(_) => Same(typ),
            },

            Type::Named(named) => match self.concrete_list(&named.type_arguments) {
                New(t) => New(Type::Named(Named::new(named.obj.clone(), t))),
                Changed(t) => New(Type::Named(Named::new(named.obj.clone(), t.to_owned()))),
                Same(_) => Same(typ),
            },

            Type::Interface(iface) => {
                let concrete_fields =
                    |fields: &'b [InterfaceField]| -> Resolved<'b, [InterfaceField]> {
                        for (i, field) in fields.iter().enumerate() {
                            let t = match self.concrete(&field.typ) {
                                New(t) => t,
                                Changed(t) => t.clone(),
                                Same(_) => continue,
                            };
                            // We have a new type, so we need to clone the entire list.
                            let mut res = Vec::with_capacity(fields.len());
                            res.extend(fields[0..i].iter().cloned());

                            res.push(InterfaceField {
                                range: field.range,
                                typ: t,
                                name: field.name.clone(),
                                optional: field.optional,
                            });

                            // Copy all remaining elements.
                            res.extend(fields[i + 1..].iter().map(|t| InterfaceField {
                                range: t.range,
                                name: t.name.clone(),
                                typ: self.concrete(&t.typ).into_owned(),
                                optional: t.optional,
                            }));

                            return New(res);
                        }

                        // All types are the same, so we can just return the original list.
                        Same(fields)
                    };

                let fields = concrete_fields(&iface.fields);
                let index = match &iface.index {
                    Some((key, val)) => Some((self.concrete(key), self.concrete(val))),
                    None => None,
                };
                let call = match &iface.call {
                    Some((params, ret)) => {
                        Some((self.concrete_list(params), self.concrete_list(ret)))
                    }
                    None => None,
                };

                // If we have any parts that aren't Same, we need to make the whole thing New.
                // Otherwise return the original type.
                if !matches!(fields, Same(_))
                    || !matches!(index, Some((Same(_), _) | (_, Same(_))))
                    || !matches!(call, Some((Same(_), _) | (_, Same(_))))
                {
                    New(Type::Interface(Interface {
                        fields: fields.into_owned(),
                        index: index
                            .map(|(k, v)| (Box::new(k.into_owned()), Box::new(v.into_owned()))),
                        call: call.map(|(p, r)| (p.into_owned(), r.into_owned())),
                    }))
                } else {
                    Same(typ)
                }
            }

            // TODO is this correct?
            // Class types are already concrete.
            Type::Class(_) => Same(typ),

            Type::Generic(generic) => match generic {
                Generic::TypeParam(param) => {
                    // If the type parameter is a concrete type, return that.
                    if let Some(arg) = self.type_args.get(param.idx) {
                        Changed(arg)
                    } else {
                        // We don't have a concrete type, so return the original type.
                        Same(typ)
                    }
                }

                Generic::Keyof(source) => {
                    let concrete_source = self.concrete(source);
                    let keys = self.keyof(&concrete_source);
                    New(keys)
                }

                Generic::Intersection(intersection) => {
                    let x = self.concrete(&intersection.x);
                    let y = self.concrete(&intersection.y);

                    match (x, y) {
                        (Same(_), Same(_)) => Same(typ),
                        (x, y) => match intersect(self, x.into(), y.into()) {
                            Owned(t) => New(t),
                            Borrowed(t) => Changed(t),
                        },
                    }
                }

                Generic::Conditional(cond) => {
                    let check = self.concrete(&cond.check_type);
                    let extends = self.concrete(&cond.extends_type);

                    // Is this a "distributed conditional type"?
                    // See https://www.typescriptlang.org/docs/handbook/advanced-types.html#distributive-conditional-types

                    match (cond.check_type.as_ref(), check.into_owned()) {
                        (Type::Generic(Generic::TypeParam(param)), Type::Union(check)) => {
                            // If check is a union, apply the check to each type in the union.
                            let mut type_args = self.type_args.to_owned();

                            // Construct a modified context that modifies the given type argument
                            // to refer only to the concrete type for this union.
                            let result: Vec<_> = check
                                .into_iter()
                                .filter_map(|c| {
                                    // Modify the type args.
                                    if type_args.len() > param.idx {
                                        type_args[param.idx] = c.clone();
                                    }
                                    let nested = self.clone().with_type_args(&type_args);
                                    match c.assignable(self.state, &extends) {
                                        Some(true) => {
                                            Some(nested.concrete(&cond.true_type).into_owned())
                                        }
                                        Some(false) => {
                                            Some(nested.concrete(&cond.false_type).into_owned())
                                        }
                                        // This implies there's a generic type in this mix,
                                        // which shouldn't happen when concretizing.
                                        None => None,
                                    }
                                })
                                .collect();

                            New(simplify_union(result))
                        }

                        // Otherwise just check the single element.
                        (_, check) => match check.assignable(self.state, &extends) {
                            Some(true) => self.concrete(&cond.true_type).same_to_changed(),
                            Some(false) => self.concrete(&cond.false_type).same_to_changed(),

                            // We don't yet have enough type information to resolve the conditional.
                            // Still, return a new type with the concretized types we have.
                            None => New(Type::Generic(Generic::Conditional(Conditional {
                                check_type: Box::new(check),
                                extends_type: Box::new(extends.into_owned()),
                                true_type: Box::new(self.concrete(&cond.true_type).into_owned()),
                                false_type: Box::new(self.concrete(&cond.false_type).into_owned()),
                            }))),
                        },
                    }
                }

                Generic::Index((source, index)) => {
                    let source = self.concrete(source);
                    let index = self.concrete(index);
                    let result = self.type_index(Span::default(), &source, &index);
                    New(result)
                }

                Generic::Mapped(mapped) => {
                    let mut iface = Interface {
                        fields: vec![],
                        index: None,
                        call: None,
                    };

                    let keys = self.underlying(&mapped.in_type).into_owned();
                    for key in keys.into_iter_unions() {
                        let value = self
                            .clone()
                            .with_mapped_key_type(Some(&key))
                            .concrete(&mapped.value_type)
                            .into_owned();

                        // If the value resolves to 'never' it should be skipped.
                        if let Type::Basic(Basic::Never) = &value {
                            continue;
                        }

                        // Get the underlying key type if it's named.

                        match key {
                            // Never means the field should be excluded.
                            Type::Basic(Basic::Never) => {
                                HANDLER.with(|handler| {
                                    handler.err("unexpected 'never' type as mapped type key");
                                });
                            }

                            // Do we have a wildcard type like "string" or "number"?
                            // If so treat it as an index signature.
                            source @ (Type::Basic(Basic::String)
                            | Type::Basic(Basic::Number)
                            | Type::Basic(Basic::Symbol)) => {
                                // TODO actually do the mapping/filtering
                                iface.index = Some((Box::new(source.clone()), Box::new(value)));
                            }

                            // Do we have a string literal?
                            Type::Literal(Literal::String(str)) => {
                                // Unwrap optional and record it on the field instead.
                                let (typ, optional) = match value {
                                    // Never means the field should be excluded.
                                    Type::Basic(Basic::Never) => continue,
                                    Type::Optional(typ) => (*typ, true),
                                    typ => (typ, false),
                                };

                                iface.fields.push(InterfaceField {
                                    range: Range::default(),
                                    name: FieldName::String(str.clone()),
                                    typ,
                                    optional,
                                });
                            }

                            typ => {
                                HANDLER.with(|handler| {
                                    handler
                                        .err(&format!("unsupported mapped key type: {:#?}", typ));
                                });
                            }
                        }
                    }

                    // If the mapped type contains optional modifiers, apply them.
                    if let Some(optional) = &mapped.optional {
                        if let Some((key, value)) = iface.index.take() {
                            let value = if *optional {
                                match value.as_ref() {
                                    Type::Optional(_) => value,
                                    _ => Box::new(Type::Optional(value)),
                                }
                            } else {
                                if let Type::Optional(inner) = *value {
                                    inner
                                } else {
                                    value
                                }
                            };
                            iface.index = Some((key, value));
                        }

                        for field in iface.fields.iter_mut() {
                            field.optional = *optional;
                        }
                    }

                    New(Type::Interface(iface))
                }

                Generic::MappedKeyType => match self.mapped_key_type {
                    Some(key) => Changed(key),
                    None => Same(typ),
                },
            },
        }
    }

    pub fn underlying_named(&self, named: &Named) -> Type {
        let type_params = named.obj.kind.type_params().collect::<Vec<_>>();
        let type_args = self.concrete_list(&named.type_arguments);
        let typ = self.obj_type(&named.obj);

        let ctx = self
            .clone()
            .with_type_params(&type_params)
            .with_type_args(&type_args);

        let span = tracing::trace_span!("underlying_named", ?named, ?type_args);
        let _guard = span.enter();
        ctx.underlying(&typ).into_owned()
    }

    pub fn underlying<'b>(&'b self, typ: &'b Type) -> Resolved<'b, Type> {
        // Ensure we resolve the concrete type.
        match self.concrete(typ) {
            Same(tt) => match tt {
                Type::Named(named) => New(named.underlying(self.state)),
                _ => Same(typ),
            },
            Changed(tt) => match tt {
                Type::Named(named) => New(named.underlying(self.state)),
                _ => Changed(tt),
            },
            New(tt) => match tt {
                Type::Named(named) => New(named.underlying(self.state)),
                _ => New(tt),
            },
        }
    }

    fn concrete_list<'b>(&'b self, v: &'b [Type]) -> Resolved<'b, [Type]> {
        for (i, typ) in v.iter().enumerate() {
            let t = match self.concrete(typ) {
                New(t) => t,
                Changed(t) => t.clone(),
                Same(_) => continue,
            };

            // We have a new type, so we need to clone the entire list.
            let mut res = Vec::with_capacity(v.len());
            res.extend(v[0..i].iter().cloned());
            res.push(t);

            // Copy all remaining elements.
            res.extend(v[i + 1..].iter().map(|t| self.concrete(t).into_owned()));
            return New(res);
        }

        // All types are the same, so we can just return the original list.
        Same(v)
    }

    fn doc_comment(&self, pos: BytePos) -> Option<String> {
        self.state
            .lookup_module(self.module)
            .and_then(|m| m.base.preceding_comments(pos.into()))
    }
}
