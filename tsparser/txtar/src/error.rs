use std::io;

use thiserror::Error;

#[derive(Error, Debug)]
pub enum MaterializeError {
    #[error("{0}")]
    Io(#[from] io::Error),
    #[error("{0}: outside parent directory")]
    DirEscape(String),
}
