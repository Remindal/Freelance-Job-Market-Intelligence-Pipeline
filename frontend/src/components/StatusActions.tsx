import { Button } from "./ui/button";
import { useUpdateStatus } from "../api/hooks";

const actions: { status: string; label: string }[] = [
  { status: "want", label: "想投" },
  { status: "proposed", label: "已投" },
  { status: "ignored", label: "忽略" },
];

export function StatusActions({
  id,
  current,
  size = "sm",
}: {
  id: number;
  current: string;
  size?: "sm" | "md";
}) {
  const mutation = useUpdateStatus();
  return (
    <div className="flex flex-wrap gap-1.5">
      {actions
        .filter((a) => a.status !== current)
        .map((a) => (
          <Button
            key={a.status}
            variant="outline"
            size={size}
            disabled={mutation.isPending}
            onClick={(e) => {
              e.stopPropagation();
              mutation.mutate({ id, status: a.status });
            }}
          >
            {a.label}
          </Button>
        ))}
      {current !== "new" && (
        <Button
          variant="ghost"
          size={size}
          disabled={mutation.isPending}
          onClick={(e) => {
            e.stopPropagation();
            mutation.mutate({ id, status: "new" });
          }}
        >
          重置
        </Button>
      )}
    </div>
  );
}
