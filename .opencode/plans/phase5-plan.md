# Phase 5 — Mock Twitch Chat

## Overview

Build a Mock Twitch Chat simulator that generates contextual messages, sends them through the backend chat WebSocket, and displays them in a Twitch-like UI. This is the "viewer" side of the system — simulating a live chat audience voting on the active topic.

---

## Proposed Changes

### 1. [NEW] `frontend/src/types/chat.ts`

Types for the mock chat system.

```typescript
export interface ChatMessage {
  id: string;
  username: string;
  message: string;
  color: string;
  is_donation: boolean;
  bits_amount: number;
  timestamp: number;
  status?: 'pending' | 'classified';
  classified_label?: string;
}

export type ChatSpeed = 'slow' | 'normal' | 'fast';

export interface ChatConfig {
  speed: ChatSpeed;
  donationRate: number; // 0-1, default 0.1
}
```

---

### 2. [NEW] `frontend/src/utils/mockChat.ts`

Message generator module with all the simulation logic.

**Username pool:** 50 pre-defined Twitch-style names:

```
["xXDarkNinjaXx", "coolkid99", "PixelWarrior", "ChatSpammer42",
 "NightOwl_", "StreamFanatic", "LurkerKing", "GlitchHunter",
 "NoobSlayer_", "TwitchFam2024", "xQcOWfan", "SubMachine",
 "ChatGPT_says_hi", "ViewCount42", "EmoteOnly", "PogChamp_",
 "KappaKing", "OmegaLul_", "PepegaBrain", "SadgeBoy",
 "MonkaS_", "CoolStoryBro", "DeadChat_", "JustVibing",
 "StreamSniper_", "AntiSimp_", "ChatMod_bot", "Hype_Train",
 "BitsBoss", "Sub_Bomb", "DroptheBan", "BigBrainPlays",
 "AFK_andy", "LurkModeON", "ChatWarrior_", "EmojiSpam",
 "KeySmashASDF", "RandomName42", "Chat_Addict", "Pog_Person",
 "Hype_Man_", "LowkeyFan", "StreamDragon", "ChatNinja_",
 "Vote_Master", "TopicKing_", "LoudTyping_", "SilentReader",
 "JustHere4Chat", "ClickBaitVictim"]
```

**Username color:** Deterministic hash-based color from username string (HSL with hue from hash, saturation 70-90%, lightness 55-70%). This ensures each username always gets the same color.

**Message templates:**

```
[
  "{item} is the best!",
  "Voting for {item}!!!",
  "{item} all the way",
  "gotta be {item}",
  "definitely {item}",
  "{item} {item} {item}",
  "nothing beats {item}",
  "{item} deserves to win",
  "my vote is {item}",
  "{item} is clearly the winner",
  "everyone knows {item} is #1",
  "{item} for the win",
  "i choose {item}",
  "{item} ftw!",
  "its gotta be {item} right?",
  "{item} is unmatched",
]
```

**Donation templates:**

```
[
  "cheer{amount} {item} deserves to win!",
  "Here's {amount} bits for {item}!",
  "cheer{amount} voting {item} all day!",
  "{amount} bits because {item} is the best!",
]
```

**Word bank logic:**
- Fetch labels from `GET /api/votes/labels?topic_id=...`
- If no labels exist yet, fall back to a generic list: `["Pizza", "Burger", "Pasta", "Sushi", "Tacos", "Ramen", "Steak", "Salad"]`

**Speed intervals:**
- `slow`: 1 message every 300-1000ms (1-3/sec)
- `normal`: 1 message every 125-333ms (3-8/sec)
- `fast`: 1 message every 50-100ms (10-20/sec)

**Exports:**

```typescript
export function generateMessage(labels: string[], topic: string): { message: string; item: string }
export function generateDonationMessage(labels: string[], topic: string): { message: string; item: string; bits: number }
export function getRandomUsername(pool: string[]): string
export function getUsernameColor(username: string): string
export function getSpeedRangeMs(speed: ChatSpeed): [number, number]
export function pickRandom<T>(arr: T[]): T
export const USERNAMES: string[]
export const GENERIC_ITEMS: string[]
```

---

### 3. [NEW] `frontend/src/components/MockChat.tsx`

The main chat UI component.

**Props:**

```typescript
interface Props {
  topicId: string | null;
  topicTitle?: string;
}
```

**State:**
- `messages: ChatMessage[]` — displayed chat messages (capped at 200, oldest dropped)
- `isRunning: boolean` — simulation active flag
- `speed: ChatSpeed` — current speed setting
- `labels: string[]` — known labels for the active topic (refreshed periodically)

