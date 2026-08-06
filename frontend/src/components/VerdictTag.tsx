import { cn } from "../lib/utils";

const verdictCls: Record<string, string> = {
  强烈推荐: "bg-[#dcfce7] text-[#15803d]",
  可投: "bg-[#dbeafe] text-[#1d4ed8]",
  观望: "bg-neutral-100 text-neutral-500",
  不推荐: "bg-[#fee2e2] text-danger",
};

export function VerdictTag({ verdict, className }: { verdict: string; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold",
        verdictCls[verdict] ?? verdictCls["观望"],
        className,
      )}
    >
      {verdict}
    </span>
  );
}
