import type { ChatSpeed } from '../types/chat';

export const USERNAMES: string[] = [
  "xXDarkNinjaXx", "coolkid99", "PixelWarrior", "ChatSpammer42",
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
  "JustHere4Chat", "ClickBaitVictim",
];

export const GENERIC_ITEMS: string[] = [
  "Pizza", "Burger", "Pasta", "Sushi", "Tacos", "Ramen", "Steak", "Salad",
];

const MESSAGE_TEMPLATES: string[] = [
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
];

const DONATION_TEMPLATES: string[] = [
  "cheer{amount} {item} deserves to win!",
  "Here's {amount} bits for {item}!",
  "cheer{amount} voting {item} all day!",
  "{amount} bits because {item} is the best!",
];

function hashString(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash;
  }
  return Math.abs(hash);
}

export function getUsernameColor(username: string): string {
  const hash = hashString(username);
  const hue = hash % 360;
  return `hsl(${hue}, 80%, 60%)`;
}

export function pickRandom<T>(arr: T[]): T {
  return arr[Math.floor(Math.random() * arr.length)];
}

export function generateMessage(labels: string[], _topic: string): { message: string; item: string } {
  const item = pickRandom(labels.length > 0 ? labels : GENERIC_ITEMS);
  const template = pickRandom(MESSAGE_TEMPLATES);
  return {
    message: template.replace(/{item}/g, item),
    item,
  };
}

export function generateDonationMessage(labels: string[], _topic: string): { message: string; item: string; bits: number } {
  const item = pickRandom(labels.length > 0 ? labels : GENERIC_ITEMS);
  const bits = [100, 500, 1000, 5000, 10000][Math.floor(Math.random() * 5)];
  const template = pickRandom(DONATION_TEMPLATES);
  return {
    message: template.replace(/{item}/g, item).replace(/{amount}/g, String(bits)),
    item,
    bits,
  };
}

export function getRandomUsername(pool: string[] = USERNAMES): string {
  return pickRandom(pool);
}

export function getSpeedRangeMs(speed: ChatSpeed): [number, number] {
  switch (speed) {
    case 'slow':
      return [300, 1000];
    case 'normal':
      return [125, 333];
    case 'fast':
      return [50, 100];
  }
}
