use std::fmt::{Display, Formatter};

use swc_common::sync::Lrc;

use crate::parser::resourceparser::resource_parser::ResourceParser;
use crate::parser::resources::apis::api::ENDPOINT_PARSER;
use crate::parser::resources::apis::authhandler::AUTHHANDLER_PARSER;
use crate::parser::resources::apis::gateway::GATEWAY_PARSER;
use crate::parser::resources::apis::service::SERVICE_PARSER;
use crate::parser::resources::infra::cron::CRON_PARSER;
use crate::parser::resources::infra::objects::OBJECTS_PARSER;
use crate::parser::resources::infra::pubsub_subscription::SUBSCRIPTION_PARSER;
use crate::parser::resources::infra::pubsub_topic::TOPIC_PARSER;
use crate::parser::resources::infra::secret::SECRET_PARSER;
use crate::parser::resources::infra::sqldb::SQLDB_PARSER;

pub mod apis;
pub mod infra;
mod parseutil;

#[derive(Debug, Clone)]
pub enum Resource {
    ServiceClient(Lrc<apis::service_client::ServiceClient>),
    APIEndpoint(Lrc<apis::api::Endpoint>),
    AuthHandler(Lrc<apis::authhandler::AuthHandler>),
    Gateway(Lrc<apis::gateway::Gateway>),
    Service(Lrc<apis::service::Service>),
    SQLDatabase(Lrc<infra::sqldb::SQLDatabase>),
    Bucket(Lrc<infra::objects::Bucket>),
    PubSubTopic(Lrc<infra::pubsub_topic::Topic>),
    PubSubSubscription(Lrc<infra::pubsub_subscription::Subscription>),
    CronJob(Lrc<infra::cron::CronJob>),
    Secret(Lrc<infra::secret::Secret>),
}

#[derive(Debug, Eq, Hash, PartialEq, Clone)]
pub enum ResourcePath {
    SQLDatabase { name: String },
    Bucket { name: String },
}

impl Display for Resource {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Resource::ServiceClient(client) => write!(f, "ServiceClient({})", client.service_name),
            Resource::APIEndpoint(api) => {
                write!(f, "APIEndpoint({}::{})", api.service_name, api.name)
            }
            Resource::AuthHandler(handler) => {
                write!(f, "AuthHandler({}::{})", handler.service_name, handler.name)
            }
            Resource::Gateway(gw) => {
                write!(f, "Gateway({})", gw.name)
            }
            Resource::SQLDatabase(db) => write!(f, "SQLDatabase({})", db.name),
            Resource::Bucket(db) => write!(f, "Bucket({})", db.name),
            Resource::PubSubTopic(topic) => write!(f, "PubSubTopic({})", topic.name),
            Resource::PubSubSubscription(sub) => write!(f, "PubSubSubscription({})", sub.name),
            Resource::CronJob(cron) => write!(f, "CronJob({})", cron.name),
            Resource::Secret(secret) => write!(f, "Secret({})", secret.name),
            Resource::Service(svc) => write!(f, "Service({})", svc.name),
        }
    }
}

pub static DEFAULT_RESOURCE_PARSERS: &[&ResourceParser] = &[
    // The service parser must come first, as other resources may depend on
    // knowing which service they belong to.
    &SERVICE_PARSER,
    &ENDPOINT_PARSER,
    &AUTHHANDLER_PARSER,
    &GATEWAY_PARSER,
    &SQLDB_PARSER,
    &OBJECTS_PARSER,
    &TOPIC_PARSER,
    &SUBSCRIPTION_PARSER,
    &CRON_PARSER,
    &SECRET_PARSER,
];
