import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// fetched_at 是 Go time.Time 序列化出的 RFC3339 字符串
export function timeAgo(iso: string): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "";
  const diff = Date.now() - t;
  const min = Math.floor(diff / 60000);
  if (min < 1) return "刚刚";
  if (min < 60) return `${min}分钟前`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}小时前`;
  const d = Math.floor(h / 24);
  if (d < 30) return `${d}天前`;
  return new Date(iso).toLocaleDateString("zh-CN");
}

export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString("zh-CN", { hour12: false });
}

export const STATUS_LABELS: Record<string, string> = {
  new: "新单",
  want: "想投",
  proposed: "已投",
  ignored: "忽略",
  rejected: "已淘汰",
  stale: "死帖",
};

export const PROPOSALS_LABELS: Record<string, string> = {
  fewer_than_5: "投标<5",
  "5_to_10": "投标5-10",
  "10_to_15": "投标10-15",
  "15_to_20": "投标15-20",
  "20_to_50": "投标20-50",
  "50_plus": "投标50+",
};

export function statusLabel(s: string): string {
  return STATUS_LABELS[s] ?? s;
}
