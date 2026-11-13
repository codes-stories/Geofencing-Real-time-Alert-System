# Geofencing & Real-time Alert System

A full-stack geofencing and vehicle tracking system with real-time alert
notifications.

- **Backend**: Go (REST API + WebSocket), PostgreSQL/PostGIS for geospatial
  queries, with an in-memory fallback store for zero-dependency local runs
- **Frontend**: React (Vite) + Leaflet interactive map + Tailwind CSS
- **Infrastructure**: Docker & Docker Compose

Draw virtual boundaries on a map, register vehicles, stream their locations,
and watch live entry/exit alerts arrive the moment a vehicle crosses a
geofence — especially restricted zones.

## Repository layout

```
.
├── backend/                  # Go API server
│   ├── main.go               # entrypoint, graceful shutdown, store selection
│   ├── Dockerfile
│   └── internal/
│       ├── api/              # HTTP handlers, middleware, validation
│       ├── geo/              # point-in-polygon + polygon validation (unit tested)
│       ├── models/           # shared domain types
│       ├── store/            # Store interface, Postgres/PostGIS + memory impls
│       └── ws/               # WebSocket hub (broadcast to all clients)
├── frontend/                 # React app (Vite + Leaflet + Tailwind)
│   ├── Dockerfile            # multi-stage: node build -> nginx
│   ├── nginx.conf            # SPA hosting + API/WS reverse proxy
│   └── src/
├── docker-compose.yml        # db (PostGIS) + backend + frontend
├── SETUP.md                  # full setup & API testing guide
└── README.md                 # this file
```

## Quick start (Docker Compose)

```bash
docker compose up --build
```

Then open **http://localhost:3000** (frontend) and **http://localhost:8080/health**
(backend). Full instructions, including running without Docker, are in
[SETUP.md](SETUP.md).

## Key features

| Area | Capabilities |
| --- | --- |
| Geofences | Create polygonal boundaries (closed `[lat, lng]` rings), filter by `delivery_zone`, `restricted_zone`, `toll_zone`, `customer_area` |
| Vehicles | Register drivers/vehicles, track current position and geofence membership |
| Location updates | `POST /vehicles/location` stores a position, detects entry/exit transitions and returns current geofences |
| Alert rules | Configure `entry` / `exit` / `both` rules per geofence, optionally scoped to one vehicle (or all vehicles) |
| Real-time | `/ws/alerts` WebSocket broadcasts every matching event to all connected clients |
| History | Filterable, paginated violation history (`vehicle_id`, `geofence_id`, date range, `limit`/`offset`) |
| Map UI | Draw geofences by clicking the map, place vehicles with a click, live polygons/markers, toast notifications |
| Extras | Per-IP rate limiting on location updates, unit tests for the geospatial core |

## API response convention

Every response (success **and** error) includes the handler execution time in
nanoseconds:

```json
{ "…": "…", "time_ns": "1234567" }
```

## Documentation

- [SETUP.md](SETUP.md) — prerequisites, local/Docker setup, curl testing
  guide, frontend usage, architecture overview
- The API endpoint reference used by the frontend lives in
  `frontend/src/api/client.js` and the handlers under `backend/internal/api/`

## Notes for reviewers

- Real-time delivery is asynchronous: `POST /vehicles/location` never blocks
  on WebSocket writes.
- Restricted zones (`restricted_zone`) always generate alerts; other
  categories alert only when a matching rule exists.
- See [SETUP.md](SETUP.md) for the video walkthrough link slot and the
  deployment checklist (Docker Hub image, Vercel/Netlify frontend).
