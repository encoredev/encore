use serde::Serialize;

use crate::encore::parser::meta::v1 as meta;
use crate::encore::runtime::v1 as rt;

#[derive(Debug, Clone, Default, Serialize)]
pub struct AppMeta {
    /// The Encore application ID. If the application is not linked to the Encore platform this will be an empty string.
    /// To link to the Encore platform run `encore app link` from your terminal in the root directory of the Encore app.
    pub app_id: String,

    /// The base URL which can be used to call the API of this running application.
    ///
    /// For local development it is "http://localhost:<port>", typically "http://localhost:4000".
    ///
    /// If a custom domain is used for this environment it is returned here, but note that
    /// changes only take effect at the time of deployment while custom domains can be updated at any time.
    pub api_base_url: String,

    /// Information about the environment the app is running in.
    pub environment: EnvironmentMeta,

    /// Information about the build.
    pub build: BuildMeta,

    /// Information about the deployment.
    pub deploy: DeployMeta,
}

impl AppMeta {
    pub fn new(rt: &rt::RuntimeConfig, md: &meta::Data) -> AppMeta {
        let env = rt.environment.as_ref();

        let app_id = env.map(|e| e.app_id.clone()).unwrap_or_default();

        let api_base_url = rt
            .infra
            .as_ref()
            .and_then(|infra| infra.resources.as_ref())
            .and_then(|res| res.gateways.first())
            .map(|gw| gw.base_url.clone())
            .unwrap_or_default();

        AppMeta {
            app_id,
            api_base_url,
            environment: env.map_or_else(EnvironmentMeta::default, EnvironmentMeta::from),
            build: BuildMeta::from(md),
            deploy: rt
                .deployment
                .as_ref()
                .map_or_else(DeployMeta::default, DeployMeta::from),
        }
    }
}

#[derive(Debug, Clone, Default, Serialize)]
pub struct EnvironmentMeta {
    /// The name of environment that this application.
    /// For local development it is "local".
    pub name: String,

    /// The type of environment is this application running in.
    /// For local development it is "development".
    pub r#type: EnvironmentType,

    /// The cloud this is running in.
    /// For local development it is "local".
    pub cloud: CloudProvider,
}

impl From<&rt::Environment> for EnvironmentMeta {
    fn from(env: &rt::Environment) -> Self {
        let env_type = rt::environment::Type::try_from(env.env_type)
            .unwrap_or(rt::environment::Type::Unspecified);
        let cloud = rt::environment::Cloud::try_from(env.cloud)
            .unwrap_or(rt::environment::Cloud::Unspecified);
        EnvironmentMeta {
            name: env.env_name.clone(),
            r#type: EnvironmentType::from(&env_type),
            cloud: CloudProvider::from(&cloud),
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "lowercase")]
#[derive(Default)]
pub enum EnvironmentType {
    // A production environment.
    Production,
    // A long-lived cloud-hosted, non-production environment, such as test environments, or local development.
    #[default]
    Development,
    // A short-lived cloud-hosted, non-production environments, such as preview environments
    Ephemeral,
    // When running automated tests.
    Test,
}

impl From<&rt::environment::Type> for EnvironmentType {
    fn from(env: &rt::environment::Type) -> Self {
        use rt::environment::Type::*;
        match env {
            Production => Self::Production,
            Ephemeral => Self::Ephemeral,
            Test => Self::Test,
            Development | Unspecified => Self::Development,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "lowercase")]
#[derive(Default)]
pub enum CloudProvider {
    AWS,
    GCP,
    Azure,
    Encore,
    #[default]
    Local,
}

impl From<&rt::environment::Cloud> for CloudProvider {
    fn from(cloud: &rt::environment::Cloud) -> Self {
        use rt::environment::Cloud::*;
        match cloud {
            Aws => Self::AWS,
            Gcp => Self::GCP,
            Azure => Self::Azure,
            Encore => Self::Encore,
            Local | Unspecified => Self::Local,
        }
    }
}

#[derive(Debug, Clone, Default, Serialize)]
pub struct BuildMeta {
    // The git commit that formed the base of this build.
    pub revision: String,
    // Whether there were uncommitted changes on top of the commit.
    pub uncommitted_changes: bool,
}

impl From<&meta::Data> for BuildMeta {
    fn from(md: &meta::Data) -> Self {
        BuildMeta {
            revision: md.app_revision.clone(),
            uncommitted_changes: md.uncommitted_changes,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
pub struct HostedService {
    // The name of the service
    pub name: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct DeployMeta {
    // The unique id of the deployment. Generated by the Encore Platform.
    pub id: String,
    // The time the deployment was made.
    pub deploy_time: chrono::DateTime<chrono::Utc>,
    // The services hosted by this deployment.
    pub hosted_services: Vec<HostedService>,
}

impl From<&rt::Deployment> for DeployMeta {
    fn from(rt: &rt::Deployment) -> Self {
        DeployMeta {
            id: rt.deploy_id.clone(),
            deploy_time: rt
                .deployed_at
                .as_ref()
                .and_then(|d| chrono::DateTime::from_timestamp(d.seconds, d.nanos as u32))
                .unwrap_or_else(chrono::Utc::now),
            hosted_services: rt
                .hosted_services
                .iter()
                .map(|s| HostedService {
                    name: s.name.clone(),
                })
                .collect(),
        }
    }
}

impl Default for DeployMeta {
    fn default() -> Self {
        DeployMeta {
            id: "".into(),
            deploy_time: chrono::Utc::now(),
            hosted_services: vec![],
        }
    }
}
