use std::collections::HashMap;
use std::error::Error;
use std::time::Duration;
use std::env;
use tonic::Request;
use tokio::time::sleep;
use tracing::{info, warn, error};
use tracing_subscriber;

pub mod cloud_guardian {
    tonic::include_proto!("cloudguardian");
}

pub mod parser;
pub mod graph;

use cloud_guardian::InfrastructureResource;
use cloud_guardian::scanner_service_client::ScannerServiceClient;
use graph::DependencyGraph;

async fn connect_with_retry(addr: String, max_retries: u32) -> Result<ScannerServiceClient<tonic::transport::Channel>, Box<dyn Error>> {
    let mut retry_count = 0;
    let mut delay = Duration::from_secs(2);

    loop {
        match ScannerServiceClient::connect(addr.clone()).await {
            Ok(client) => {
                info!("✅ Połączono z serwerem gRPC pod adresem: {}", addr);
                return Ok(client);
            }
            Err(e) => {
                retry_count += 1;
                if retry_count > max_retries {
                    error!("❌ Nie udało się połączyć po {} próbach: {}", max_retries, e);
                    return Err(Box::new(e));
                }
                warn!("⚠️ Próba połączenia {}/{} nieudana: {}. Ponawiam za {:?}...", retry_count, max_retries, e, delay);
                sleep(delay).await;
                delay *= 2; // Wykładniczy backoff
            }
        }
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    tracing_subscriber::fmt::init();
    info!("🚀 Startujemy Agenta CloudGuardian...");

    // [AC 3] Ścieżka do folderu jako argument CLI
    let args: Vec<String> = env::args().collect();
    let scan_path = if args.len() > 1 {
        &args[1]
    } else {
        "."
    };

    info!("🔍 Skanowanie folderu: {}", scan_path);

    // [AC 1] Pipeline: Skanuj folder -> Buduj Graf
    let resources = parser::parse_terraform_dir(scan_path);
    info!("📦 Znaleziono {} zasobów w plikach .tf", resources.len());

    let mut graph = DependencyGraph::new();
    for res in resources {
        graph.add_resource(res);
    }

    // [AC 2] Inicjalizacja połączenia gRPC
    let server_addr = env::var("SERVER_ADDR").unwrap_or_else(|_| "http://[::1]:50051".to_string());
    let mut client = connect_with_retry(server_addr, 5).await?;

    // [AC 1] Wyślij każdy zasób przez gRPC (z zachowaniem kolejności topologicznej)
    let sorted_ids = graph.topological_sort();
    info!("📤 Rozpoczynanie wysyłania zasobów do serwera...");

    for resource_id in sorted_ids {
        if let Some(resource) = graph.resources.get(&resource_id) {
            info!("📡 Wysyłanie zasobu: {}", resource_id);
            
            let request = Request::new(resource.clone());
            match client.report_resource(request).await {
                Ok(response) => {
                    info!("✅ Serwer potwierdził zasób {}: {}", resource_id, response.into_inner().message);
                }
                Err(e) => {
                    error!("❌ Błąd podczas wysyłania zasobu {}: {}", resource_id, e);
                }
            }
            // Mała przerwa między wysyłaniem
            sleep(Duration::from_millis(100)).await;
        }
    }

    info!("🏁 Zakończono skanowanie i wysyłanie danych.");
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use tonic::transport::Server;
    use cloud_guardian::scanner_service_server::{ScannerService, ScannerServiceServer};
    use cloud_guardian::{InfrastructureResource, PiiResult, ScanResponse};
    use tonic::{Response, Status};

    struct MockScannerService;

    #[tonic::async_trait]
    impl ScannerService for MockScannerService {
        async fn report_resource(&self, _request: Request<InfrastructureResource>) -> Result<Response<ScanResponse>, Status> {
            Ok(Response::new(ScanResponse {
                success: true,
                message: "Mock success".into(),
            }))
        }
        async fn report_pii_finding(&self, _request: Request<PiiResult>) -> Result<Response<ScanResponse>, Status> {
            Ok(Response::new(ScanResponse {
                success: true,
                message: "Mock PII success".into(),
            }))
        }
    }

    #[tokio::test]
    async fn test_connect_with_retry_eventual_success() {
        let addr = "http://127.0.0.1:50052";
        let server_addr = "127.0.0.1:50052".parse().unwrap();

        tokio::spawn(async move {
            sleep(Duration::from_secs(3)).await;
            let scanner = MockScannerService;
            Server::builder()
                .add_service(ScannerServiceServer::new(scanner))
                .serve(server_addr)
                .await
                .unwrap();
        });

        let result = connect_with_retry(addr.to_string(), 5).await;
        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_connect_with_retry_failure() {
        let addr = "http://127.0.0.1:50053";
        let result = connect_with_retry(addr.to_string(), 2).await;
        assert!(result.is_err());
    }
}
