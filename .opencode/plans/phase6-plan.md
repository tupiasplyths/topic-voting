# Phase 6 — Polish

## Overview

Final polish pass: OBS-friendly display overlay with smooth animations, error handling & UX improvements, Docker setup for all services, and bug fixes.

From the original plan: *Phase 6 — OBS-friendly display mode, animations, error handling*

---

## A. Display Page Enhancements

The current `DisplayPage` uses the same `VoteBarChart` (Recharts) as the host dashboard. For an OBS Browser Source overlay, pure CSS bars are more performant, reliable, and animatable. A dedicated overlay component gives full control over transitions.

### 1. [NEW] `frontend/src/components/DisplayOverlay.tsx`

Dedicated OBS overlay component — replaces `VoteBarChart` on the display page.

**Props:**
```typescript
interface Props {
  entries: LeaderboardEntry[];
  maxEntries: number;
}
```

**Features:**
- Pure CSS horizontal bars (no Recharts dependency for this view)
- Bars animate width changes with `transition: width 500ms ease-out`
- Entries reorder with smooth CSS transitions using `transform: translateY()` — each entry is absolutely positioned, and when rank changes, the Y offset animates
- Rank position badges (1st gold, 2nd silver, 3rd bronze)
- Crown icon (from lucide-react `Crown`) on the #1 entry with a subtle pulse animation
- Point count with animated number transitions (CSS counter or `requestAnimationFrame` counting)
- "NEW" badge on entries that just appeared (track previous entries set)
- Donations/weighted votes subtly highlighted
- Full-width, no scrollbars, no unnecessary chrome

**Animation approach:**
- Use `key`-based rendering with `previousEntries` ref to detect rank changes
- Each bar item gets `style={{ transform: translateY(index * itemHeight) }}` with `transition: transform 400ms ease-out`
- Width changes: `style={{ width: pct% }}` with `transition: width 500ms ease-out`
- New entries: `animation: fadeSlideIn 300ms ease-out` keyframe
- Pulse on #1: `@keyframes pulse { 0%, 100% { opacity: 1 } 50% { opacity: 0.7 } }` — 2s infinite

### 2. [EDIT] `frontend/src/pages/DisplayPage.tsx`

- Replace `VoteBarChart` with `DisplayOverlay`
- Add a subtle loading state when no data yet
- Add `?show_title=true` query param to toggle topic title visibility (default true)
- Ensure transparent background mode works for OBS (set `body { background: transparent }` when `?bg=transparent`)

### 3. [EDIT] `frontend/src/index.css`

Add keyframe animations for the display overlay:
```css
@keyframes fadeSlideIn {
  from { opacity: 0; transform: translateX(-20px); }
  to { opacity: 1; transform: translateX(0); }
}

@keyframes pulse-crown {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.1); }
}
```

---

## B. Error Handling & UX Improvements

### 4. [NEW] `frontend/src/components/Toast.tsx`

Lightweight toast notification system.

**Features:**
- Toast appears at top-right, auto-dismisses after 3 seconds
- Supports: `success` (green), `error` (red), `info` (blue) types
- Slide-in/fade-out animation
- Stack multiple toasts

**API:**
```typescript
// Simple hook-based approach
function useToast(): { toast: (message: string, type: 'success' | 'error' | 'info') => void }
```

Rendered once at the app root level via context or a fixed container.

### 5. [EDIT] `frontend/src/hooks/useWebSocket.ts`

**Bug fix:** Currently connects even when `url` is empty string. The hook should:
- Skip connection entirely when `url` is falsy (empty string, null)
- Only attempt to connect when a valid URL is provided
- Clean up properly when URL changes to empty (close existing connection)

```typescript
// In useEffect: check url before connecting
useEffect(() => {
  if (!url) {
    close();
    setStatus('closed');
    return;
  }
  connect();
  return () => { close(); };
}, [connect, close, url]);
```

This fixes the issue in `HostPage` and `DisplayPage` where empty URLs cause failed WebSocket connections.

### 6. [EDIT] `frontend/src/components/TopicManager.tsx`

- Clear error message automatically after 5 seconds (currently errors persist until next action)
- Add success toast when topic is created or closed
- Disable "Close" button while request is in-flight

### 7. [EDIT] `frontend/src/components/Leaderboard.tsx`

- Show loading spinner during initial fetch (before data arrives)
- Better empty state with an animated "waiting for votes" indicator

### 8. [EDIT] `frontend/src/api/client.ts`

- Add response interceptor for global error handling
- On network error or 5xx: show toast notification
- On 401/403: not applicable for this app (no auth), but structure allows future extension

---

## C. Docker & Deployment

### 9. [NEW] `backend/Dockerfile`

