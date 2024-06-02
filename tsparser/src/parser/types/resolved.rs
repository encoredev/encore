use std::borrow::{Borrow, Cow};
use std::cmp::Ordering;
use std::fmt;
use std::hash::{Hash, Hasher};
use std::ops::Deref;

use Resolved::*;

pub enum Resolved<'a, B: ?Sized + 'a>
where
    B: ToOwned,
{
    New(<B as ToOwned>::Owned),
    Changed(&'a B),
    Same(&'a B),
}

impl<'a, B: ?Sized + 'a> Into<Cow<'a, B>> for Resolved<'a, B>
where
    B: ToOwned,
{
    fn into(self) -> Cow<'a, B> {
        match self {
            New(owned) => Cow::Owned(owned),
            Changed(borrowed) | Same(borrowed) => Cow::Borrowed(borrowed),
        }
    }
}

impl<B: ?Sized + ToOwned> Clone for Resolved<'_, B> {
    fn clone(&self) -> Self {
        match *self {
            New(ref o) => {
                let b: &B = o.borrow();
                New(b.to_owned())
            }
            Changed(b) => Changed(b),
            Same(b) => Same(b),
        }
    }

    fn clone_from(&mut self, source: &Self) {
        match (self, source) {
            (&mut New(ref mut dest), &New(ref o)) => o.borrow().clone_into(dest),
            (t, s) => *t = s.clone(),
        }
    }
}

impl<B: ?Sized + ToOwned> Resolved<'_, B> {
    /// Acquires a mutable reference to the owned form of the data.
    ///
    /// Clones the data if it is not already owned.
    pub fn to_mut(&mut self) -> &mut <B as ToOwned>::Owned {
        match *self {
            Changed(borrowed) | Same(borrowed) => {
                *self = New(borrowed.to_owned());
                match *self {
                    Changed(..) | Same(..) => unreachable!(),
                    New(ref mut owned) => owned,
                }
            }
            New(ref mut owned) => owned,
        }
    }

    /// Converts `Same` to `Changed`.
    pub fn same_to_changed(self) -> Self {
        match self {
            Same(borrowed) => Changed(borrowed),
            _ => self,
        }
    }

    /// Extracts the owned data.
    ///
    /// Clones the data if it is not already owned.
    pub fn into_owned(self) -> <B as ToOwned>::Owned {
        match self {
            Same(borrowed) | Changed(borrowed) => borrowed.to_owned(),
            New(owned) => owned,
        }
    }
}

impl<B: ?Sized + ToOwned> Deref for Resolved<'_, B>
where
    B::Owned: Borrow<B>,
{
    type Target = B;

    fn deref(&self) -> &B {
        match *self {
            Changed(borrowed) | Same(borrowed) => borrowed,
            New(ref owned) => owned.borrow(),
        }
    }
}

impl<B: ?Sized> Eq for Resolved<'_, B> where B: Eq + ToOwned {}

impl<B: ?Sized> Ord for Resolved<'_, B>
where
    B: Ord + ToOwned,
{
    #[inline]
    fn cmp(&self, other: &Self) -> Ordering {
        Ord::cmp(&**self, &**other)
    }
}

impl<'a, 'b, B: ?Sized, C: ?Sized> PartialEq<Resolved<'b, C>> for Resolved<'a, B>
where
    B: PartialEq<C> + ToOwned,
    C: ToOwned,
{
    #[inline]
    fn eq(&self, other: &Resolved<'b, C>) -> bool {
        PartialEq::eq(&**self, &**other)
    }
}

impl<'a, B: ?Sized> PartialOrd for Resolved<'a, B>
where
    B: PartialOrd + ToOwned,
{
    #[inline]
    fn partial_cmp(&self, other: &Resolved<'a, B>) -> Option<Ordering> {
        PartialOrd::partial_cmp(&**self, &**other)
    }
}

impl<B: ?Sized, D> fmt::Debug for Resolved<'_, B>
where
    D: fmt::Debug,
    B: fmt::Debug + ToOwned<Owned = D>,
{
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match *self {
            Changed(ref b) => write!(f, "Changed({:?})", b),
            Same(ref b) => write!(f, "Same({:?})", b),
            New(ref o) => write!(f, "New({:?})", o),
        }
    }
}

impl<B: ?Sized, D> fmt::Display for Resolved<'_, B>
where
    D: fmt::Display,
    B: fmt::Display + ToOwned<Owned = D>,
{
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match *self {
            Changed(ref b) | Same(ref b) => fmt::Display::fmt(b, f),
            New(ref o) => fmt::Display::fmt(o, f),
        }
    }
}

impl<B: ?Sized, D> Default for Resolved<'_, B>
where
    D: Default,
    B: ToOwned<Owned = D>,
{
    /// Creates an owned Resolved<'a, B> with the default value for the contained owned value.
    fn default() -> Self {
        New(<B as ToOwned>::Owned::default())
    }
}

impl<B: ?Sized> Hash for Resolved<'_, B>
where
    B: Hash + ToOwned,
{
    #[inline]
    fn hash<H: Hasher>(&self, state: &mut H) {
        Hash::hash(&**self, state)
    }
}

impl<T: ?Sized + ToOwned> AsRef<T> for Resolved<'_, T> {
    fn as_ref(&self) -> &T {
        self
    }
}
