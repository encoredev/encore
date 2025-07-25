use crate::{EncoreName, EndpointName};
use std::str::FromStr;

#[derive(Debug, Clone)]
pub enum Caller {
    APIEndpoint(EndpointName),
    PubSubMessage {
        topic: EncoreName,
        subscription: EncoreName,
        message_id: String,
    },
    App {
        deploy_id: String,
    },
    Gateway {
        /// The name of the gateway.
        gateway: EncoreName,
    },
    EncorePrincipal(String),
}

impl Caller {
    pub fn serialize(&self) -> String {
        match self {
            Caller::APIEndpoint(name) => format!("api:{}.{}", name.service(), name.endpoint()),
            Caller::PubSubMessage {
                topic,
                subscription,
                message_id,
            } => format!("pubsub:{topic}:{subscription}:{message_id}"),
            Caller::Gateway { gateway } => {
                format!("gateway:{gateway}")
            }
            Caller::App { deploy_id } => format!("app:{deploy_id}"),
            Caller::EncorePrincipal(name) => format!("encore:{name}"),
        }
    }

    /// Whether private APIs can be called
    pub fn private_api_access(&self) -> bool {
        use Caller::*;
        match self {
            APIEndpoint(_) | PubSubMessage { .. } | App { .. } | EncorePrincipal(_) => true,

            Gateway { .. } => false,
        }
    }

    pub fn is_gateway(&self) -> bool {
        matches!(&self, Caller::Gateway { .. })
    }
}

impl FromStr for Caller {
    type Err = anyhow::Error;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        fn parse(s: &str) -> Option<Caller> {
            let (kind, rest) = s.split_once(':')?;

            Some(match kind {
                "api" => {
                    let (service, endpoint) = rest.split_once('.')?;

                    Caller::APIEndpoint(EndpointName::new(service, endpoint))
                }
                "pubsub" => {
                    let mut parts = rest.splitn(3, ':');
                    let topic = parts.next()?;
                    let subscription = parts.next()?;
                    let message_id = parts.next()?;
                    Caller::PubSubMessage {
                        topic: EncoreName::from(topic),
                        subscription: EncoreName::from(subscription),
                        message_id: message_id.to_string(),
                    }
                }
                "app" => Caller::App {
                    deploy_id: rest.to_string(),
                },
                "gateway" => {
                    let mut parts = rest.splitn(2, '.');
                    let gateway = parts.next()?;
                    Caller::Gateway {
                        gateway: EncoreName::from(gateway),
                    }
                }
                "encore" => Caller::EncorePrincipal(rest.to_string()),
                _ => return None,
            })
        }

        parse(s).ok_or_else(|| anyhow::anyhow!("invalid caller string"))
    }
}
