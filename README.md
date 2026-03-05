# 🛡️ CloudGuardian

**CloudGuardian** to zaawansowany system typu **Security & FinOps Hybrid**, który łączy analizę statyczną infrastruktury (IaC), wielowątkowe skanowanie danych wrażliwych (PII) oraz sztuczną inteligencję (Ollama) działającą lokalnie do generowania poprawek bezpieczeństwa.

## 🚀 Architektura Systemu

System składa się z trzech głównych komponentów:

1.  **Agent (Rust)**: Ultra-szybki skaner napisany w Rust.
    *   Skanuje pliki Terraform (`.tf`) i buduje graf zależności zasobów.
    *   Wielowątkowy silnik regex (`rayon`) do wykrywania PII (E-mail, Karty Kredytowe, PESEL).
    *   Implementacja algorytmów Luhna i sum kontrolnych dla minimalizacji False Positives.
    *   Komunikacja gRPC z serwerem.
2.  **Server (Go)**: Centralny mózg systemu.
    *   Odbiera dane przez gRPC i zapisuje je w PostgreSQL.
    *   Integruje się z lokalnym API **Ollama** do analizy ryzyka.
    *   Udostępnia REST API dla Dashboardu.
3.  **Command Center (Next.js)**: Interaktywny dashboard.
    *   Wizualizacja grafu infrastruktury (`React Flow`).
    *   Monitoring kosztów i incydentów bezpieczeństwa w czasie rzeczywistym.
    *   Remediacja AI: generowanie gotowych snippetów Terraform naprawiających luki.

## 🛠️ Szybki Start

### 1. Wymagania
*   Docker & Docker Compose
*   Rust (wersja 1.75+)
*   Go (wersja 1.21+)
*   Node.js (wersja 18+)

### 2. Uruchomienie Infrastruktury
```bash
docker-compose up -d
```

### 3. Przygotowanie AI (Ollama)
Pobierz model, który będzie analizował Twoją infrastrukturę lokalnie:
```bash
docker exec -it cloudguardian-ollama-1 ollama pull llama3:latest
```

### 4. Uruchomienie Serwera (Go)
```bash
cd server
go run .
```
Serwer uruchomi gRPC na porcie `:50051` oraz REST API na porcie `:8080`.

### 5. Uruchomienie Dashboardu (Next.js)
```bash
cd frontend
npm install
npm run dev
```
Otwórz [http://localhost:3000](http://localhost:3000).

### 6. Uruchomienie Agenta (Rust)
Przeskanuj swoją infrastrukturę:
```bash
cd agent
cargo run -- ../stress_test/
```

## 🔒 Bezpieczeństwo i Prywatność
CloudGuardian został zaprojektowany z myślą o najwyższych standardach prywatności:
*   **Lokalne LLM**: Analiza danych przez AI odbywa się w Twojej sieci (Ollama), żadne dane nie opuszczają serwera.
*   **Statyczna Analiza**: Skanujemy kod IaC przed wdrożeniem, zapobiegając błędom typu *Public S3 Bucket*.
*   **Data Sampling**: Agent skanuje próbki danych, co pozwala na błyskawiczną analizę nawet terabajtowych dumpów baz danych.

## 📊 Testy Obciążeniowe
System zawiera wbudowane skrypty do generowania ogromnych zbiorów danych (5000+ zasobów, miliony rekordów PII), aby zweryfikować wydajność silnika napisanego w Rust.

---
*Projekt stworzony w ramach demonstracji potęgi połączenia Rust, Go i lokalnych modeli AI.*
