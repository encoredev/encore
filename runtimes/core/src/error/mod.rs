mod conversions;

use backtrace::SymbolName;
use colored::Colorize;
use serde::{Deserialize, Serialize};
use std::fmt::Display;

/// AppError represents an error that occurred in the application langauge
/// (i.e. in TypeScript code, not the rust runtime).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AppError {
    /// The error message
    pub message: String,

    /// The stack trace
    pub stack: StackTrace,

    /// The cause of the error (if any)
    pub cause: Option<Box<AppError>>,
}

impl AppError {
    /// Create a new AppError with the given message.
    ///
    /// Note: The stack trace will be set to the rust stack trace at the time this function is called.
    /// If you want to set the stack trace to something else, use `AppError::with_stack`.
    #[track_caller]
    pub fn new<S: Into<String>>(message: S) -> Self {
        Self {
            message: message.into(),
            stack: capture_stack_trace(),
            cause: None,
        }
    }

    /// Wrap an existing error with a new message.
    ///
    /// Note: The stack trace will be set to the rust stack trace at the time this function is called.
    /// If you want to set the stack trace to something else, use `AppError::with_stack`.
    #[track_caller]
    pub fn wrap<Err: Into<AppError>, S: Into<String>>(error: Err, message: S) -> Self {
        Self {
            message: message.into(),
            stack: capture_stack_trace(),
            cause: Some(Box::new(error.into())),
        }
    }

    /// Updates the stack trace of the error to the given stack trace
    pub fn with_stack(self, stack: StackTrace) -> Self {
        Self { stack, ..self }
    }

    /// Updates the cause of the error to the given error
    pub fn with_cause<Err: Into<AppError>>(self, cause: Err) -> Self {
        Self {
            cause: Some(Box::new(cause.into())),
            ..self
        }
    }

    /// Trims the stack trace to remove any frames from before
    /// the given file and line number.
    ///
    /// This is useful for removing frames caused by a conversion into an AppError.
    /// If the given file and line number are not found in the stack trace, then the original
    /// stack trace is returned.
    pub fn trim_stack(self, file: &str, line: u32, drop_extra: usize) -> Self {
        let idx = self
            .stack
            .iter()
            .position(|frame| frame.file == file && frame.line == line)
            .map(|idx| idx + 1 + drop_extra); // + 1 for the frame we called this on

        match idx {
            Some(idx) => Self {
                stack: self.stack.into_iter().skip(idx).collect(),
                ..self
            },
            None => return self,
        }
    }
}

const MAX_FRAMES_TO_DISPLAY: usize = 6;
const STACK_TAB_SIZE: &str = "  ";

/// Write the stack trace to the given formatter.
pub fn write_stack_trace<W: std::fmt::Write>(stack: &StackTrace, f: &mut W) -> std::fmt::Result {
    // If we have a stack trace add it
    if stack.len() > 0 {
        write!(f, "\n{}{}", STACK_TAB_SIZE, "Stack:".magenta())?;

        // What's the longest function name including module name (for modules not named "main")
        let mut longest_func = 0;
        for frame in stack.iter().take(MAX_FRAMES_TO_DISPLAY + 1) {
            if let Some(func) = &frame.function {
                longest_func = longest_func.max(
                    func.len() + // Function name length
                        frame.module.as_ref().map(|s| s.len() + 1).unwrap_or(0), // plus "[module]." if module is present
                );
            }
        }

        let mut count = 0;
        for frame in stack {
            let (module_and_function, readable_len) = match (&frame.module, &frame.function) {
                (Some(module), Some(function)) => (
                    format!("{}.{}", module.bright_black(), function.magenta()),
                    module.len() + function.len() + 1,
                ),
                (None, Some(function)) => (format!("{}", function.magenta()), function.len()),
                (Some(_), None) => ("".to_string(), 0),
                (None, None) => ("".to_string(), 0),
            };

            let spacing_after_function = if longest_func >= readable_len {
                " ".repeat(longest_func - readable_len)
            } else {
                "".to_string()
            };

            let line_and_column = frame
                .column
                .map(|col| format!("{}:{}", frame.line, col))
                .unwrap_or(format!("{}", frame.line));

            write!(
                f,
                "\n{}{}at {}{} {}:{}",
                STACK_TAB_SIZE,
                STACK_TAB_SIZE,
                module_and_function,
                spacing_after_function,
                frame.file,
                line_and_column
            )?;

            // Only print the first 6 frames
            if count >= MAX_FRAMES_TO_DISPLAY {
                write!(
                    f,
                    "\n{}{}... remaining {} frames omitted...",
                    STACK_TAB_SIZE,
                    STACK_TAB_SIZE,
                    stack.len() - count
                )?;
                break;
            }
            count += 1;
        }
    }

    Ok(())
}