Multi-stage Go build:
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /server /server
COPY --from=builder /app/migrations /migrations
EXPOSE 8585
CMD ["/server"]
```

### 10. [NEW] `frontend/Dockerfile`

Multi-stage Node build:
```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 5442
CMD ["nginx", "-g", "daemon off;"]
```

### 11. [NEW] `frontend/nginx.conf`

Nginx config for SPA routing + API/WS proxy:
- Serve static files from `/usr/share/nginx/html`
- Fallback to `index.html` for SPA routes
- Proxy `/api/` to backend
- Proxy `/ws/` to backend with WebSocket upgrade

### 12. [NEW] `docker-compose.yml`

At project root, matching the spec:
```yaml
services:
  backend:
    build: ./backend
    ports: ["8585:8585"]
    env_file: .env
    depends_on:
      classifier:
        condition: service_healthy

  classifier:
    build: ./classifier
    ports: ["4747:4747"]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:4747/health"]
      interval: 5s
      retries: 10

  frontend:
    build: ./frontend
    ports: ["5442:5442"]
    depends_on:
      - backend
```

Note: PostgreSQL runs externally (as per spec: 192.168.1.101), not in Docker Compose.

### 13. [EDIT] `.env.example`

- Update `DB_HOST` comment to clarify Docker vs local usage
- Add `VITE_API_URL` for frontend container

---

## D. Minor Fixes & Cleanup

### 14. [EDIT] `frontend/src/components/MockChat.tsx`

- Fix: when `topicId` changes while running, simulation should restart cleanly (currently the `useEffect` for `topicId` change reconnects WS but doesn't clear the timer, leading to potential double-scheduling)
- Add: pause indicator when simulation is stopped
- Add: "Clear chat" button to reset the message list

### 15. [EDIT] `frontend/src/components/VoteBarChart.tsx`

- Fix: height jumps when entries change (use a minimum height or animate height changes)
- Add: empty state message ("No data yet")

---

## File Summary

| File | Action | Area |
|------|--------|------|
| `frontend/src/components/DisplayOverlay.tsx` | NEW | A — OBS display |
| `frontend/src/pages/DisplayPage.tsx` | EDIT | A — OBS display |
| `frontend/src/index.css` | EDIT | A — Animations |
| `frontend/src/components/Toast.tsx` | NEW | B — Error handling |
| `frontend/src/hooks/useWebSocket.ts` | EDIT | B — Bug fix |
| `frontend/src/components/TopicManager.tsx` | EDIT | B — UX |
| `frontend/src/components/Leaderboard.tsx` | EDIT | B — UX |
| `frontend/src/api/client.ts` | EDIT | B — Error handling |
| `backend/Dockerfile` | NEW | C — Docker |
| `frontend/Dockerfile` | NEW | C — Docker |
| `frontend/nginx.conf` | NEW | C — Docker |
| `docker-compose.yml` | NEW | C — Docker |
| `.env.example` | EDIT | C — Config |
| `frontend/src/components/MockChat.tsx` | EDIT | D — Fixes |
| `frontend/src/components/VoteBarChart.tsx` | EDIT | D — Fixes |

**Total: 15 files (6 new, 9 edited)**

---

## Key Design Decisions

1. **DisplayOverlay is a separate component from VoteBarChart:** OBS Browser Source has specific constraints (no scrollbars, smooth animations, predictable layout). Recharts works for the host dashboard but introduces rendering overhead and layout unpredictability for an OBS overlay. Pure CSS gives full control.

2. **Toast over modal/dialog:** A lightweight toast is less intrusive than blocking modals. The host is managing topics while watching the leaderboard — toasts inform without interrupting workflow.

3. **nginx for frontend container:** The spec calls for a production-ready frontend container. nginx handles static file serving efficiently, SPA routing, and proxying API/WS requests. In development, Vite's proxy still handles this.

4. **No new npm dependencies:** Toast, animations, and overlay all use existing React + Tailwind. No additional libraries.

---

## Verification Plan

### Build Verification
```bash
cd frontend && npm run build        # TypeScript + Vite build
cd backend && go build ./cmd/server # Go build
docker compose build                # All containers build
```

### Manual Testing — Display Page
1. Create topic, start mock chat on fast speed
2. Open `/display?topic_id=...` in a separate browser
3. Verify: bars animate smoothly when values change
4. Verify: entries reorder smoothly when ranks change
5. Verify: new entries slide in from the left
6. Verify: #1 entry has crown with pulse
7. Verify: `?bg=transparent` gives transparent background (works as OBS overlay)
8. Verify: `?max_entries=5` shows only top 5

### Manual Testing — Error Handling
1. Stop the backend, try to create a topic → verify toast error appears and auto-dismisses
2. Start the backend, create a topic → verify success toast
3. Verify error messages in TopicManager auto-clear after 5s

### Manual Testing — Docker
```bash
docker compose up --build
# Backend health check
curl http://localhost:8585/api/health
# Classifier health check
curl http://localhost:4747/health
# Frontend
open http://localhost:5442
```
