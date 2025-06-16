# Specyfikacja wymagań systemowych

## Wymagania użytkownika

| ID | Wymaganie |
|----|-----------|
| UR-1 | Użytkownik kompiluje i uruchamia runtime na dowolnym host‑Linux w <10 s |
| UR-2 | Użytkownik startuje proces w izolacji jednym poleceniem |
| UR-3 | Kod źródłowy można przeanalizować (open source, komentarze) |
| UR-4 | Proces root wewnątrz kontenera nie jest rootem na hoście |
| UR-5 | W przypadku OOM proces jest ubijany lub restartowany automatycznie |
| UR-6 | Zespół łatwo czyta i testuje kod |

## Wymagania funkcjonalne

| ID | Opis |
|----|------|
| FR-1 | Komenda `go build` tworzy binarkę ≤6 MB |
| FR-2 | `pull` pobiera i keszuje obraz tar |
| FR-3 | `run` tworzy przestrzenie PID/UTS/MNT/user/ns oraz wykonuje `pivot_root` |
| FR-4 | `ps` wyświetla PID-y działających kontenerów |
| FR-5 | Kod ma ≤3000 LOC, komentarze >25% |
| FR-6 | Mapowania `uid_map/gid_map` ustawiają 0↔UID_host |
| FR-7 | Watchdog restartuje proces, `restartMax` określa limit |
| FR-8 | Limit RAM/CPU przez cgroup v2 |
| FR-9 | 100% poleceń CLI objęte testami Go |
| FR-10 | CI uruchamia `go vet` i `gocyclo` |

## Wymagania niefunkcjonalne

Patrz dokument [NFR-coverage](NFR-coverage.md).