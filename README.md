# Cloud-Systems-Design-And-Implementation

Conference hall reservation app with a Go backend and a lightweight frontend.

## Project Structure

- `backend/` - Gin API + PostgreSQL connection
- `frontend/` - Static frontend served by backend

## Backend Setup

1. Start PostgreSQL (from `backend/`):

	```bash
	docker compose up -d
	```

2. (Optional) Copy env file:

	```bash
	cp .env.example .env
	```

3. Run backend (from `backend/`):

	```bash
	go run .
	```

Server runs on `http://localhost:8080` by default.

## API Endpoints

- `GET /health` - health check
- `POST /api/auth/register`
  - body: `{ "fullName": "John Doe", "email": "john@example.com", "password": "password123" }`
- `POST /api/auth/login`
  - body: `{ "email": "john@example.com", "password": "password123" }`
- `GET /api/reservations`
	- auth: `Authorization: Bearer <jwt-token>`
	- response: current logged-in user's reservations
- `POST /api/reservations`
	- auth: `Authorization: Bearer <jwt-token>`
	- body: `{ "hallId": "grand-auditorium", "date": "2026-03-23", "start": "10:00", "end": "11:00", "attendees": 20, "purpose": "Team Sync" }`

## Frontend

The frontend is served by the Go server at:

- `GET /` - conference hall reservation UI

Features:

- User register and login
- Hall catalog view
- Reservation creation form
- Reservation list loaded from PostgreSQL via backend endpoints