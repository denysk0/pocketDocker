# Podręcznik użytkownika

## Instalacja

1. Zbuduj narzędzie:

```bash
go build -o pocket-docker ./cmd/pocket-docker
```

2. (Opcjonalnie) Pobierz przykładowy obraz:

```bash
./pocket-docker pull https://example.com/busybox.tar
```

## Uruchamianie kontenera

```bash
sudo ./pocket-docker run --rootfs busybox.tar --cmd "/bin/sh -c 'echo hello'" --memory 67108864
```

- `--rootfs` – ścieżka do obrazu lub nazwa w cache.
- `--cmd` – polecenie startowe w kontenerze.
- `--memory` – limit pamięci w bajtach.

## Zarządzanie

- Lista kontenerów: `./pocket-docker ps`
- Zatrzymanie kontenera: `sudo ./pocket-docker stop <ID>`
- Usunięcie kontenera: `./pocket-docker rm <ID>`
- Dostęp do logów: `./pocket-docker logs <ID>`

Więcej opcji dostępnych jest poprzez `--help` dla każdej komendy.