**Behavior:**

1. On mount or `topicId` change:
   - Fetch labels from `getLabels(topicId)`
   - Start a periodic refresh of labels every 5 seconds (to pick up new labels as they're classified)
   - Connect to `WS /ws/chat?topic_id=...` for sending/receiving messages

2. Simulation loop (when `isRunning` is true):
   - Use `setTimeout` with random delay based on speed (re-schedule after each message)
   - Each tick:
     - Pick random username from pool
     - Decide if donation (~10% chance)
     - Generate message using templates + labels
     - Send via WebSocket: `{ type: "chat_message", data: { username, message, is_donation, bits_amount } }`
     - Add to local `messages` state immediately (optimistic, with `status: "pending"`)
   - Use `useRef` for the timer ID so it can be cleaned up

3. Rendering:
   - Twitch-like dark background (`bg-gray-950`)
   - Messages displayed in a scrollable container, auto-scrolling to bottom on new messages
   - Each message: `<span style={{color}}>{username}</span>: {message}`
   - Donation messages: highlighted with purple/pink background, bits icon (from lucide-react `Gem`), and "x{bits}" badge
   - System messages (e.g., "Simulation started", "Simulation stopped") styled differently (gray, centered)

**Controls bar (top of component):**
- Start/Stop toggle button (green/red)
- Speed selector: three buttons or a segmented control (slow / normal / fast)
- Stats: messages sent count

**Empty state:**
- When no topic is active: show message "No active topic. Create a topic on the Host page first."

---

### 4. [EDIT] `frontend/src/pages/ChatPage.tsx`

Replace the placeholder with the real implementation.

```tsx
export default function ChatPage() {
  const [activeTopic, setActiveTopic] = useState<Topic | null>(null);

  useEffect(() => {
    getActiveTopic()
      .then(setActiveTopic)
      .catch(() => {});
  }, []);

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-bold text-white">Mock Twitch Chat</h1>
        {activeTopic && (
          <span className="text-sm text-gray-400">
            Topic: <span className="text-indigo-400">{activeTopic.title}</span>
          </span>
        )}
      </div>
      <MockChat
        topicId={activeTopic?.id ?? null}
        topicTitle={activeTopic?.title}
      />
    </div>
  );
}
```

---

## File Summary

| File | Action | Description |
|------|--------|-------------|
| `frontend/src/types/chat.ts` | NEW | Chat message and simulation config types |
| `frontend/src/utils/mockChat.ts` | NEW | Message generator, username pool, color utility |
| `frontend/src/components/MockChat.tsx` | NEW | Main chat UI with Twitch-like rendering + simulation controls |
| `frontend/src/pages/ChatPage.tsx` | EDIT | Replace placeholder with MockChat integration |

No new npm dependencies needed.

---

## Key Design Decisions

1. **Optimistic message display:** Messages are added to the local list immediately when sent. The server's `chat_pending` broadcast confirms delivery to other clients. This avoids waiting for a round-trip before displaying the sender's own message.

2. **Labels refresh every 5s:** As the classifier generates new labels, the mock chat picks them up automatically and starts generating messages about the new items. This creates a natural "momentum" effect where popular items get more chat activity.

3. **Message cap at 200:** Prevents memory issues during long-running simulations. Oldest messages are dropped. Auto-scroll keeps the view at the bottom.

4. **Username color via HSL hash:** Deterministic per username (same user always same color). Uses a simple string hash to generate hue in [0, 360), fixed saturation (80%), fixed lightness (60%).

5. **No new dependencies needed:** Everything uses existing packages (React, lucide-react, Tailwind).

6. **`useRef` for simulation timer:** The simulation timeout is stored in a ref to avoid stale closures and allow clean cancellation. A `useEffect` cleanup cancels the timer on unmount or speed change.

---

## Verification Plan

### Manual Testing

1. Start the Go backend + Python classifier
2. Open the Host page, create an active topic (e.g., "Best Food")
3. Navigate to the Chat page
4. Verify the topic name is displayed
5. Click "Start" — messages should begin appearing at normal speed
6. Verify messages contain contextual food items
7. Switch speed to "fast" — verify messages come faster
8. Check the Host dashboard — leaderboard should be updating in real-time
9. Stop the simulation — verify no more messages are sent
10. Verify donation messages appear with purple highlight and bits badge

### Build Verification

```bash
cd frontend && npm run build
```
