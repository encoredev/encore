use crate::encore::runtime::v1 as pb;
use crate::objects;

#[derive(Debug)]
pub struct Bucket {
    client: Box<s3::Bucket>,
}

impl Bucket {
    pub(super) fn new(region: s3::Region, creds: s3::creds::Credentials, cfg: &pb::Bucket) -> Self {
        let client = s3::Bucket::new(&cfg.cloud_name, region, creds).expect("unable to construct bucket client");
        Self {
            client,
        }
    }
}

impl objects::Bucket for Bucket {
}
