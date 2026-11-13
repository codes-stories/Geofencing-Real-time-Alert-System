# SETUP.md — Geofencing & Real-time Alert System

## 1. Prerequisites

| Tool | Version | Purpose |
| --- | --- | --- |
| Go | 1.26+ | Backend build & tests |
| Node.js | 20+ (24 tested) | Frontend build |
| npm | comes with Node | Frontend dependencies |
| Docker + Docker Compose | Recent (Compose v2) | Full-stack containerized run |
| curl / websocat / browser | — | API testing |

Optional (only for running the backend **without** Docker against a real
database): PostgreSQL 16 + PostGIS 3.4.

---

## 2. Run with Docker Compose (recommended)

From the repository root:

```bash
docker compose up --build
```

This starts three containers:

| Service | Image | Host port | Description |
| --- | --- | --- | --- |
| `db` | `kartoza/postgis:16-3.4` | internal only | PostgreSQL + PostGIS, volume `pgdata` |
| `backend` | built from `backend/` | `8080` | Go API + WebSocket |
| `frontend` | built from `frontend/` | `3000` | React app served by nginx (proxies `/geofences`, `/vehicles`, `/alerts`, `/violations`, `/health`, `/ws` to the backend) |

Open:

- Frontend: http://localhost:3000
- Backend health: http://localhost:8080/health

### Port conflicts

Ports are overridable via environment variables:

```bash
BACKEND_PORT=18081 FRONTEND_PORT=13000 docker compose up --build
```

### Useful commands

```bash
docker compose ps                  # status + health
docker compose logs -f backend     # follow backend logs
docker compose down                # stop (keeps the db volume)
docker compose down -v             # stop and delete all data
```

---

## 3. Run locally without Docker

### Backend (in-memory store — zero dependencies)

```bash
cd backend
go test ./...          # unit tests (point-in-polygon, validation)
PORT=8090 go run .     # serves on http://localhost:8090
```

Without `DATABASE_URL` the server uses an **in-memory store** (data resets on
restart). This is the fastest way to develop against the API.

### Backend (PostgreSQL + PostGIS)

```bash
docker run -d --name geofence-db -p 5433:5432 \
  -e POSTGRES_USER=geofence -e POSTGRES_PASSWORD=geofence \
  -e POSTGRES_DB=geofence \
  kartoza/postgis:16-3.4

cd backend
DATABASE_URL=postgres://geofence:geofence@localhost:5433/geofence?sslmode=disable \
  PORT=8090 go run .
```

The schema (see `backend/internal/store/schema.sql`) is applied
automatically at startup — it is fully idempotent (`CREATE EXTENSION`,
`CREATE TABLE IF NOT EXISTS`, indexes).

### Frontend (dev server with hot reload)

```bash
cd frontend
npm install
npm run dev            # http://localhost:5173, proxies API to :8080
```

The Vite dev server proxies `/geofences`, `/vehicles`, `/alerts`,
`/violations`, `/health` and `/ws` (websocket) to `http://localhost:8080`.
If your backend runs on another port, edit the proxy target in
`frontend/vite.config.js`.

### Environment variables

| Variable | Default | Component | Meaning |
| --- | --- | --- | --- |
| `PORT` | `8080` | backend | HTTP listen port |
| `DATABASE_URL` | *(unset)* | backend | Postgres DSN; unset ⇒ in-memory store |
| `RATE_LIMIT_PER_MIN` | `300` | backend | Max location updates per IP per minute (`<= 0` disables) |
| `VITE_API_BASE` | `''` | frontend (build) | Absolute backend URL for deployed frontends, e.g. `https://api.example.com` |
| `BACKEND_PORT` | `8080` | compose | Host port published for the backend |
| `FRONTEND_PORT` | `3000` | compose | Host port published for the frontend |

---

## 4. API testing guide (curl)

> Replace `$BASE` with your backend origin (e.g. `http://localhost:8080`).
> Every response contains `"time_ns"` — the handler execution time in
> nanoseconds.

```bash
BASE=http://localhost:8080
```

### Health

```bash
curl -s $BASE/health
# {"status":"ok","time_ns":"123456"}
```

### 1. Create a geofence — `POST /geofences`

Coordinates are `[latitude, longitude]` pairs; the ring must be **closed**
(first == last) with at least 4 points.

```bash
curl -s -X POST $BASE/geofences -H 'Content-Type: application/json' -d '{
  "name": "Downtown Delivery Zone",
  "description": "Main delivery area for downtown customers",
  "coordinates": [
    [37.7749, -122.4194],
    [37.7849, -122.4194],
    [37.7849, -122.4094],
    [37.7749, -122.4094],
    [37.7749, -122.4194]
  ],
  "category": "delivery_zone"
}'
# {"id":"geo_xxxxxxxx","name":"Downtown Delivery Zone","status":"active","time_ns":"..."}
```

`category` must be one of `delivery_zone`, `restricted_zone`, `toll_zone`,
`customer_area`.

