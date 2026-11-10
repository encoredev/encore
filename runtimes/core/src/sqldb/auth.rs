use std::sync::{Arc, OnceLock};

use anyhow::Context;
use google_cloud_auth::credentials::{AccessTokenCredentials, Builder};

use crate::encore::runtime::v1 as pb;

/// The scope Cloud SQL accepts an access token under as a database password.
const GCP_IAM_SCOPE: &str = "https://www.googleapis.com/auth/sqlservice.login";

/// How a database's role authenticates to its server.
#[derive(Clone)]
pub enum Auth {
    /// With the role's password, if it has one, which is in the database's
    /// connection config already.
    Password,
    /// With an access token of the Google Cloud identity the process runs as,
    /// which Cloud SQL's IAM database authentication takes as the password.
    /// Each connection gets a current one: a token only has to be valid when
    /// a connection authenticates, and the connection outlives it.
    GcpIam(GcpIamTokens),
}

impl Auth {
    pub(crate) fn from_pb(method: pb::SqlAuthMethod) -> Self {
        match method {
            pb::SqlAuthMethod::Password => Auth::Password,
            pb::SqlAuthMethod::GcpIam => Auth::GcpIam(GcpIamTokens::process()),
        }
    }

    pub(crate) fn is_gcp_iam(&self) -> bool {
        matches!(self, Auth::GcpIam(_))
    }

    /// Returns the config to open a connection with: `base`, with the
    /// password set to a current token if the role logs in with one.
    pub(crate) async fn connect_config(
        &self,
        base: &Arc<tokio_postgres::Config>,
    ) -> anyhow::Result<Arc<tokio_postgres::Config>> {
        match self {
            Auth::Password => Ok(base.clone()),
            Auth::GcpIam(tokens) => {
                let token = tokens.token().await?;
                let mut config = (**base).clone();
                config.password(token);
                Ok(Arc::new(config))
            }
        }
    }
}

/// Access tokens for Cloud SQL IAM database authentication.
#[derive(Clone)]
pub struct GcpIamTokens {
    /// Built on first use, so that a process which never connects never looks
    /// for credentials. The credentials cache a token until shortly before it
    /// expires, so connections opened in the meantime share it.
    credentials: Arc<tokio::sync::OnceCell<AccessTokenCredentials>>,
}

impl GcpIamTokens {
    /// The tokens of the process's Application Default Credentials: on Cloud
    /// Run, its service account's, from the metadata server.
    fn process() -> Self {
        static TOKENS: OnceLock<GcpIamTokens> = OnceLock::new();
        TOKENS
            .get_or_init(|| GcpIamTokens {
                credentials: Arc::new(tokio::sync::OnceCell::new()),
            })
            .clone()
    }

    #[cfg(test)]
    pub(crate) fn with_credentials(credentials: AccessTokenCredentials) -> Self {
        GcpIamTokens {
            credentials: Arc::new(tokio::sync::OnceCell::new_with(Some(credentials))),
        }
    }

    async fn token(&self) -> anyhow::Result<String> {
        let credentials = self
            .credentials
            .get_or_try_init(|| async {
                Builder::default()
                    .with_scopes([GCP_IAM_SCOPE])
                    .build_access_token_credentials()
                    .context("failed to find Google Cloud credentials")
            })
            .await?;
        let token = credentials
            .access_token()
            .await
            .context("failed to get an access token")?;
        Ok(token.token)
    }
}

#[cfg(test)]
pub(crate) mod test {
    use std::sync::Mutex;

    use google_cloud_auth::credentials::{
        AccessToken, AccessTokenCredentialsProvider, CacheableResource, CredentialsProvider,
    };
    use google_cloud_auth::errors::CredentialsError;

    use super::*;

    /// Hands out the tokens it is given, one per call, and then fails.
    #[derive(Debug)]
    pub(crate) struct FakeTokens(Mutex<Vec<&'static str>>);

    impl FakeTokens {
        pub(crate) fn auth(tokens: &[&'static str]) -> Auth {
            let mut tokens = tokens.to_vec();
            tokens.reverse();
            let credentials = AccessTokenCredentials::from(FakeTokens(Mutex::new(tokens)));
            Auth::GcpIam(GcpIamTokens::with_credentials(credentials))
        }
    }

    impl CredentialsProvider for FakeTokens {
        async fn headers(
            &self,
            _: http::Extensions,
        ) -> Result<CacheableResource<http::HeaderMap>, CredentialsError> {
            unimplemented!("only access tokens are used")
        }

        async fn universe_domain(&self) -> Option<String> {
            None
        }
    }

    impl AccessTokenCredentialsProvider for FakeTokens {
        async fn access_token(&self) -> Result<AccessToken, CredentialsError> {
            match self.0.lock().unwrap().pop() {
                Some(token) => Ok(AccessToken {
                    token: token.to_owned(),
                }),
                None => Err(CredentialsError::from_msg(false, "no more tokens")),
            }
        }
    }

    fn base() -> Arc<tokio_postgres::Config> {
        let mut config = tokio_postgres::Config::new();
        config.host("10.0.0.3").user("my-sa@my-project.iam");
        Arc::new(config)
    }

    #[tokio::test]
    async fn password_auth_connects_with_the_config_as_is() {
        let base = base();
        let config = Auth::Password.connect_config(&base).await.unwrap();
        assert!(Arc::ptr_eq(&base, &config));
    }

    #[tokio::test]
    async fn gcp_iam_connects_each_time_with_a_current_token() {
        let auth = FakeTokens::auth(&["ya29.first", "ya29.second"]);
        let base = base();

        let first = auth.connect_config(&base).await.unwrap();
        let second = auth.connect_config(&base).await.unwrap();

        assert_eq!(first.get_password(), Some(&b"ya29.first"[..]));
        assert_eq!(second.get_password(), Some(&b"ya29.second"[..]));
        assert_eq!(first.get_user(), Some("my-sa@my-project.iam"));
        assert_eq!(base.get_password(), None, "the base config changed");
    }

    #[tokio::test]
    async fn gcp_iam_reports_a_token_it_could_not_get() {
        let err = FakeTokens::auth(&[])
            .connect_config(&base())
            .await
            .unwrap_err();
        let err = format!("{err:#}");
        assert!(err.contains("failed to get an access token"), "{err}");
        assert!(err.contains("no more tokens"), "{err}");
    }
}
