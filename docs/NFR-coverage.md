# Pokrycie wymagań niefunkcjonalnych

| Kategoria ISO 25010 | Metryka/Kryterium | ID |
|---------------------|------------------|----|
| Wydajność | start kontenera <3 s, `ps` <1 s, zużycie RAM runtime <10 MB | NFR-PE-1 |
| Użyteczność | `--help` dla każdej komendy, zgodność z POSIX‑CLI, kolorowe logi | NFR-US-1 |
| Niezawodność | ≥99% uptime przy `restartMax` ≥3, watchdog ≤5 s | NFR-RE-1 |
| Bezpieczeństwo | domyślnie user-ns, brak roota, seccomp default‑deny | NFR-SE-1 |
| Kompatybilność | działa na glibc 2.31+, musl 1.2+ | NFR-CO-1 |
| Łatwość konserwacji | wynik gocyclo ≤15, `go vet` bez błędów, diagram architektury | NFR-MA-1 |
| Przenaszalność | cross‑compile GOOS=linux GOARCH=arm64 w CI, brak CGO | NFR-PO-1 |
| Funkcjonalna zgodność | pokrycie testami jednostkowymi ≥80%, każdy FR ma test e2e | NFR-FS-1 |