Validation failures return `400` with `{"error":"…","time_ns":"…"}`:

```bash
# unclosed polygon
curl -s -X POST $BASE/geofences -H 'Content-Type: application/json' \
  -d '{"name":"bad","coordinates":[[0,0],[0,1],[1,1],[1,0]],"category":"toll_zone"}'
```

### 2. List geofences — `GET /geofences?category=`

```bash
curl -s $BASE/geofences
curl -s "$BASE/geofences?category=restricted_zone"
# optional pagination (adds total_count to the response):
curl -s "$BASE/geofences?limit=10&offset=0"
```

### 3. Register a vehicle — `POST /vehicles`

```bash
curl -s -X POST $BASE/vehicles -H 'Content-Type: application/json' -d '{
  "vehicle_number": "KA-01-AB-1234",
  "driver_name": "John Doe",
  "vehicle_type": "truck",
  "phone": "+1234567890"
}'
# {"id":"veh_xxxxxxxx","vehicle_number":"KA-01-AB-1234","status":"active","time_ns":"..."}
```

Duplicate `vehicle_number` → `409`.

### 4. List vehicles — `GET /vehicles`

```bash
curl -s $BASE/vehicles
curl -s "$BASE/vehicles?limit=20&offset=20"   # optional pagination
```

### 5. Update a vehicle location — `POST /vehicles/location`

This endpoint stores the position, detects **entry/exit** transitions against
the previous state, writes violation history and pushes real-time alerts.

```bash
curl -s -X POST $BASE/vehicles/location -H 'Content-Type: application/json' -d '{
  "vehicle_id": "veh_xxxxxxxx",
  "latitude": 37.7849,
  "longitude": -122.4194,
  "timestamp": "2025-01-15T10:35:00Z"
}'
# {"vehicle_id":"veh_…","location_updated":true,
#  "current_geofences":[{"geofence_id":"geo_…","geofence_name":"…","status":"inside"}],
#  "time_ns":"…"}
```

Try moving the vehicle **outside** the polygon and back in to generate
`exit` + `entry` events.

### 6. Get current location — `GET /vehicles/location/{vehicle_id}`

```bash
curl -s $BASE/vehicles/location/veh_xxxxxxxx
```

### 7. Configure an alert rule — `POST /alerts/configure`

```bash
# rule for one vehicle
curl -s -X POST $BASE/alerts/configure -H 'Content-Type: application/json' -d '{
  "geofence_id": "geo_xxxxxxxx",
  "vehicle_id": "veh_xxxxxxxx",
  "event_type": "entry"
}'

# rule for ALL vehicles (omit vehicle_id)
curl -s -X POST $BASE/alerts/configure -H 'Content-Type: application/json' -d '{
  "geofence_id": "geo_xxxxxxxx",
  "event_type": "both"
}'
# {"alert_id":"alert_…","geofence_id":"geo_…","vehicle_id":"…","event_type":"entry","status":"active","time_ns":"…"}
```

`event_type` ∈ `entry` | `exit` | `both`.

> **Note:** `restricted_zone` geofences are *always* monitored — they emit
> alerts for entry and exit even without an explicit rule. Other categories
> alert only when a matching rule exists.

### 8. List alert rules — `GET /alerts`

```bash
curl -s $BASE/alerts
curl -s "$BASE/alerts?geofence_id=geo_xxxxxxxx"
curl -s "$BASE/alerts?vehicle_id=veh_xxxxxxxx"
curl -s "$BASE/alerts?limit=10&offset=0"      # optional pagination
```

### 9. Violation history — `GET /violations/history`

```bash
curl -s "$BASE/violations/history"
curl -s "$BASE/violations/history?vehicle_id=veh_xxxxxxxx&limit=100"
curl -s "$BASE/violations/history?geofence_id=geo_xxxxxxxx&start_date=2025-01-01T00:00:00Z&end_date=2025-02-01T00:00:00Z"
curl -s "$BASE/violations/history?limit=25&offset=25"
# {"violations":[…],"total_count":245,"time_ns":"…"}
```

Query params: `vehicle_id`, `geofence_id`, `start_date`, `end_date`
(RFC3339), `limit` (default 50, max 500), `offset` (default 0).

> **Pagination note:** `GET /geofences`, `GET /vehicles` and `GET /alerts`
> also accept optional `limit`/`offset`. Without them the full list is
> returned exactly as specified in the contract; with them the response
> additionally includes `total_count`.

### WebSocket — `/ws/alerts`

Connect any WebSocket client and then trigger a location update:

```bash
# terminal 1 (requires node 22+, or use websocat/browser)
node -e '
const ws = new WebSocket("ws://localhost:8080/ws/alerts");
ws.onopen = () => console.log("connected");
ws.onmessage = (e) => console.log("ALERT", e.data);
'

# terminal 2 — trigger an entry event
curl -s -X POST $BASE/vehicles/location -H 'Content-Type: application/json' \
  -d '{"vehicle_id":"veh_xxxxxxxx","latitude":37.7800,"longitude":-122.4150,"timestamp":"2025-01-15T10:35:00Z"}'
```

