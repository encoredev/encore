use std::sync::{
    atomic::{AtomicU64, Ordering},
    Arc,
};

use crate::metrics::{CounterOps, GaugeOps};

impl CounterOps<u64> for AtomicU64 {
    fn increment(&self, value: u64) {
        self.fetch_add(value, Ordering::Release);
    }

    fn get(&self) -> crate::metrics::MetricValue {
        crate::metrics::MetricValue::CounterU64(self.load(Ordering::Acquire))
    }
}

impl CounterOps<i64> for AtomicU64 {
    fn increment(&self, value: i64) {
        self.fetch_add(value as u64, Ordering::Release);
    }

    fn get(&self) -> crate::metrics::MetricValue {
        crate::metrics::MetricValue::CounterI64(self.load(Ordering::Acquire) as i64)
    }
}

impl<T> CounterOps<T> for Arc<AtomicU64>
where
    AtomicU64: CounterOps<T>,
{
    fn increment(&self, value: T) {
        CounterOps::<T>::increment(&(**self), value)
    }

    fn get(&self) -> crate::metrics::MetricValue {
        CounterOps::<T>::get(&(**self))
    }
}

impl GaugeOps<u64> for AtomicU64 {
    fn set(&self, value: u64) {
        self.swap(value, Ordering::AcqRel);
    }

    fn get(&self) -> crate::metrics::MetricValue {
        crate::metrics::MetricValue::GaugeU64(self.load(Ordering::Acquire))
    }
}

impl GaugeOps<i64> for AtomicU64 {
    fn set(&self, value: i64) {
        self.swap(value as u64, Ordering::AcqRel);
    }

    fn get(&self) -> crate::metrics::MetricValue {
        crate::metrics::MetricValue::GaugeI64(self.load(Ordering::Acquire) as i64)
    }
}

impl GaugeOps<f64> for AtomicU64 {
    fn set(&self, value: f64) {
        self.swap(value.to_bits(), Ordering::AcqRel);
    }

    fn get(&self) -> crate::metrics::MetricValue {
        crate::metrics::MetricValue::GaugeF64(f64::from_bits(self.load(Ordering::Acquire)))
    }
}

impl<T> GaugeOps<T> for Arc<AtomicU64>
where
    AtomicU64: GaugeOps<T>,
{
    fn set(&self, value: T) {
        GaugeOps::<T>::set(&(**self), value)
    }

    fn get(&self) -> crate::metrics::MetricValue {
        GaugeOps::<T>::get(&(**self))
    }
}
