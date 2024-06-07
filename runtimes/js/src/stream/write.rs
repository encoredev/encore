use std::collections::VecDeque;
use std::pin::Pin;
use std::task::{Context, Poll};

use bytes::Bytes;
use tokio::io::AsyncRead;
use tokio::sync::oneshot;

pub fn new() -> (WriteHalf, ReadHalf) {
    let (sender, receiver) = tokio::sync::mpsc::unbounded_channel();
    let write = WriteHalf { sender };
    let read = ReadHalf {
        events: receiver,
        done: false,
        done_callback: None,
        bufs: VecDeque::with_capacity(16),
        partial: None,
    };
    (write, read)
}

pub struct WriteHalf {
    sender: tokio::sync::mpsc::UnboundedSender<StreamEvent>,
}

impl WriteHalf {
    pub fn write(&mut self, data: Bytes, callback: Option<oneshot::Sender<()>>) {
        // TODO: Log error?
        _ = self.sender.send(StreamEvent {
            data: StreamEventData::Write(data),
            callback,
        });
    }

    pub fn writev(&mut self, bufs: Vec<Bytes>, callback: Option<oneshot::Sender<()>>) {
        // TODO: Log error?
        _ = self.sender.send(StreamEvent {
            data: StreamEventData::WriteMulti(bufs),
            callback,
        });
    }

    pub fn end(&mut self, callback: Option<oneshot::Sender<()>>) {
        // TODO: Log error?
        _ = self.sender.send(StreamEvent {
            data: StreamEventData::End,
            callback,
        });
    }
}

struct StreamEvent {
    data: StreamEventData,
    callback: Option<oneshot::Sender<()>>,
}

enum StreamEventData {
    Write(Bytes),
    WriteMulti(Vec<Bytes>),
    End,
}

pub struct ReadHalf {
    events: tokio::sync::mpsc::UnboundedReceiver<StreamEvent>,

    // True if the write end has been closed.
    done: bool,
    done_callback: Option<oneshot::Sender<()>>,

    // The bufs to read from.
    bufs: VecDeque<BufWithCB>,
    // The partially read buffer.
    partial: Option<PartiallyRead>,
}

struct BufWithCB {
    buf: Bytes,
    callback: Option<oneshot::Sender<()>>,
}

struct PartiallyRead {
    buf: Bytes,
    pos: usize,
    callback: Option<oneshot::Sender<()>>,
}

impl PartiallyRead {
    /// The number of bytes left to read in the buffer.
    fn len(&self) -> usize {
        self.buf.len() - self.pos
    }
}

impl AsyncRead for ReadHalf {
    fn poll_read(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut tokio::io::ReadBuf<'_>,
    ) -> Poll<std::io::Result<()>> {
        // First process any outstanding events.
        if !self.done {
            'EventLoop: loop {
                match self.events.poll_recv(cx) {
                    // The events channel has been closed.
                    Poll::Ready(None) => {
                        self.done = true;
                        break 'EventLoop;
                    }

                    // There is no more events to read at the moment.
                    Poll::Pending => break 'EventLoop,

                    Poll::Ready(Some(event)) => {
                        let mut callback = event.callback;
                        match event.data {
                            StreamEventData::Write(buf) => {
                                self.bufs.push_back(BufWithCB { buf, callback });
                            }
                            StreamEventData::WriteMulti(bufs) => {
                                let num_bufs = bufs.len();
                                self.bufs.reserve(num_bufs);
                                for (i, buf) in bufs.into_iter().enumerate() {
                                    // Only add the callback to the last buffer.
                                    let callback = if i == num_bufs - 1 {
                                        callback.take()
                                    } else {
                                        None
                                    };
                                    self.bufs.push_back(BufWithCB { buf, callback });
                                }
                            }
                            StreamEventData::End => {
                                self.done = true;
                                self.done_callback = callback;
                            }
                        }
                    }
                }
            }
        }

        let mut did_read = false;
        'ReadLoop: loop {
            let space_remaining = buf.remaining();
            if space_remaining == 0 || (self.bufs.is_empty() && self.partial.is_none()) {
                // No space remaining in the buffer, or no more data to read.
                break 'ReadLoop;
            }

            // Determine the buffer to read from.
            let mut partial = match self.partial.take() {
                Some(partial) => partial,
                None => {
                    // Find the next non-empty buffer to read from.
                    let next = 'BufferLoop: loop {
                        match self.bufs.pop_front() {
                            // Found a non-empty buffer.
                            Some(next) if !next.buf.is_empty() => break next,
                            // Found an empty buffer; skip it.
                            Some(_) => continue 'BufferLoop,
                            // No more buffers to read from.
                            None => continue 'ReadLoop,
                        }
                    };
                    PartiallyRead {
                        buf: next.buf,
                        pos: 0,
                        callback: next.callback,
                    }
                }
            };

            // Post-condition: we have a non-empty buffer to read from, and self.partial is None.
            assert!(self.partial.is_none());
            assert!(partial.len() > 0);

            if partial.len() > space_remaining {
                // We can't fit the whole buffer in the space remaining.
                let pos = partial.pos;
                buf.put_slice(&partial.buf[pos..(pos + space_remaining)]);

                // Store the partial read state in self.partial.
                partial.pos += space_remaining;
                self.partial = Some(partial);

                return Poll::Ready(Ok(()));
            } else {
                // We can fit the whole buffer in the space remaining.
                buf.put_slice(&partial.buf[partial.pos..]);

                if let Some(callback) = partial.callback {
                    _ = callback.send(());
                }

                // Store that we did read data so we can return Ready.
                did_read = true;
            }
        }

        if self.done {
            if let Some(done_callback) = self.done_callback.take() {
                _ = done_callback.send(());
            }
        }

        if did_read || self.done {
            // We read some data, or we're done, so we're ready.
            Poll::Ready(Ok(()))
        } else {
            Poll::Pending
        }
    }
}