Message shape:

```json
{
  "event_id": "evt_999",
  "event_type": "entry",
  "timestamp": "2025-01-15T10:35:00Z",
  "vehicle": {"vehicle_id": "veh_456", "vehicle_number": "KA-01-AB-1234", "driver_name": "John Doe"},
  "geofence": {"geofence_id": "geo_123", "geofence_name": "Downtown Delivery Zone", "category": "delivery_zone"},
  "location": {"latitude": 37.7849, "longitude": -122.4194}
}
```

---

## 5. Frontend usage guide

1. Open http://localhost:3000 (Docker) or http://localhost:5173 (`npm run dev`).
2. **Map tab**
   - *Draw geofence*: click **Draw geofence**, click the map to add vertices,
     press **Finish** (the polygon auto-closes), fill in name / description /
     category and save. Polygons are colour-coded per category.
   - *Set vehicle location*: pick a vehicle in **Set vehicle location** mode
     and click the map — the backend evaluates geofences and may push an
     alert. You can also open a vehicle marker popup and use
     *Set location here*.
   - The sidebar lists geofences with a category filter; clicking one flies
     the map to its bounds.
3. **Vehicles tab** — register vehicles and see their current geofence
   status (refreshed from `GET /vehicles/location/{id}`).
4. **Alerts tab** — create rules (geofence + optional vehicle + event type)
   and manage the list.
5. **Violations tab** — filter history by vehicle, geofence and date range,
   paginate with Prev/Next using `total_count`.
6. **Live alerts** — the header shows a connection dot (green *Live* /
   red *Reconnecting*). Incoming events pop toast notifications and land in
   the bell feed (badge = unread count).

---

## 6. Architecture overview

```
Browser (React + Leaflet)
   │  REST (JSON)                         ▲ WebSocket /ws/alerts
   ▼                                      │
nginx (frontend container) ── proxies ──► Go API server
   /geofences /vehicles /alerts           │
   /violations /health /ws                ├─ api/    handlers, validation, rate limit, CORS
                                          ├─ ws/     hub: register clients, broadcast alerts
   POST /vehicles/location flow:          ├─ geo/    ray-casting point-in-polygon (+ tests)
   1. validate + save location            └─ store/  Store interface
   2. ST_Contains(geofences, point)  ◄─────── PostgreSQL + PostGIS (GiST index)
   3. diff vs previous membership          or   in-memory (no DATABASE_URL)
   4. write violations (entry/exit)
   5. match alert rules → Broadcast (async goroutine, never blocks HTTP)
   6. respond with current_geofences
```

Design notes:

- **Entry/exit detection** compares the containment set of the new position
  with `vehicle_geofence_state`; the state is replaced atomically after the
  diff, so consecutive updates at the same spot produce no duplicate events.
- **Alert delivery** is decoupled: the event is persisted first, then handed
  to the hub via `go hub.Broadcast(...)`. Slow WebSocket clients are dropped
  rather than allowed to stall location updates.
- **`time_ns`** is injected by a single response helper
  (`api.writeJSON`) measuring `time.Since(start)` per handler.
- **Rate limiting** is a fixed-window per-IP counter on
  `POST /vehicles/location` (`RATE_LIMIT_PER_MIN`).

---

## 7. Tests

```bash
cd backend
go test ./...        # unit + integration tests
go vet ./...
gofmt -l .
```

The suite covers:

- `internal/geo` — point-in-polygon (square, concave, open/closed rings,
  boundary points, real-world coordinates), polygon validation rules,
  GeoJSON rendering
- `internal/api` — httptest-driven integration tests for every endpoint:
  validation (400s), duplicates (409), not-founds (404), entry/exit
  detection, violation history filters/pagination, alert-rule matching
  (per-vehicle vs all-vehicles vs restricted-zone auto-alert), WebSocket
  alert payloads, `time_ns` presence, CORS/OPTIONS and rate limiting

```bash
cd frontend
npm run build        # production build must pass
```

---

## 8. Deployment checklist

- **Backend image**: `docker build -t <user>/geofencing-backend ./backend`
  then `docker push <user>/geofencing-backend` (set `DATABASE_URL` and
  `PORT` on the target platform; expose port 8080).
- **Frontend**: deploy the `frontend/` folder to Vercel / Netlify /
  Cloudflare Pages with build command `npm run build`, output `dist`, and
  `VITE_API_BASE=https://<your-backend-host>` so API + WebSocket calls reach
  the containerized backend. (The provided `frontend/Dockerfile` +
  `nginx.conf` alternatively run the SPA behind nginx with the same proxy.)
- **Video walkthrough**: record the 30–45 min demo described in the
  assessment and link it here: `<VIDEO_LINK>`.
