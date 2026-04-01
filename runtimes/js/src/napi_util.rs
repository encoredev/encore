use napi::{Either, Env, JsFunction, JsObject, JsUnknown, NapiRaw, NapiValue};
use std::sync::RwLock;

pub trait PromiseHandler: Clone + Send + Sync + 'static {
    type Output: Send + 'static;

    fn resolve(&self, env: Env, val: Option<napi::JsUnknown>) -> Self::Output;
    fn reject(&self, env: Env, val: napi::JsUnknown) -> Self::Output;
    fn error(&self, env: Env, err: napi::Error) -> Self::Output;
}

/// A clonable oneshot sender. Uses `Arc<Mutex<Option<oneshot::Sender>>>` so it
/// can be shared between resolve and reject `.then()` callbacks, with only the
/// first one to fire actually sending.
pub struct OnceSender<T> {
    inner: std::sync::Arc<std::sync::Mutex<Option<tokio::sync::oneshot::Sender<T>>>>,
}

impl<T> OnceSender<T> {
    pub fn new(tx: tokio::sync::oneshot::Sender<T>) -> Self {
        Self {
            inner: std::sync::Arc::new(std::sync::Mutex::new(Some(tx))),
        }
    }

    pub fn send(&self, val: T) {
        if let Some(tx) = self.inner.lock().expect("OnceSender mutex poisoned").take() {
            _ = tx.send(val);
        }
    }
}

impl<T> Clone for OnceSender<T> {
    fn clone(&self) -> Self {
        Self {
            inner: self.inner.clone(),
        }
    }
}

pub fn await_promise<T, H>(env: Env, result: JsUnknown, tx: OnceSender<T>, handler: H)
where
    H: PromiseHandler<Output = T>,
    T: Send + 'static,
{
    // An inner function to enable using ? for error handling.
    let outer_handler = handler.clone();
    let outer_tx = tx.clone();
    let inner = move || -> napi::Result<()> {
        // If the result is a promise, wait for it to resolve, and send the result to the channel.
        // Otherwise, send the result immediately.
        if result.is_promise()? {
            let result: JsObject = result.try_into()?;
            let then: JsFunction = result.get_named_property("then")?;

            let cb = {
                let handler = handler.clone();
                let tx = tx.clone();
                env.create_function_from_closure("callback", move |ctx| {
                    let res = match ctx.try_get::<JsUnknown>(0) {
                        Ok(Either::A(success)) => handler.resolve(env, Some(success)),
                        Ok(Either::B(_)) => handler.resolve(env, None),
                        Err(err) => handler.error(env, err),
                    };

                    tx.send(res);
                    ctx.env.get_undefined()
                })?
            };

            let eb = {
                let handler = handler.clone();
                env.create_function_from_closure("error_callback", move |ctx| {
                    let res = match ctx.get(0) {
                        Ok(exception) => handler.reject(env, exception),
                        Err(err) => handler.error(env, err),
                    };

                    tx.send(res);
                    ctx.env.get_undefined()
                })?
            };

            then.call(Some(&result), &[cb, eb])?;
        } else {
            let res = handler.resolve(env, Some(result));
            tx.send(res);
        }

        Ok(())
    };

    inner().unwrap_or_else(move |err| {
        let res = outer_handler.error(env, err);
        outer_tx.send(res);
    });
}

/// The error type returned by [`call_function`] when the JS function call fails.
pub enum CallError {
    /// The JS function threw an exception. Contains the thrown JS value
    /// (e.g. an APIError instance) so the caller can inspect it.
    Exception(JsUnknown),
    /// A NAPI-level error occurred (not a JS exception).
    Error(napi::Error),
}

/// Calls a JS function using the raw NAPI C API, returning either the result
/// value or a [`CallError`] that preserves the thrown JS exception object.
/// This avoids going through napi-rs's `.call()` which wraps exceptions in
/// `napi::Error` (losing the original JS value needed for e.g. APIError inspection).
pub fn call_function<V: NapiRaw>(
    env: Env,
    func: &JsFunction,
    this: Option<&JsObject>,
    args: &[V],
) -> Result<JsUnknown, CallError> {
    use napi::sys;
    use std::ptr;

    unsafe {
        let raw_env = env.raw();
        let raw_this = this
            .map(|v| v.raw())
            .or_else(|| env.get_undefined().ok().map(|u| u.raw()))
            .ok_or_else(|| {
                CallError::Error(napi::Error::new(
                    napi::Status::GenericFailure,
                    "Get raw this failed".to_owned(),
                ))
            })?;
        let raw_args = args
            .iter()
            .map(|arg| arg.raw())
            .collect::<Vec<sys::napi_value>>();
        let mut result = ptr::null_mut();

        let status = sys::napi_call_function(
            raw_env,
            raw_this,
            func.raw(),
            raw_args.len(),
            raw_args.as_ptr(),
            &mut result,
        );

        match status {
            sys::Status::napi_ok => Ok(JsUnknown::from_raw_unchecked(raw_env, result)),
            sys::Status::napi_pending_exception => {
                let mut exception = ptr::null_mut();
                assert_eq!(
                    sys::napi_get_and_clear_last_exception(raw_env, &mut exception),
                    sys::Status::napi_ok,
                );
                Err(CallError::Exception(JsUnknown::from_raw_unchecked(
                    raw_env, exception,
                )))
            }
            _ => Err(CallError::Error(napi::Error::new(
                napi::Status::from(status),
                "".to_owned(),
            ))),
        }
    }
}

/// EnvMap is a thread-safe map that stores values associated with Env objects.
/// It is intended for storing one value per napi_env. We need the map to work with
/// worker pooling, where we can have multiple napi envs that each need their own copy.
/// It uses a vector under the hood since the number of values is small (one per worker).
pub struct EnvMap<T> {
    map: RwLock<Vec<(usize, T)>>,
}

impl<T> EnvMap<T> {
    pub const fn new() -> Self {
        Self {
            map: RwLock::new(Vec::new()),
        }
    }

    pub fn get(&self, env: Env) -> Option<T>
    where
        T: Clone,
    {
        let elems = self.map.read().unwrap();
        for (addr, value) in elems.iter() {
            if *addr == env.raw().addr() {
                return Some(value.clone());
            }
        }
        None
    }

    pub fn get_or_init<F>(&self, env: Env, init: F) -> T
    where
        T: Clone,
        F: FnOnce() -> T,
    {
        let addr = env.raw().addr();

        // First try to read
        let num_scanned = {
            let map = self.map.read().unwrap();
            for (key, value) in map.iter() {
                if *key == addr {
                    return value.clone();
                }
            }
            map.len()
        };

        // If not found, get write lock and initialize
        let mut map = self.map.write().unwrap();

        // Double-check in case another thread initialized it.
        // We only need to check from the last scanned index
        for (key, value) in map[num_scanned..].iter() {
            if *key == addr {
                return value.clone();
            }
        }

        let value = init();
        map.push((addr, value.clone()));
        value
    }
}

impl<T> Default for EnvMap<T> {
    fn default() -> Self {
        Self::new()
    }
}
