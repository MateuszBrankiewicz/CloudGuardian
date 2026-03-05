use crate::cloud_guardian::InfrastructureResource;
use hcl::{Block, Body, Expression};
use std::collections::HashMap;
use std::fs;
use std::path::Path;
use tracing::warn;

pub fn parse_terraform_dir<P: AsRef<Path>>(dir: P) -> Vec<InfrastructureResource> {
    let mut resources = Vec::new();

    if let Ok(entries) = fs::read_dir(dir) {
        for entry in entries.flatten() {
            let path = entry.path();
            if path.extension().and_then(|s| s.to_str()) == Some("tf") {
                resources.extend(parse_terraform_file(&path));
            }
        }
    }

    resources
}

pub fn parse_terraform_file<P: AsRef<Path>>(path: P) -> Vec<InfrastructureResource> {
    let mut resources = Vec::new();
    let content = match fs::read_to_string(&path) {
        Ok(c) => c,
        Err(e) => {
            warn!("Failed to read file {:?}: {}", path.as_ref(), e);
            return resources;
        }
    };

    let body: Body = match hcl::from_str(&content) {
        Ok(b) => b,
        Err(e) => {
            warn!("Failed to parse HCL file {:?}: {}", path.as_ref(), e);
            return resources;
        }
    };

    for block in body.into_blocks() {
        if block.identifier() == "resource" {
            if let Some(resource) = extract_resource(&block) {
                resources.push(resource);
            }
        }
    }

    resources
}

fn extract_resource(block: &Block) -> Option<InfrastructureResource> {
    let labels: Vec<&str> = block.labels().iter().map(|l| l.as_str()).collect();
    if labels.len() < 2 {
        return None;
    }

    let resource_type = labels[0];
    let resource_name = labels[1];
    let provider = resource_type.split('_').next().unwrap_or("unknown");

    let mut is_public = false;
    let mut tags = HashMap::new();

    // Check direct attributes
    for attr in block.body().attributes() {
        match attr.expr() {
            Expression::Bool(v) => {
                if attr.key() == "publicly_accessible" {
                    is_public = *v;
                }
            }
            Expression::String(v) => match attr.key() == "acl" {
                true => match v.contains("public") {
                    true => {
                        is_public = true;
                    }
                    false => (),
                },
                false => (),
            },
            Expression::Object(obj) => {
                if attr.key() == "tags" {
                    for (k, v) in obj {
                        let key_str = k.to_string();
                        if let Expression::String(val_str) = v {
                            tags.insert(key_str, val_str.clone());
                        }
                    }
                }
            }
            _ => {}
        }
    }

    Some(InfrastructureResource {
        resource_id: format!("{}.{}", resource_type, resource_name),
        provider: provider.to_string(),
        r#type: resource_type.to_string(),
        metadata: HashMap::new(),
        estimated_cost: 0.0,
        tags,
        is_public,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_mock_file() {
        // Assume running from project root or agent/
        let mut path = Path::new("tests/terraform_mock.tf");
        if !path.exists() {
            path = Path::new("agent/tests/terraform_mock.tf");
        }

        let resources = parse_terraform_file(path);
        assert_eq!(resources.len(), 3);

        let s3 = resources
            .iter()
            .find(|r| r.resource_id == "aws_s3_bucket.prod_data")
            .expect("S3 not found");
        assert_eq!(s3.provider, "aws");
        assert_eq!(s3.r#type, "aws_s3_bucket");
        assert!(s3.is_public);
        assert_eq!(s3.tags.get("Environment"), Some(&"production".to_string()));

        let db = resources
            .iter()
            .find(|r| r.resource_id == "aws_db_instance.main_db")
            .expect("DB not found");
        assert_eq!(db.r#type, "aws_db_instance");
        assert!(!db.is_public);

        let private_s3 = resources
            .iter()
            .find(|r| r.resource_id == "aws_s3_bucket.private_logs")
            .expect("Private S3 not found");
        assert!(!private_s3.is_public);
    }
}
