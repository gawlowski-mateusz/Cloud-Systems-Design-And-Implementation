# Raport - Projekt 1

## 1. Temat aplikacji

Zrealizowano aplikację webową do rezerwacji sal konferencyjnych z obsługą multimediów.

## 2. Backend + Frontend jako oddzielne moduły

1. Backend: API w Go (Gin), katalog `backend/`.
2. Frontend: aplikacja statyczna (HTML/CSS/JS), katalog `frontend/`.
3. Moduły są niezależne i mogą być hostowane osobno.

## 3. Endpointy

### GET

1. `GET /health`
2. `GET /api/reservations`
3. `GET /api/media`
4. `GET /api/media/:id`

### POST

1. `POST /api/auth/register`
2. `POST /api/auth/login`
3. `POST /api/reservations`
4. `POST /api/media` (upload pliku)

Wymóg GET/POST dla plików jest spełniony przez `POST /api/media` i `GET /api/media/:id`.

## 4. Docker

1. `backend/Dockerfile` + `backend/docker-compose.yml`
2. `frontend/Dockerfile` + `frontend/docker-compose.yml`

Spełniono wymóg osobnej konfiguracji Dockera dla obu modułów.

## 5. AWS CLI

Konfiguracja wykonywana poleceniem:

```bash
aws configure
```

Weryfikacja:

```bash
aws sts get-caller-identity
```

## 6. Terraform

Folder `infrastructure/` zawiera:

1. `main.tf`
2. `variables.tf`
3. `outputs.tf`
4. `terraform.tfvars.example`

Konfiguracja obejmuje:

1. AWS Elastic Beanstalk (frontend i backend osobno),
2. RDS PostgreSQL,
3. S3 na pliki,
4. CloudWatch Log Groups,
5. AWS Cognito User Pool + App Client.

## 7. Cognito

Backend wspiera tryb `AUTH_PROVIDER=cognito`.

Wymagane zmienne:

1. `COGNITO_REGION`
2. `COGNITO_CLIENT_ID`

Rejestracja/logowanie użytkownika odbywa się przez AWS Cognito.

## 8. Wdrożenie i testy

Weryfikacja funkcjonalna obejmuje:

1. rejestrację i logowanie,
2. tworzenie oraz pobieranie rezerwacji,
3. upload i pobieranie plików,
4. integrację z PostgreSQL,
5. poprawne uruchomienie kontenerów backend/frontend.

## 9. Odpowiednik przez AWS Console

Równoważną infrastrukturę należy utworzyć ręcznie w konsoli AWS:

1. RDS,
2. S3,
3. Cognito,
4. 2x Elastic Beanstalk,
5. CloudWatch.

Następnie wdrożyć oba moduły i potwierdzić działanie endpointów i UI.

## 10. Podsumowanie

Wymagania projektowe zostały zaadresowane przez komplet:

1. aplikacji (backend + frontend),
2. konfiguracji Docker,
3. konfiguracji Terraform,
4. integracji z AWS Cognito,
5. dokumentacji uruchomienia i wdrożenia.
