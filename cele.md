🏗️ 1. Architektura Systemu (System Design)
System składa się z trzech głównych komponentów:

Scanner Agent (Rust): Niskopoziomowy, ultra-szybki silnik do analizy plików IaC (Terraform) oraz skanowania strumieni danych w poszukiwaniu danych wrażliwych.

Control Plane (Go): Centralny serwer zarządzający logiką, kosztami chmury (API AWS/GCP) oraz orkiestracją zadań AI.

Insight Dashboard (React): Wizualny interfejs przedstawiający graf zasobów, ryzyka i rekomendacje kosztowe.

📅 Roadmapa i Backlog Zadań
Faza 1: Fundamenty i gRPC (Tydzień 1)
Cel: Uruchomienie szkieletu komunikacyjnego.

[ ] Zadanie 1.1: Konfiguracja Monorepo (struktura: /agent, /server, /ui, /proto).

[ ] Zadanie 1.2: Definicja cloudguardian.proto. Określenie usług przesyłania metryk, wyników skanowania PII i alertów.

[ ] Zadanie 1.3: Implementacja boilerplate'u gRPC w Rust (klient) i Go (serwer).

[ ] Zadanie 1.4: Konteneryzacja środowiska lokalnego (Docker Compose dla PostgreSQL i Qdrant/Vector DB).

Faza 2: Rust IaC Engine (Tydzień 2-3)
Cel: Analiza struktury chmury przed jej wdrożeniem.

[ ] Zadanie 2.1: Integracja biblioteki hcl-rs do parsowania plików Terraform.

[ ] Zadanie 2.2: Wyciąganie metadanych zasobów (Resource Type, Name, Visibility, Encryption Status).

[ ] Zadanie 2.3: Mapowanie zależności (budowa grafu zasobów wewnątrz Rusta i przesyłanie go do Go).

Faza 3: PII Deep Scanner (Tydzień 4-5)
Cel: Wykrywanie danych wrażliwych (Serce Rusta).

[ ] Zadanie 3.1: Implementacja silnika Regex + Walidacja statystyczna (np. algorytm Luhna dla kart płatniczych).

[ ] Zadanie 3.2: Moduł streamingu danych (czytanie próbek z S3/SQL bez zapisywania na dysku).

[ ] Zadanie 3.3: Generowanie "Privacy Score" dla każdego zasobu na podstawie wykrytych danych.

Faza 4: Go Orchestrator & FinOps (Tydzień 6-7)
Cel: Integracja z realnym światem chmury i kosztami.

[ ] Zadanie 4.1: Integracja z AWS Price List API / CloudWatch w celu pobierania realnych kosztów.

[ ] Zadanie 4.2: Silnik korelacji: Łączenie kosztu z ryzykiem (np. "Ten bucket kosztuje 2000$, a zawiera niezaszyfrowane dane PII").

[ ] Zadanie 4.3: Implementacja API dla frontendu (REST/WebSockets dla danych live).

Faza 5: Local AI Advisor (Tydzień 8-9)
Cel: Wykorzystanie LLM do generowania rozwiązań.

[ ] Zadanie 5.1: Integracja z Ollama API (Model: Llama 3 lub DeepSeek-Coder).

[ ] Zadanie 5.2: Implementacja warstwy RAG (Retrieval-Augmented Generation) w Go – dostarczanie kontekstu o infrastrukturze do modelu.

[ ] Zadanie 5.3: Moduł generowania "Remediation Snippets" (gotowe poprawki Terraform sugerowane przez AI).

Faza 6: React Visualization (Tydzień 10+)
Cel: UI klasy Enterprise.

[ ] Zadanie 6.1: Implementacja mapy zasobów przy użyciu React Flow.

[ ] Zadanie 6.2: Dashboard kosztowy (wykresy zużycia vs ryzyko).

[ ] Zadanie 6.3: Terminal interaktywny do czatu z asystentem AI o infrastrukturze.