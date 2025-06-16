# Przewodnik dewelopera

## Struktura repozytorium

- `cmd/` – główna komenda `pocket-docker`.
- `internal/cli` – obsługa CLI i flag.
- `internal/runtime` – logika uruchamiania procesów w izolacji.
- `internal/store` – zapis informacji o kontenerach i obrazach.
- `internal/logging` – obsługa logów kontenerów.
- `scripts/` – skrypty testowe.

## Konwencje kodu

- Kod i komentarze wyłącznie po angielsku.
- `gofmt` jest obowiązkowy przed commitem.
- Funkcje nie powinny przekraczać **15** w skali `gocyclo`.

## Jak dodać funkcję

1. Utwórz issue z opisem i odniesieniem do identyfikatora wymagania (BR/FR/NFR).
2. Stwórz gałąź funkcjonalną i dodaj testy jednostkowe.
3. Otwórz pull request z opisem zmian i linkiem do wymagania.
