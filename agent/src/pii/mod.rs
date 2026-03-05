use regex::Regex;
use rayon::prelude::*;
use std::sync::Arc;
use crate::cloud_guardian::PiiResult;
use std::collections::HashMap;
use std::fs::{self, File};
use std::io::{BufRead, BufReader};
use std::path::Path;
use tracing::{info, warn};

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

    /// Scans a directory for PII using data sampling (e.g., first 1000 lines).
    pub fn scan_directory_for_pii<P: AsRef<Path>>(&self, dir: P, sample_lines: usize) -> Vec<PiiResult> {
        let mut all_results = Vec::new();
        self.visit_dirs_pii(dir.as_ref(), &mut all_results, sample_lines);
        all_results
    }

    fn visit_dirs_pii(&self, dir: &Path, all_results: &mut Vec<PiiResult>, sample_lines: usize) {
        if let Ok(entries) = fs::read_dir(dir) {
            for entry in entries.flatten() {
                let path = entry.path();
                if path.is_dir() {
                    self.visit_dirs_pii(&path, all_results, sample_lines);
                } else if path.is_file() {
                    let ext = path.extension().and_then(|s| s.to_str()).unwrap_or("");
                    if matches!(ext, "csv" | "sql" | "txt" | "json") {
                        info!("🔍 Pobieranie próbek z pliku: {:?}", path);
                        let results = self.scan_file_sampled(&path, sample_lines);
                        all_results.extend(results);
                    }
                }
            }
        }
    }

    fn scan_file_sampled<P: AsRef<Path>>(&self, path: P, sample_lines: usize) -> Vec<PiiResult> {
        let file = match File::open(&path) {
            Ok(f) => f,
            Err(e) => {
                warn!("Could not open file {:?}: {}", path.as_ref(), e);
                return Vec::new();
            }
        };

        let reader = BufReader::new(file);
        let lines: Vec<String> = reader.lines()
            .take(sample_lines)
            .filter_map(|l| l.ok())
            .collect();

        let resource_id = path.as_ref().to_string_lossy().to_string();
        self.scan_lines_parallel(lines, &resource_id)
    }

    /// Scans a single string for all known PII patterns and validates them.
    pub fn scan_text(&self, text: &str, resource_id: &str) -> Vec<PiiResult> {
        let mut results = Vec::new();
        for (data_type, regex) in &self.patterns {
            let mut count = 0;
            for mat in regex.find_iter(text) {
                let matched_str = mat.as_str();
                
                if data_type == "credit_card" {
                    let digits: String = matched_str.chars().filter(|c| c.is_ascii_digit()).collect();
                    if is_luhn_valid(&digits) {
                        count += 1;
                    }
                } 
                else if data_type == "pesel" {
                    if is_pesel_valid(matched_str) {
                        count += 1;
                    }
                }
                else {
                    count += 1;
                }
            }

            if count > 0 {
                results.push(PiiResult {
                    resource_id: resource_id.to_string(),
                    data_type: data_type.clone(),
                    confidence: 1.0,
                    occurrence_count: count as i32,
                });
            }
        }
        results
    }

    /// Scans multiple lines of text in parallel using rayon.
    pub fn scan_lines_parallel(&self, lines: Vec<String>, resource_id: &str) -> Vec<PiiResult> {
        let arc_self = Arc::new(self);
        
        let intermediate_results: Vec<Vec<PiiResult>> = lines
            .par_iter()
            .map(|line| {
                arc_self.scan_text(line, resource_id)
            })
            .collect();

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

/// [AC 1] Luhn algorithm for credit card validation
fn is_luhn_valid(number: &str) -> bool {
    if number.is_empty() { return false; }
    let mut sum = 0;
    let mut double = false;
    for c in number.chars().rev() {
        if let Some(mut digit) = c.to_digit(10) {
            if double {
                digit *= 2;
                if digit > 9 {
                    digit -= 9;
                }
            }
            sum += digit;
            double = !double;
        } else {
            return false;
        }
    }
    sum % 10 == 0
}

/// [AC 2] PESEL validation
fn is_pesel_valid(pesel: &str) -> bool {
    if pesel.len() != 11 {
        return false;
    }

    let digits: Vec<u32> = pesel.chars().filter_map(|c| c.to_digit(10)).collect();
    if digits.len() != 11 {
        return false;
    }

    let weights = [1, 3, 7, 9, 1, 3, 7, 9, 1, 3];
    let mut sum = 0;
    for i in 0..10 {
        sum += digits[i] * weights[i];
    }

    let last_digit = (10 - (sum % 10)) % 10;
    last_digit == digits[10]
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_luhn_validation() {
        assert!(is_luhn_valid("79927398713")); 
        assert!(is_luhn_valid("49927398716"));
        assert!(is_luhn_valid("4242424242424242"));
        assert!(!is_luhn_valid("4242424242424241"));
    }

    #[test]
    fn test_pesel_validation() {
        assert!(is_pesel_valid("44051401359")); 
        assert!(is_pesel_valid("02070803628")); 
        assert!(!is_pesel_valid("44051401358")); 
        assert!(!is_pesel_valid("12345678901"));
    }

    #[test]
    fn test_pii_validation_integration() {
        let engine = PiiEngine::new();
        let text = "Card: 4242424242424242, Fake: 1111222233334441, PESEL: 44051401359";
        let results = engine.scan_text(text, "test-resource");

        let card_result = results.iter().find(|r| r.data_type == "credit_card").expect("Card PII not found");
        assert_eq!(card_result.occurrence_count, 1);

        let pesel_result = results.iter().find(|r| r.data_type == "pesel").expect("PESEL PII not found");
        assert_eq!(pesel_result.occurrence_count, 1);
    }
}
