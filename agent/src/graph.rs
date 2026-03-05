use std::collections::{HashMap, HashSet};
use crate::cloud_guardian::InfrastructureResource;

#[derive(Debug, Default)]
pub struct DependencyGraph {
    /// Mapping from resource_id to the resource data
    pub resources: HashMap<String, InfrastructureResource>,
    /// Adjacency list: resource_id -> set of resource_ids it depends on
    pub adj: HashMap<String, HashSet<String>>,
}

impl DependencyGraph {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn add_resource(&mut self, resource: InfrastructureResource) {
        let resource_id = resource.resource_id.clone();
        for dep in &resource.dependencies {
            self.adj
                .entry(resource_id.clone())
                .or_default()
                .insert(dep.clone());
        }
        self.resources.insert(resource_id, resource);
    }

    pub fn get_dependencies(&self, resource_id: &str) -> Option<&HashSet<String>> {
        self.adj.get(resource_id)
    }

    pub fn has_dependency(&self, from: &str, to: &str) -> bool {
        self.adj.get(from).map_or(false, |deps| deps.contains(to))
    }

    /// Returns a list of all resources in a valid creation order (simple topological sort)
    /// This is a bonus to show the power of the graph.
    pub fn topological_sort(&self) -> Vec<String> {
        let mut result = Vec::new();
        let mut visited = HashSet::new();
        let mut temp_visited = HashSet::new();

        for node in self.resources.keys() {
            if !visited.contains(node) {
                self.topo_visit(node, &mut visited, &mut temp_visited, &mut result);
            }
        }

        result
    }

    fn topo_visit(
        &self,
        node: &String,
        visited: &mut HashSet<String>,
        temp_visited: &mut HashSet<String>,
        result: &mut Vec<String>,
    ) {
        if temp_visited.contains(node) {
            // Cycle detected - in real IaC this would be an error
            return;
        }
        if !visited.contains(node) {
            temp_visited.insert(node.clone());
            if let Some(deps) = self.adj.get(node) {
                for dep in deps {
                    // Only visit if the dependency is also a resource we manage
                    if self.resources.contains_key(dep) {
                        self.topo_visit(dep, visited, temp_visited, result);
                    }
                }
            }
            temp_visited.remove(node);
            visited.insert(node.clone());
            result.push(node.clone());
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn create_test_resource(id: &str, deps: Vec<String>) -> InfrastructureResource {
        InfrastructureResource {
            resource_id: id.into(),
            provider: "aws".into(),
            r#type: "resource_type".into(),
            metadata: HashMap::new(),
            estimated_cost: 0.0,
            tags: HashMap::new(),
            is_public: false,
            dependencies: deps,
        }
    }

    #[test]
    fn test_graph_dependencies() {
        let mut graph = DependencyGraph::new();

        let vpc = create_test_resource("aws_vpc.main", vec![]);
        let sg = create_test_resource("aws_security_group.web", vec!["aws_vpc.main".into()]);

        graph.add_resource(vpc);
        graph.add_resource(sg);

        assert!(graph.has_dependency("aws_security_group.web", "aws_vpc.main"));
        assert!(!graph.has_dependency("aws_vpc.main", "aws_security_group.web"));

        let order = graph.topological_sort();
        assert_eq!(order[0], "aws_vpc.main");
        assert_eq!(order[1], "aws_security_group.web");
    }
}
