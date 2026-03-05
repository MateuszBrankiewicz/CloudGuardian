use std::collections::HashMap;
use std::error::Error;
use std::time::Duration;
use tonic::Request;
use tokio::time::sleep;
use tracing::{info, warn, error};
use tracing_subscriber;

pub mod cloud_guardian {
    tonic::include_proto!("cloudguardian");
}

use cloud_guardian::InfrastructureResource;
use cloud_guardian::scanner_service_client::ScannerServiceClient;

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
    info!("🚀 Startujemy Agenta...");

    let mut client = connect_with_retry("http://[::1]:50051".to_string(), 5).await?;

    let mut metadata_map = HashMap::new();
    metadata_map.insert("env".to_string(), "production".to_string());
    metadata_map.insert("region".to_string(), "eu-central-1".to_string());

    let request = Request::new(InfrastructureResource {
        resource_id: "s3-prod.data".into(),
        provider: "aws".into(),
        r#type: "aws_s3_bucket".into(),
        metadata: metadata_map,
        estimated_cost: 150.50,
        tags: HashMap::new(),
        is_public: false,
    });

    info!("📤 Wysyłanie raportu...");

    let response = client.report_resource(request).await?;

    info!("✅ Odpowiedź: {}", response.into_inner().message);

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

        // Uruchamiamy serwer z opóźnieniem w tle
        tokio::spawn(async move {
            sleep(Duration::from_secs(3)).await;
            let scanner = MockScannerService;
            Server::builder()
                .add_service(ScannerServiceServer::new(scanner))
                .serve(server_addr)
                .await
                .unwrap();
        });

        // Próba połączenia powinna się udać po kilku próbach
        let result = connect_with_retry(addr.to_string(), 5).await;
        assert!(result.is_ok(), "Połączenie powinno się udać po restarcie serwera");
    }

    #[tokio::test]
    async fn test_connect_with_retry_failure() {
        let addr = "http://127.0.0.1:50053"; // Nikt tu nie słucha
        let result = connect_with_retry(addr.to_string(), 2).await;
        assert!(result.is_err(), "Połączenie powinno zwrócić błąd po wyczerpaniu prób");
    }
}