/// Display the error in a human readable format.
impl Display for AppError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(
            f,
            "{} {}",
            "Error:".red(),
            self.message.replace("\n", "\n\t")
        )?;
        write_stack_trace(&self.stack, f)?;

        // While there are more causes, print them
        let mut cause = &self.cause;
        while cause.is_some() {
            let error = cause.as_ref().expect("cause should not be None");

            write!(f, "\n\n\t{} {}", "Caused by:".red(), error.message)?;
            write_stack_trace(&error.stack, f)?;
            cause = &error.cause;
        }

        Ok(())
    }
}

/// A stack trace from an error.
pub type StackTrace = Vec<StackFrame>;

/// A stack frame in a backtrace from an error.
///
/// Note: The serde field names, match those used in our Go `errinsrc` package.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct StackFrame {
    /// The name of the file on the file system
    #[serde(rename = "file")]
    pub file: String,

    /// The line number in that file
    #[serde(rename = "line")]
    pub line: u32,

    /// The column number in that file (if available)
    #[serde(skip_serializing)]
    pub column: Option<u32>,

    /// The name of the module containing the line (if available)
    #[serde(skip_serializing)]
    pub module: Option<String>,

    /// The name of the function or method containing the line (if available)
    #[serde(rename = "func")]
    pub function: Option<String>,
}

/// capture_stack_trace captures a stack trace from the current location in the rust code base
#[track_caller]
fn capture_stack_trace() -> StackTrace {
    let caller = std::panic::Location::caller();

    let backtrace = backtrace::Backtrace::new();

    let stacktrace: StackTrace = convert_backtrace_to_stack_trace(&backtrace)
        .into_iter()
        .skip_while(|frame| {
            // Skip all the frames before the caller
            !(frame.file.ends_with(caller.file()) && frame.line == caller.line())
        })
        .collect();

    // If the backtrace is empty, then we just return the caller which thanks to the
    // track_caller macro means we will always have at least one frame to report
    if stacktrace.len() == 0 {
        vec![StackFrame {
            file: caller.file().to_string(),
            line: caller.line(),
            column: None,
            module: None,
            function: None,
        }]
    } else {
        // otherwise we can report the full trace
        stacktrace
    }
}

/// convert_backtrace_to_stack_trace converts a rust backtrace to a stack trace
fn convert_backtrace_to_stack_trace(backtrace: &backtrace::Backtrace) -> StackTrace {
    let mut stack_trace = Vec::new();

    for frame in backtrace.frames() {
        if let Some(symbol) = frame.symbols().get(0) {
            match (symbol.filename(), symbol.lineno()) {
                (Some(filename), Some(line)) => {
                    let (module, function) = split_symbol_into_module_function(symbol.name());

                    let frame = StackFrame {
                        file: trim_file_path(filename.to_string_lossy().to_string()),
                        line,
                        column: symbol.colno(),
                        module,
                        function,
                    };

                    if !is_common_rust_frame(&frame) {
                        stack_trace.push(frame);
                    }
                }
                _ => continue,
            }
        }
    }

    return stack_trace;
}

fn split_symbol_into_module_function(
    symbol: Option<SymbolName>,
) -> (Option<String>, Option<String>) {
    match symbol {
        Some(symbol) => {
            let symbol_str = symbol.to_string();
            let parts: Vec<&str> = symbol_str.split("::").collect();
            if parts.len() < 3 {
                return (None, Some(symbol.to_string()));
            }

            let function_idx = parts
                .iter()
                .rposition(|s| s.starts_with(|c: char| c.is_uppercase()))
                .unwrap_or_else(|| {
                    parts
                        .iter()
                        .rposition(|s| (*s).eq("{{closure}}"))
                        .map(|idx| idx - 1)
                        .unwrap_or_else(|| parts.len() - 2)
                });

            let module = parts[..function_idx].join("::");
            let function = parts[function_idx..parts.len() - 1].join("::");

            (Some(module), Some(function))
        }
        None => (None, None),
    }
}

/// trim_file_path trims the file path to be relative to the root of the binary being compiled
fn trim_file_path(full_path: String) -> String {
    let compile_target = env!("ENCORE_BINARY_SRC_PATH");

    full_path
        .strip_prefix(compile_target)
        .map(|str| str[1..].to_string())
        .unwrap_or(full_path)
        .to_string()
}

fn is_common_rust_frame(frame: &StackFrame) -> bool {
    return frame.file.starts_with("/rustc/");
}
