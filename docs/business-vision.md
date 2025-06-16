# Wizja biznesowa projektu Pocket-Docker

Projekt **Pocket-Docker** ma dostarczyć lekki i edukacyjny runtime kontenerowy.
Główne cele biznesowe to:

1. **Minimalizacja kosztów laboratoriów** – narzędzie działa jako pojedynczy plik binarny (~5 MB) kompilowany w mniej niż 10 s.
2. **Transparentność technologii** – ma pokazywać, że kontener to jedynie zestaw prymitywów Linuksa.
3. **Zastosowanie w środowiskach CTF/IoT/edge** – pełen Docker jest zbyt ciężki, Pocket‑Docker ma wypełnić tę lukę.
4. **Bezpieczeństwo** – brak uprawnień roota w przestrzeni użytkownika, seccomp oraz OOM‑killer.
5. **Łatwość dalszego rozwoju** – czytelna specyfikacja i testy obniżają koszty utrzymania.

Projekt skierowany jest do uczelni, firm szkoleniowych oraz zespołów tworzących rozwiązania osadzone lub pentesterskie.