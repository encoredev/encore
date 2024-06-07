extern crate proc_macro;

use quote::{format_ident, quote, quote_spanned};
use syn::spanned::Spanned;
use syn::{parse_macro_input, parse_quote, Data, DeriveInput, Fields, GenericParam, Generics};

#[proc_macro_derive(LitParser)]
pub fn derive_lit_parser(input: proc_macro::TokenStream) -> proc_macro::TokenStream {
    // Parse the input tokens into a syntax tree
    let input = parse_macro_input!(input as DeriveInput);

    let name = input.ident;

    // Add a bound `T: LitParser` to every type parameter T.
    let generics = add_trait_bounds(input.generics);
    let (impl_generics, ty_generics, where_clause) = generics.split_for_impl();

    let input_ident = format_ident!("input");
    let impl_stream = generate_impl(&input.data, &input_ident);

    // Build the output, possibly using quasi-quotation
    let expanded = quote! {
        // The generated impl.
        #[allow(non_snake_case)]
        impl #impl_generics litparser::LitParser for #name #ty_generics #where_clause {
            fn parse_lit(#input_ident: &swc_ecma_ast::Expr) -> anyhow::Result<Self> {
                #impl_stream
            }
        }
    };

    // Hand the output tokens back to the compiler.
    proc_macro::TokenStream::from(expanded)
}

// Add a bound `T: LitParser` to every type parameter T.
fn add_trait_bounds(mut generics: Generics) -> Generics {
    for param in &mut generics.params {
        if let GenericParam::Type(ref mut type_param) = *param {
            type_param
                .bounds
                .push(parse_quote!(tsparser::litparser::LitParser));
        }
    }
    generics
}

// Generate an implementation to parse literals for a type.
fn generate_impl(data: &Data, input_ident: &syn::Ident) -> proc_macro2::TokenStream {
    let init_stream = fields_init(data);
    let match_stream = match_expr(data, input_ident);
    let return_stream = gen_return(data);
    match *data {
        Data::Struct(ref data) => match data.fields {
            Fields::Named(_) => {
                quote! {
                    #init_stream
                    #match_stream
                    #return_stream
                }
            }
            Fields::Unnamed(_) => {
                unimplemented!()
            }
            Fields::Unit => {
                unimplemented!()
            }
        },
        Data::Enum(_) | Data::Union(_) => unimplemented!(),
    }
}

/// Turns a struct into a list of field initialization statements in the form:
///    let field_name: ::std::option::Option<field_type> = None;
fn fields_init(data: &Data) -> proc_macro2::TokenStream {
    match *data {
        Data::Struct(ref data) => match data.fields {
            Fields::Named(ref fields) => {
                let inits = fields.named.iter().map(|f| {
                    let name = &f.ident;
                    let typ = &f.ty;
                    quote_spanned! {f.span() =>
                        let mut #name: ::std::option::Option<#typ> = None;
                    }
                });
                quote! { #(#inits)* }
            }
            Fields::Unnamed(_) => {
                todo!()
            }
            Fields::Unit => {
                todo!()
            }
        },
        Data::Enum(_) | Data::Union(_) => unimplemented!(),
    }
}

fn match_expr(data: &Data, input_ident: &syn::Ident) -> proc_macro2::TokenStream {
    let lit_ident = format_ident!("lit");
    let prop_ident = format_ident!("prop");
    let kv_ident = format_ident!("kv");
    let field_case_stream = gen_field_match_cases(data, &kv_ident);
    let match_prop_stream = match_prop(&prop_ident, &kv_ident, field_case_stream);
    quote! {
        match #input_ident {
            swc_ecma_ast::Expr::Object(ref #lit_ident) => {
                for #prop_ident in &#lit_ident.props {
                    #match_prop_stream
                }
            }
            _ => anyhow::bail!("expected object literal"),
        }
    }
}

// Generates an expression for matching a prop with the given prop expression.
fn match_prop(
    prop_ident: &syn::Ident,
    kv_ident: &syn::Ident,
    match_case_stream: proc_macro2::TokenStream,
) -> proc_macro2::TokenStream {
    quote! {
        match #prop_ident {
            swc_ecma_ast::PropOrSpread::Spread(_) => anyhow::bail!("spread operator not supported"),
            swc_ecma_ast::PropOrSpread::Prop(prop) => match prop.as_ref() {
                swc_ecma_ast::Prop::Shorthand(_)
                | swc_ecma_ast::Prop::Assign(_)
                | swc_ecma_ast::Prop::Getter(_)
                | swc_ecma_ast::Prop::Setter(_)
                | swc_ecma_ast::Prop::Method(_) => {
                    anyhow::bail!("prop type {:?} not supported", prop)
                }

                swc_ecma_ast::Prop::KeyValue(#kv_ident) => match &#kv_ident.key {
                    swc_ecma_ast::PropName::Ident(ident) => match ident.sym.as_ref() {
                        #match_case_stream
                    },
                    swc_ecma_ast::PropName::Str(str) => match str.value.as_ref() {
                        #match_case_stream
                    }
                    swc_ecma_ast::PropName::Num(_)
                    | swc_ecma_ast::PropName::BigInt(_)
                    | swc_ecma_ast::PropName::Computed(_) => {
                        anyhow::bail!("prop name kind {:?} not supported", kv.key)
                    }
                },
            }
        }
    }
}

// Generates an expression for the match cases for the fields.
fn gen_field_match_cases(data: &Data, kv_ident: &syn::Ident) -> proc_macro2::TokenStream {
    match *data {
        Data::Struct(ref data) => match data.fields {
            Fields::Named(ref fields) => {
                let match_cases = fields.named.iter().map(|f| {
                    let name = &f.ident;
                    let match_literal = format!("{}", name.as_ref().unwrap());
                    quote_spanned! {f.span() =>
                        #match_literal => {
                            if #name.is_some() {
                                anyhow::bail!("field {} set twice", #match_literal);
                            }
                            let val = LitParser::parse_lit(&*#kv_ident.value)?;
                            #name = Some(val);
                        }
                    }
                });
                quote! {
                    #(#match_cases)*
                    x @ _ => anyhow::bail!("unrecognized prop name {}", x),
                }
            }
            Fields::Unnamed(_) => {
                todo!()
            }
            Fields::Unit => {
                todo!()
            }
        },
        Data::Enum(_) | Data::Union(_) => unimplemented!(),
    }
}

fn gen_return(data: &Data) -> proc_macro2::TokenStream {
    match *data {
        Data::Struct(ref data) => match data.fields {
            Fields::Named(ref fields) => {
                let field_names = fields.named.iter().map(|f| {
                    let name = &f.ident;

                    if is_optional(&f.ty) {
                        quote_spanned! {f.span() =>
                            #name: #name.flatten()
                        }
                    } else {
                        quote_spanned! {f.span() =>
                            #name: #name.ok_or_else(|| anyhow::anyhow!(concat!(stringify!(#name), " not set")))?
                        }
                    }
                });
                quote! {
                    Ok(Self {
                        #(#field_names),*
                    })
                }
            }
            Fields::Unnamed(_) => {
                todo!()
            }
            Fields::Unit => {
                todo!()
            }
        },
        Data::Enum(_) | Data::Union(_) => unimplemented!(),
    }
}

fn is_optional(ty: &syn::Type) -> bool {
    match ty {
        syn::Type::Path(syn::TypePath {
            qself: None,
            path: syn::Path { segments, .. },
        }) => {
            // Return true if the last path segment is "Option".
            segments.last().map_or(false, |seg| seg.ident == "Option")
        }
        _ => false,
    }
}
