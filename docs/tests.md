# Plan testów

Dokument opisuje strategię testowania projektu **Pocket-Docker**.

## Poziomy testów

1. **Testy jednostkowe** – sprawdzają poszczególne funkcje pakietów Go.
2. **Testy integracyjne** – skrypty w `scripts/` uruchamiają pełne scenariusze CLI.
3. **Testy end‑to‑end** – `scripts/test_e2e.sh` weryfikuje najważniejsze funkcje narzędzia.

## Pokrycie i automatyzacja

- Minimalne pokrycie testami jednostkowymi: **80%**.
- Wszystkie komendy CLI posiadają testy.
- W CI uruchamiane są `go vet`, `gofmt -d` oraz `gocyclo`.

## Macierz CI

| System | Architektura |
|--------|--------------|
| Ubuntu 22.04 | amd64 |
| Ubuntu 22.04 | arm64 (cross‑compile) |
