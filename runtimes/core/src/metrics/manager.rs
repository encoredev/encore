use crate::{
    encore::runtime::v1 as pb,
    metrics::{exporter, registry::Registry},
};
use std::time::Duration;

pub struct ManagerConfig<'a> {
    provider: &'a pb::metrics_provider::Provider,
    collection_interval: Duration,
}

enum Provider {
    EncoreCloud(exporter::Gcp),
    Gcp(exporter::Gcp),
    Aws(AwsCloudWatch),
    PromRemoteWrite(PrometheusRemoteWrite),
    Datadog(Datadog),
}

impl Provider {
    pub fn exporter(self) -> impl Exporter {
        match self {
            Provider::EncoreCloud(exp) => exp,
            Provider::Gcp(exp) => exp,
            Provider::Aws(_exp) => todo!(),
            Provider::PromRemoteWrite(_exp) => todo!(),
            Provider::Datadog(_exp) => todo!(),
        }
    }
}
struct AwsCloudWatch {}
struct PrometheusRemoteWrite {}
struct Datadog {}

pub struct Manager {
    exporter: Box<dyn Exporter>,
    registry: Registry,
}

pub trait Exporter {
    fn export(&self);
}
