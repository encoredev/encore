use std::fmt::Display;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

pub trait Fetcher: Clone + Sync + Send {
    type Item;
    type Error: Display;

    fn fetch(
        self,
        max_items: usize,
    ) -> Pin<Box<dyn Future<Output = Result<Vec<Self::Item>, Self::Error>> + Send + 'static>>;
    fn process(self, item: Self::Item) -> Pin<Box<dyn Future<Output = ()> + Send + 'static>>;
}

#[derive(Debug, Clone)]
pub struct Config {
    /// The maximum number of items to process at once.
    pub max_concurrency: usize,

    /// The maximum number of items to fetch at once.
    pub max_batch_size: usize,
}

pub async fn process_concurrently<F: Fetcher>(cfg: Config, fetcher: F) {
    // Semaphore representing work being processed.
    let sem = Arc::new(tokio::sync::Semaphore::new(cfg.max_concurrency));

    // The effective max batch size is the minimum of the maximum concurrency
    // and the maximum batch size.
    let max_batch = cfg.max_concurrency.min(cfg.max_batch_size);

    // Retry policy configuration.
    let (base_sleep, max_sleep, mut err_sleep) = {
        use tokio::time::Duration;
        let base = Duration::from_millis(150);
        let max = Duration::from_secs(5);
        (base, max, base)
    };

    // Fetch work.
    loop {
        // How many items shall we fetch?
        let (to_fetch, permit) = {
            // Wait for at least one permit.
            let mut permit = sem.acquire().await.expect("semaphore is closed");

            // Do we have any additional available permits and the max batch size allows for it?
            let extra = sem.available_permits().min(max_batch - 1);
            if extra > 0 {
                // Acquire additional permits. Guaranteed to succeed since we've checked
                // the available permits, and there's no race since this task is the only one
                // acquiring permits.
                let extra_permit = sem
                    .acquire_many(extra as u32)
                    .await
                    .expect("semaphore is closed");
                permit.merge(extra_permit);
            }
            (1 + extra, permit)
        };

        let fetch_result = fetcher.clone().fetch(to_fetch).await;
        match fetch_result {
            Ok(work) => {
                err_sleep = base_sleep;

                // Process the work. We forget the permit here,
                // and release individual items back to the semaphore as we process them.
                permit.forget();
                for item in work {
                    let fut = fetcher.clone().process(item);
                    let sem = sem.clone();
                    tokio::spawn(async move {
                        fut.await;
                        sem.add_permits(1);
                    });
                }
            }
            Err(err) => {
                log::error!("encore: pub/sub fetch error: {err}, retrying.");
                tokio::time::sleep(err_sleep).await;
                err_sleep = err_sleep.mul_f32(1.5).min(max_sleep);
            }
        }
    }
}
