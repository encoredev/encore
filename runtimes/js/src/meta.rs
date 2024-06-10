use encore_runtime_core::meta;
use napi_derive::napi;

#[napi(object)]
#[derive(Debug, Clone)]
pub struct AppMeta {
    pub app_id: String,
    pub api_base_url: String,
    pub environment: EnvironmentMeta,
    pub build: BuildMeta,
    pub deploy: DeployMeta,
}

impl From<meta::AppMeta> for AppMeta {
    fn from(rt: meta::AppMeta) -> Self {
        AppMeta {
            app_id: rt.app_id,
            api_base_url: rt.api_base_url,
            environment: rt.environment.into(),
            build: rt.build.into(),
            deploy: rt.deploy.into(),
        }
    }
}

#[napi(object)]
#[derive(Debug, Clone)]
pub struct EnvironmentMeta {
    pub name: String,
    pub r#type: EnvironmentType,
    pub cloud: CloudProvider,
}

impl From<meta::EnvironmentMeta> for EnvironmentMeta {
    fn from(rt: meta::EnvironmentMeta) -> Self {
        EnvironmentMeta {
            name: rt.name,
            r#type: rt.r#type.into(),
            cloud: rt.cloud.into(),
        }
    }
}

#[napi]
#[derive(Debug)]
pub enum EnvironmentType {
    Production,
    Development,
    Ephemeral,
    Test,
}

impl From<meta::EnvironmentType> for EnvironmentType {
    fn from(rt: meta::EnvironmentType) -> Self {
        match rt {
            meta::EnvironmentType::Production => Self::Production,
            meta::EnvironmentType::Development => Self::Development,
            meta::EnvironmentType::Ephemeral => Self::Ephemeral,
            meta::EnvironmentType::Test => Self::Test,
        }
    }
}

#[napi]
#[derive(Debug)]
#[allow(clippy::upper_case_acronyms)]
pub enum CloudProvider {
    AWS,
    GCP,
    Azure,
    Encore,
    Local,
}

impl From<meta::CloudProvider> for CloudProvider {
    fn from(rt: meta::CloudProvider) -> Self {
        match rt {
            meta::CloudProvider::AWS => Self::AWS,
            meta::CloudProvider::GCP => Self::GCP,
            meta::CloudProvider::Azure => Self::Azure,
            meta::CloudProvider::Encore => Self::Encore,
            meta::CloudProvider::Local => Self::Local,
        }
    }
}

#[napi(object)]
#[derive(Debug, Clone)]
pub struct BuildMeta {
    pub revision: String,
    pub uncommitted_changes: bool,
}

impl From<meta::BuildMeta> for BuildMeta {
    fn from(rt: meta::BuildMeta) -> Self {
        BuildMeta {
            revision: rt.revision,
            uncommitted_changes: rt.uncommitted_changes,
        }
    }
}

#[napi(object)]
#[derive(Debug, Clone)]
pub struct DeployMeta {
    pub id: String,
    pub deploy_time: String,
}

impl From<meta::DeployMeta> for DeployMeta {
    fn from(rt: meta::DeployMeta) -> Self {
        DeployMeta {
            id: rt.id,
            deploy_time: rt.deploy_time.to_string(),
        }
    }
}
