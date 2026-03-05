use std::collections::HashMap;
use std::fs;
use std::path::Path;
use crate::cloud_guardian::InfrastructureResource;
use hcl::{Body, Block, Expression};
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
    let mut dependencies = Vec::new();

    // Check direct attributes
    for attr in block.body().attributes() {
        let expr = attr.expr();
        
        // Find dependencies in the expression
        find_traversals(expr, &mut dependencies);

        match expr {
            Expression::Bool(v) => {
                if attr.key() == "publicly_accessible" {
                    is_public = *v;
                }
            }
            Expression::String(v) => {
                if attr.key() == "acl" {
                    if v.contains("public") {
                        is_public = true;
                    }
                }
            }
            Expression::Object(obj) => {
                if attr.key() == "tags" {
                    for (k, v) in obj {
                        let key_str = k.to_string();
                        if let Expression::String(val_str) = v {
                            tags.insert(key_str, val_str.clone());
                        }
                        find_traversals(v, &mut dependencies);
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
        dependencies,
    })
}

/// Recursively find resource traversals (references) in an HCL expression
fn find_traversals(expr: &Expression, dependencies: &mut Vec<String>) {
    match expr {
        Expression::Traversal(traversal) => {
            // A resource ref like `aws_vpc.main.id`
            // traversal.expr is the base (e.g. Variable("aws_vpc"))
            // traversal.operators has the rest (e.g. [GetAttr("main"), GetAttr("id")])
            let base = traversal.expr.to_string();
            let mut parts = vec![base];
            
            for op in &traversal.operators {
                match op {
                    hcl::expr::TraversalOperator::GetAttr(attr) => {
                        parts.push(attr.to_string());
                    }
                    hcl::expr::TraversalOperator::Index(idx_expr) => {
                        parts.push(idx_expr.to_string());
                    }
                    _ => {}
                }
            }

            if parts.len() >= 2 {
                let resource_ref = format!("{}.{}", parts[0], parts[1]);
                if !dependencies.contains(&resource_ref) {
                    dependencies.push(resource_ref);
                }
            }
        }
        Expression::Array(arr) => {
            for e in arr {
                find_traversals(e, dependencies);
            }
        }
        Expression::Object(obj) => {
            for (_, e) in obj {
                find_traversals(e, dependencies);
            }
        }
        _ => {}
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_mock_file() {
        let mut path = Path::new("tests/terraform_mock.tf");
        if !path.exists() {
            path = Path::new("agent/tests/terraform_mock.tf");
        }
        
        let resources = parse_terraform_file(path);
        assert_eq!(resources.len(), 3);
    }

    #[test]
    fn test_extract_dependencies() {
        let hcl = r#"
            resource "aws_security_group" "web_sg" {
                vpc_id = aws_vpc.main.id
            }
            resource "aws_vpc" "main" {
                cidr_block = "10.0.0.0/16"
            }
        "#;
        let body: Body = hcl::from_str(hcl).unwrap();
        let blocks: Vec<Block> = body.into_blocks().into_iter().filter(|b| b.identifier() == "resource").collect();
        
        let sg_res = extract_resource(&blocks[0]).unwrap();
        assert_eq!(sg_res.resource_id, "aws_security_group.web_sg");
        assert!(sg_res.dependencies.contains(&"aws_vpc.main".to_string()));
    }
}
