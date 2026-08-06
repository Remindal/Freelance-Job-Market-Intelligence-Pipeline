import { useEffect, useState } from "react";
import { toast } from "sonner";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { CancelFetch, GetProgress } from "../wailsjs/go/desktop/App";

export interface ProgressEvent {
  stage: string; // fetch_start | feed_done | score | done
  feed: string;
  feed_index: number;
  feed_total: number;
  feed_jobs: number;
  score_done: number;
  score_total: number;
  new: number;
  fetched: number;
}

// 抓取进度条：订阅后端 jobs:progress 事件，done 后短暂停留再消失
export function FetchProgress() {
  const [prog, setProg] = useState<ProgressEvent | null>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    let hideTimer: ReturnType<typeof setTimeout>;
    // 挂载时先拉一次当前状态：错过 fetch_start 事件也能显示进行中
    GetProgress()
      .then((v) => {
        if (v.active) {
          setProg(v as unknown as ProgressEvent);
          setVisible(true);
        }
      })
      .catch(() => {});
    const off = EventsOn("jobs:progress", (p: ProgressEvent) => {
      setProg(p);
      setVisible(true);
      clearTimeout(hideTimer);
      if (p.stage === "done") {
        hideTimer = setTimeout(() => setVisible(false), 4000);
      }
    });
    return () => {
      off();
      clearTimeout(hideTimer);
    };
  }, []);

  if (!visible || !prog) return null;

  let text = "正在抓取…";
  let percent = -1; // -1 = 不定态
  switch (prog.stage) {
    case "fetch_start":
      text = "正在抓取…";
      break;
    case "feed_done":
      text = `已抓完「${prog.feed}」源（${prog.feed_jobs} 条） ${prog.feed_index}/${prog.feed_total}`;
      percent = prog.feed_total > 0 ? (prog.feed_index / prog.feed_total) * 40 : -1;
      break;
    case "score":
      text = `LLM 打分中 ${prog.score_done}/${prog.score_total}`;
      percent = 40 + (prog.score_total > 0 ? (prog.score_done / prog.score_total) * 55 : 0);
      break;
    case "done":
      text = `本轮完成：抓取 ${prog.fetched} 条，新单 ${prog.new} 条`;
      percent = 100;
      break;
  }

  return (
    <div className="mb-3 rounded-lg border border-primary/20 bg-primary-light/50 px-4 py-2.5">
      <div className="flex items-center gap-2 text-xs text-primary">
        <span
          className={
            "inline-block h-2 w-2 rounded-full bg-primary" +
            (prog.stage === "done" ? "" : " animate-pulse")
          }
        />
        <span className="flex-1">{text}</span>
        {prog.stage !== "done" && (
          <button
            type="button"
            className="rounded border border-primary/30 px-2 py-0.5 text-[11px] text-primary hover:bg-primary/10"
            onClick={() => {
              CancelFetch()
                .then((msg) => toast.info(msg))
                .catch((e) => toast.error(String(e)));
            }}
          >
            取消抓取
          </button>
        )}
      </div>
      <div className="mt-1.5 h-1 overflow-hidden rounded-full bg-primary/10">
        {percent >= 0 ? (
          <div
            className="h-full rounded-full bg-primary transition-all duration-500"
            style={{ width: `${percent}%` }}
          />
        ) : (
          <div className="h-full w-1/3 animate-pulse rounded-full bg-primary/60" />
        )}
      </div>
    </div>
  );
}
