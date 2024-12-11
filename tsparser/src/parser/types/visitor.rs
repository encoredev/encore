use std::collections::HashSet;

use litparser::Sp;

use super::{
    validation, Array, Basic, ClassType, Conditional, Custom, EnumType, Generic, Index, Inferred,
    Interface, InterfaceField, Intersection, Keyof, Literal, Mapped, MappedKeyType, Named,
    ObjectId, Optional, ResolveState, This, Tuple, Type, TypeParam, Union, Validated, WireSpec,
};

pub trait Visit {
    fn resolve_state(&self) -> &ResolveState;
    fn seen_decls(&mut self) -> &mut HashSet<ObjectId>;

    #[inline]
    fn visit_basic(&mut self, node: &Basic) {
        <Basic as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_array(&mut self, node: &Array) {
        <Array as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_interface(&mut self, node: &Interface) {
        <Interface as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_interface_field(&mut self, node: &InterfaceField) {
        <InterfaceField as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_union(&mut self, node: &Union) {
        <Union as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_tuple(&mut self, node: &Tuple) {
        <Tuple as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_literal(&mut self, node: &Literal) {
        <Literal as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_class(&mut self, node: &ClassType) {
        <ClassType as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_enum(&mut self, node: &EnumType) {
        <EnumType as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_named(&mut self, node: &Named) {
        <Named as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_optional(&mut self, node: &Optional) {
        <Optional as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_this(&mut self, node: &This) {
        <This as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_validation(&mut self, node: &validation::Expr) {
        <validation::Expr as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_validated(&mut self, node: &Validated) {
        <Validated as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_type(&mut self, node: &Type) {
        <Type as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_types(&mut self, node: &[Type]) {
        <[Type] as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_generic(&mut self, node: &Generic) {
        <Generic as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_type_param(&mut self, node: &TypeParam) {
        <TypeParam as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_index(&mut self, node: &Index) {
        <Index as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_mapped(&mut self, node: &Mapped) {
        <Mapped as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_mapped_key_type(&mut self, node: &MappedKeyType) {
        <MappedKeyType as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_keyof(&mut self, node: &Keyof) {
        <Keyof as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_conditional(&mut self, node: &Conditional) {
        <Conditional as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_intersection(&mut self, node: &Intersection) {
        <Intersection as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_inferred(&mut self, node: &Inferred) {
        <Inferred as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_custom(&mut self, node: &Custom) {
        <Custom as VisitWith<Self>>::visit_children_with(node, self)
    }

    #[inline]
    fn visit_wire_spec(&mut self, node: &WireSpec) {
        <WireSpec as VisitWith<Self>>::visit_children_with(node, self)
    }
}

pub trait VisitWith<V: ?Sized + Visit> {
    fn visit_with(&self, visitor: &mut V);
    fn visit_children_with(&self, visitor: &mut V);
}

impl<V> VisitWith<V> for This
where
    V: ?Sized + Visit,
{
    #[inline]
    fn visit_with(&self, _visitor: &mut V) {}

    #[inline]
    fn visit_children_with(&self, _visitor: &mut V) {}
}

impl<V: ?Sized + Visit> VisitWith<V> for Basic {
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_basic(visitor, self)
    }
    fn visit_children_with(&self, _visitor: &mut V) {}
}

impl<V: ?Sized + Visit> VisitWith<V> for Interface {
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_interface(visitor, self)
    }

    fn visit_children_with(&self, visitor: &mut V) {
        self.fields
            .iter()
            .for_each(|item| <InterfaceField as VisitWith<V>>::visit_with(item, visitor))
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for InterfaceField {
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_interface_field(visitor, self)
    }

    fn visit_children_with(&self, visitor: &mut V) {
        <Type as VisitWith<V>>::visit_with(&self.typ, visitor)
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Array {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_array(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <Type as VisitWith<V>>::visit_with(&self.0, visitor)
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Union {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_union(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        self.types
            .iter()
            .for_each(|item| <Type as VisitWith<V>>::visit_with(item, visitor))
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Tuple {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_tuple(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        self.types
            .iter()
            .for_each(|item| <Type as VisitWith<V>>::visit_with(item, visitor))
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Optional {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_optional(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <Type as VisitWith<V>>::visit_with(&self.0, visitor)
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for validation::Expr {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_validation(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, _visitor: &mut V) {}
}

impl<V: ?Sized + Visit> VisitWith<V> for Validated {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_validated(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <Type as VisitWith<V>>::visit_with(&self.typ, visitor);
        <validation::Expr as VisitWith<V>>::visit_with(&self.expr, visitor);
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Literal {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_literal(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, _visitor: &mut V) {}
}

impl<V: ?Sized + Visit> VisitWith<V> for ClassType {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_class(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, _visitor: &mut V) {}
}

impl<V: ?Sized + Visit> VisitWith<V> for EnumType {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_enum(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, _visitor: &mut V) {}
}

impl<V: ?Sized + Visit> VisitWith<V> for TypeParam {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_type_param(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        if let Some(constraint) = &self.constraint {
            <Type as VisitWith<V>>::visit_with(constraint, visitor);
        }
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Index {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_index(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <Type as VisitWith<V>>::visit_with(&self.source, visitor);
        <Type as VisitWith<V>>::visit_with(&self.index, visitor);
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Mapped {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_mapped(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <Type as VisitWith<V>>::visit_with(&self.in_type, visitor);
        <Type as VisitWith<V>>::visit_with(&self.value_type, visitor);
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for MappedKeyType {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_mapped_key_type(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, _visitor: &mut V) {}
}

impl<V: ?Sized + Visit> VisitWith<V> for Keyof {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_keyof(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <Type as VisitWith<V>>::visit_with(&self.0, visitor);
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Conditional {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_conditional(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <Type as VisitWith<V>>::visit_with(&self.check_type, visitor);
        <Type as VisitWith<V>>::visit_with(&self.extends_type, visitor);
        <Type as VisitWith<V>>::visit_with(&self.true_type, visitor);
        <Type as VisitWith<V>>::visit_with(&self.false_type, visitor);
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Intersection {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_intersection(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <Type as VisitWith<V>>::visit_with(&self.x, visitor);
        <Type as VisitWith<V>>::visit_with(&self.y, visitor);
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Inferred {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_inferred(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, _visitor: &mut V) {}
}

impl<V: ?Sized + Visit> VisitWith<V> for Named {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_named(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        // Only recurse if we haven't seen this object before, to avoid infinite recursion
        // with recursive types.
        if visitor.seen_decls().insert(self.obj.id) {
            let underlying = self.underlying(visitor.resolve_state());
            <Type as VisitWith<V>>::visit_with(&underlying, visitor);
        }

        self.type_arguments
            .iter()
            .for_each(|item| <Type as VisitWith<V>>::visit_with(item, visitor))
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Generic {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_generic(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        match self {
            Generic::TypeParam(inner) => <TypeParam as VisitWith<V>>::visit_with(inner, visitor),
            Generic::Index(inner) => <Index as VisitWith<V>>::visit_with(inner, visitor),
            Generic::Mapped(inner) => <Mapped as VisitWith<V>>::visit_with(inner, visitor),
            Generic::MappedKeyType(inner) => {
                <MappedKeyType as VisitWith<V>>::visit_with(inner, visitor)
            }
            Generic::Keyof(inner) => <Keyof as VisitWith<V>>::visit_with(inner, visitor),
            Generic::Conditional(inner) => {
                <Conditional as VisitWith<V>>::visit_with(inner, visitor)
            }
            Generic::Intersection(inner) => {
                <Intersection as VisitWith<V>>::visit_with(inner, visitor)
            }
            Generic::Inferred(inner) => <Inferred as VisitWith<V>>::visit_with(inner, visitor),
        }
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Custom {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_custom(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        match self {
            Custom::WireSpec(inner) => <WireSpec as VisitWith<V>>::visit_with(inner, visitor),
        }
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for WireSpec {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_wire_spec(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <Type as VisitWith<V>>::visit_with(&self.underlying, visitor)
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for Type {
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_type(visitor, self)
    }

    fn visit_children_with(&self, visitor: &mut V) {
        match self {
            Self::Basic(basic) => <Basic as VisitWith<V>>::visit_with(basic, visitor),
            Self::Array(array) => <Array as VisitWith<V>>::visit_with(array, visitor),
            Type::Interface(interface) => {
                <Interface as VisitWith<V>>::visit_with(interface, visitor)
            }
            Type::Union(union) => <Union as VisitWith<V>>::visit_with(union, visitor),
            Type::Tuple(tuple) => <Tuple as VisitWith<V>>::visit_with(tuple, visitor),
            Type::Literal(literal) => <Literal as VisitWith<V>>::visit_with(literal, visitor),
            Type::Class(class_type) => <ClassType as VisitWith<V>>::visit_with(class_type, visitor),
            Type::Enum(enum_type) => <EnumType as VisitWith<V>>::visit_with(enum_type, visitor),
            Type::Named(named) => <Named as VisitWith<V>>::visit_with(named, visitor),
            Type::Optional(opt) => <Optional as VisitWith<V>>::visit_with(opt, visitor),
            Type::This(t) => <This as VisitWith<V>>::visit_with(t, visitor),
            Type::Generic(generic) => <Generic as VisitWith<V>>::visit_with(generic, visitor),
            Type::Validation(expr) => <validation::Expr as VisitWith<V>>::visit_with(expr, visitor),
            Type::Validated(expr) => <Validated as VisitWith<V>>::visit_with(expr, visitor),
            Type::Custom(custom) => <Custom as VisitWith<V>>::visit_with(custom, visitor),
        }
    }
}

impl<V: ?Sized + Visit> VisitWith<V> for [Type] {
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <V as Visit>::visit_types(visitor, self)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        self.iter()
            .for_each(|item| <Type as VisitWith<V>>::visit_with(item, visitor))
    }
}

impl<V, T> VisitWith<V> for std::boxed::Box<T>
where
    V: ?Sized + Visit,
    T: VisitWith<V>,
{
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <T as VisitWith<V>>::visit_with(&**self, visitor)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <T as VisitWith<V>>::visit_children_with(&**self, visitor)
    }
}

impl<V, T> VisitWith<V> for Sp<T>
where
    V: ?Sized + Visit,
    T: VisitWith<V>,
{
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <T as VisitWith<V>>::visit_with(&**self, visitor)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <T as VisitWith<V>>::visit_children_with(&**self, visitor)
    }
}

impl<V, T> VisitWith<V> for std::vec::Vec<T>
where
    V: ?Sized + Visit,
    [T]: VisitWith<V>,
{
    #[inline]
    fn visit_with(&self, visitor: &mut V) {
        <[T] as VisitWith<V>>::visit_with(self, visitor)
    }

    #[inline]
    fn visit_children_with(&self, visitor: &mut V) {
        <[T] as VisitWith<V>>::visit_children_with(self, visitor)
    }
}
