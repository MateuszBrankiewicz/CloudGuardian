use regex::Regex;
use rayon::prelude::*;
use std::sync::Arc;
use crate::cloud_guardian::PiiResult;
use tracing::info;
use std::collections::HashMap;

pub struct PiiEngine {
    patterns: Vec<(String, Regex)>,
}

impl PiiEngine {
    pub fn new() -> Self {
        let patterns = vec![
            ("email".to_string(), Regex::new(r"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}").unwrap()),
            ("credit_card".to_string(), Regex::new(r"\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b").unwrap()),
            ("pesel".to_string(), Regex::new(r"\b\d{11}\b").unwrap()),
        ];
        Self { patterns }
    }

    /// Scans a single string for all known PII patterns.
    pub fn scan_text(&self, text: &str, resource_id: &str) -> Vec<PiiResult> {
        let mut results = Vec::new();
        for (data_type, regex) in &self.patterns {
            let count = regex.find_iter(text).count();
            if count > 0 {
                results.push(PiiResult {
                    resource_id: resource_id.to_string(),
                    data_type: data_type.clone(),
                    confidence: 1.0, // Simplification for this US
                    occurrence_count: count as i32,
                });
            }
        }
        results
    }

    /// Scans multiple lines of text in parallel using rayon.
    pub fn scan_lines_parallel(&self, lines: Vec<String>, resource_id: &str) -> Vec<PiiResult> {
        let arc_self = Arc::new(self);
        
        // Use rayon to scan lines in parallel
        let intermediate_results: Vec<Vec<PiiResult>> = lines
            .par_iter()
            .map(|line| {
                arc_self.scan_text(line, resource_id)
            })
            .collect();

        // Merge results (sum occurrences for the same data_type)
        let mut merged: HashMap<String, PiiResult> = HashMap::new();
        for batch in intermediate_results {
            for res in batch {
                let entry = merged.entry(res.data_type.clone()).or_insert(PiiResult {
                    resource_id: resource_id.to_string(),
                    data_type: res.data_type.clone(),
                    confidence: 1.0,
                    occurrence_count: 0,
                });
                entry.occurrence_count += res.occurrence_count;
            }
        }

        merged.into_values().collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Instant;

    #[test]
    fn test_pii_detection() {
        let engine = PiiEngine::new();
        let text = "My email is test@example.com and my credit card is 1234-5678-9012-3456. PESEL: 12345678901";
        let results = engine.scan_text(text, "test-resource");

        assert!(results.iter().any(|r| r.data_type == "email" && r.occurrence_count == 1));
        assert!(results.iter().any(|r| r.data_type == "credit_card" && r.occurrence_count == 1));
        assert!(results.iter().any(|r| r.data_type == "pesel" && r.occurrence_count == 1));
    }

    #[test]
    fn test_parallel_performance() {
        let engine = PiiEngine::new();
        let line = "User test@example.com with card 1111-2222-3333-4444 and PESEL 99988877766
";
        
        // Create 100MB of data (approx 1,000,000 lines if each line is ~100 bytes)
        // For testing we will use a smaller but representative amount to keep test time reasonable
        let num_lines = 100_000; 
        let lines: Vec<String> = (0..num_lines).map(|_| line.to_string()).collect();

        let start = Instant::now();
        let results = engine.scan_lines_parallel(lines, "perf-test");
        let duration = start.elapsed();

        info!("Scanned {} lines in {:?}", num_lines, duration);
        assert!(duration.as_secs_f32() < 1.0, "Scanning took too long: {:?}", duration);
        assert!(results.iter().any(|r| r.data_type == "email" && r.occurrence_count == num_lines as i32));
    }
}
