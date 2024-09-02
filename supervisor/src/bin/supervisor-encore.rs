use encore_supervisor::supervisor::{Process, Supervisor};
use tokio_util::sync::CancellationToken;

#[tokio::main]
pub async fn main() {
    env_logger::init();
    let procs = vec![Process {
        name: "echo".to_string(),
        program: "echo".to_string(),
        args: vec!["hello".to_string(), "world".to_string()],
        env: vec![],
        cwd: std::env::current_dir().unwrap(),
        restart_policy: Box::new(std::iter::empty()),
    }];

    let sv = Supervisor::new(procs);
    let token = CancellationToken::new();
    sv.supervise(token).await
}
