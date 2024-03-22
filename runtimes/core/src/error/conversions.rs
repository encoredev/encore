use crate::error::{AppError};

impl From<&str> for AppError { fn from(message: &str) -> Self { AppError::new(message)  } }
impl From<String> for AppError { fn from(message: String) -> Self { AppError::new(message)  } }
impl From<anyhow::Error> for AppError {
    fn from(error: anyhow::Error) -> Self {
        let message = error.to_string();

        // Create a chain of causes
        let mut cause: Option<Box<AppError>> = None;
        while let Some(err) = error.chain().next_back() {
            cause = Some(Box::new(Self{
                message: err.to_string(),
                stack: vec![],
                cause,
            }));
        }

        return Self {
            message,
            stack: vec![],
            cause,
        }
    }
}