# Cloud-Systems-Design-And-Implementation

Projekt: aplikacja webowa do rezerwacji sal konferencyjnych z obsługą plików multimedialnych.

## 1. Zakres funkcjonalny

### Backend API (Go + Gin)

Wymagane endpointy zostały zrealizowane:

1. `GET /health` - status API i połączenia DB
2. `GET /ready` - status gotowości backendu i połączenia DB
3. `GET /api/reservations` - lista rezerwacji zalogowanego użytkownika
4. `GET /api/media` - lista plików zalogowanego użytkownika
5. `GET /api/media/:id` - pobranie pliku multimedialnego
6. `POST /api/auth/register` - rejestracja użytkownika
7. `POST /api/auth/login` - logowanie użytkownika
8. `POST /api/reservations` - utworzenie rezerwacji
9. `POST /api/media` - upload pliku multimedialnego (`multipart/form-data`, pole `file`)

To spełnia warunek min. 2x GET i 2x POST, w tym GET/POST dla plików.

### Moduł Frontend

Frontend działa jako niezależna aplikacja statyczna:

1. rejestracja i logowanie,
2. tworzenie i podgląd rezerwacji,
3. upload plików,
4. lista plików i pobieranie.

Adres backendu jest konfigurowalny przez `frontend/config.js`:

```js
window.APP_CONFIG = {
  API_BASE_URL: "http://localhost:8080"
};
```

## 2. Struktura projektu

1. `backend/` - niezależny moduł API
2. `frontend/` - niezależny moduł UI
3. `infrastructure/` - Terraform dla AWS

## 3. Docker

### Backend

Pliki:

1. `backend/Dockerfile`
2. `backend/docker-compose.yml`
3. `backend/.ebignore`

Uruchomienie:

```bash
cd backend
docker compose up -d --build
```

To uruchamia:

1. API (`localhost:8080`)
2. PostgreSQL (`localhost:5432`)

Do Elastic Beanstalk buduj paczkę z katalogu `backend/`, ale bez lokalnego `docker-compose.yml`. Plik `backend/.ebignore` wycina go z paczki, więc EB użyje `backend/Dockerfile` zamiast trybu compose z lokalnym Postgres.

### Frontend

Pliki:

1. `frontend/Dockerfile`
2. `frontend/nginx.conf`
3. `frontend/docker-compose.yml`

Uruchomienie:

```bash
cd frontend
docker compose up -d --build
```

Frontend będzie dostępny pod `http://localhost:3000`.

## 4. Konfiguracja AWS CLI

W Learner Lab dane z `AWS Details` zmieniają się dynamicznie. Konfiguruj CLI po każdym starcie labu:

```bash
aws configure
```

Podaj:

1. Access Key ID
2. Secret Access Key
3. Region (np. `eu-central-1`)
4. Output format (`json`)

Weryfikacja:

```bash
aws sts get-caller-identity
```

## 5. Terraform

### Zawartość

Folder `infrastructure/` zawiera:

1. `main.tf`
2. `variables.tf`
3. `outputs.tf`
4. `terraform.tfvars.example`

Konfiguracja obejmuje:

1. 2 niezależne środowiska AWS Elastic Beanstalk (frontend i backend),
2. RDS PostgreSQL,
3. S3 na pliki multimedialne,
4. CloudWatch Log Groups,
5. AWS Cognito (User Pool + App Client).

### Uruchomienie Terraform

```bash
cd infrastructure
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform plan
terraform apply
```

## 6. Cognito w aplikacji

Backend wspiera dwa tryby auth przez zmienną `AUTH_PROVIDER`:

1. `local` - lokalne konto + hasło,
2. `cognito` - rejestracja i logowanie przez AWS Cognito.

W trybie `cognito` ustaw w backendzie:

1. `AUTH_PROVIDER=cognito`
2. `COGNITO_REGION=<aws-region>`
3. `COGNITO_CLIENT_ID=<user-pool-client-id>`

## 7. Deployment na AWS Elastic Beanstalk

Przykładowy flow:

1. Zbuduj paczki ZIP dla backend i frontend.
2. Wyślij je do bucketu wersji (`aws_s3_bucket.eb_versions`).
3. Ustaw klucze ZIP w `terraform.tfvars`:
   - `backend_source_bundle_key`
   - `frontend_source_bundle_key`
4. Wykonaj `terraform apply`.
5. Odczytaj URL-e z `terraform output`.

Backend ZIP powinien zawierać `Dockerfile`, kod Go i pliki modułu, ale nie lokalny `docker-compose.yml`.

## 8. Weryfikacja działania

Po wdrożeniu sprawdź:

1. rejestrację/logowanie,
2. dodawanie i listowanie rezerwacji,
3. upload i pobieranie plików,
4. logi CloudWatch,
5. zapis danych w RDS,
6. obiekty plików w S3.

## 9. Odpowiednik konfiguracji przez AWS Console

Aby spełnić punkt z konfiguracją przez WWW AWS:

1. utwórz ręcznie RDS PostgreSQL,
2. utwórz S3 bucket,
3. utwórz Cognito User Pool i App Client,
4. utwórz 2 aplikacje Elastic Beanstalk,
5. skonfiguruj ENV backendu identycznie jak w Terraform,
6. uruchom frontend i backend jako oddzielne deploymenty,
7. włącz logowanie do CloudWatch.

## 10. Raport

Raport podsumowujący znajduje się w pliku `REPORT.md`.
