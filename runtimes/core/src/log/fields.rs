/// Fields control the names of the fields that are used in the log output.
#[derive(Debug)]
pub struct FieldConfig {
    pub timestamp_field_name: &'static str,

    pub level_field_name: &'static str,
    pub level_trace_value: &'static str,
    pub level_debug_value: &'static str,
    pub level_info_value: &'static str,
    pub level_warn_value: &'static str,
    pub level_error_value: &'static str,
    pub level_fatal_value: &'static str,

    pub message_field_name: &'static str,

    pub error_field_name: &'static str,

    pub caller_field_name: &'static str,

    pub stack_trace_field_name: &'static str,
}

pub static DEFAULT_FIELDS: FieldConfig = FieldConfig {
    timestamp_field_name: "time",

    level_field_name: "level",
    level_trace_value: "trace",
    level_debug_value: "debug",
    level_info_value: "info",
    level_warn_value: "warn",
    level_error_value: "error",
    level_fatal_value: "fatal",

    message_field_name: "message",

    error_field_name: "error",

    caller_field_name: "caller",

    stack_trace_field_name: "stack",
};

pub static GCP_FIELDS: FieldConfig = FieldConfig {
    timestamp_field_name: "timestamp",

    level_field_name: "severity",
    level_trace_value: "DEBUG",
    level_debug_value: "DEBUG",
    level_info_value: "INFO",
    level_warn_value: "WARNING",
    level_error_value: "ERROR",
    level_fatal_value: "CRITICAL",

    message_field_name: "message",

    error_field_name: "error",

    caller_field_name: "caller",

    stack_trace_field_name: "stacktrace",
};

impl FieldConfig {
    pub fn default() -> &'static FieldConfig {
        // If we're running in GCP, then we'll use the GCP fields.
        for var in &[
            "GCP_PROJECT",
            "GOOGLE_CLOUD_PROJECT",
            "GCP_METADATA_PROJECT",
        ] {
            if let Ok(_) = std::env::var(var) {
                return &GCP_FIELDS;
            }
        }
        &DEFAULT_FIELDS
    }
}
