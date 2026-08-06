import { cn } from "../lib/utils";

// 90+ 绿 / 70+ 黄 / 其余灰
export function ScoreBadge({ score, className }: { score: number; className?: string }) {
  const cls =
    score >= 90
      ? "bg-[#dcfce7] text-[#15803d]"
      : score >= 70
        ? "bg-[#fef3c7] text-warning"
        : "bg-neutral-100 text-neutral-500";
  return (
    <span
      className={cn(
        "tnum inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold",
        cls,
        className,
      )}
    >
      {score}分
    </span>
  );
}
