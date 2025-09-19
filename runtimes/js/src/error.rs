use crate::{log::parse_js_stack, pvalue::parse_pvalues};
use encore_runtime_core::api;
use napi::{Env, JsUnknown};

pub fn coerce_to_api_error(env: Env, val: napi::JsUnknown) -> Result<api::Error, api::Error> {
    let obj = val.coerce_to_object().map_err(|_| api::Error {
        code: api::ErrCode::Internal,
        message: api::ErrCode::Internal.default_public_message().into(),
        internal_message: Some("an unknown exception was thrown".into()),
        details: None,
        stack: None,
    })?;

    // Get the details field.
    let details = obj
        .get_named_property::<napi::JsUnknown>("details")
        .and_then(parse_pvalues)
        .map(|val| val.map(Box::new))
        .map_err(|e| api::Error {
            code: api::ErrCode::Internal,
            message: api::ErrCode::Internal.default_public_message().into(),
            internal_message: Some(format!("unable to parse error details: {e}")),
            details: None,
            stack: None,
        })?;

    // Get the message field.
    let mut message: String = obj
        .get_named_property::<JsUnknown>("message")
        .and_then(|val| val.coerce_to_string())
        .and_then(|val| env.from_js_value(val))
        .map_err(|e| api::Error {
            code: api::ErrCode::Internal,
            message: api::ErrCode::Internal.default_public_message().into(),
            internal_message: Some(format!("unable to parse error message: {e}")),
            details: None,
            stack: None,
        })?;

    // Get the error code field.
    let code: api::ErrCode = obj
        .get_named_property::<JsUnknown>("code")
        .and_then(|val| val.coerce_to_string())
        .and_then(|val| env.from_js_value::<String, _>(val))
        .map(|val| {
            val.parse::<api::ErrCode>()
                .unwrap_or(api::ErrCode::Internal)
        })
        .unwrap_or(api::ErrCode::Internal);

    // Get the cause field
    let cause: Option<String> = obj
        .get_named_property::<JsUnknown>("cause")
        .ok()
        .and_then(|val| {
            let val_type = val.get_type().ok()?;
            if val_type == napi::ValueType::Null || val_type == napi::ValueType::Undefined {
                None
            } else {
                val.coerce_to_object()
                    .and_then(|val| val.get_named_property::<JsUnknown>("message"))
                    .and_then(|val| val.coerce_to_string())
                    .and_then(|val| env.from_js_value(val))
                    .ok()
            }
        });

    if let Some(cause) = cause {
        message.push_str(": ");
        message.push_str(&cause);
    }

    // Get the JS stack
    let stack = obj
        .get_named_property::<JsUnknown>("stack")
        .and_then(|val| parse_js_stack(&env, val))
        .ok();

    let mut internal_message = None;
    if code == api::ErrCode::Internal {
        internal_message = Some(message);
        message = api::ErrCode::Internal.default_public_message().into();
    }

    Ok(api::Error {
        code,
        message,
        stack,
        internal_message,
        details,
    })
}
