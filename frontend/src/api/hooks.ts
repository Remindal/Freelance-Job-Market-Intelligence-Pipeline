import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { toast } from "sonner";

import {
  GetJob,
  GetStats,
  ListJobs,
  RunNow,
  UpdateStatus,
} from "../wailsjs/go/desktop/App";
import { desktop, domain } from "../wailsjs/go/models";
import type { JobsFilterInput } from "./types";

export function useJobs(filter: JobsFilterInput) {
  return useQuery({
    queryKey: ["jobs", filter],
    queryFn: () => ListJobs(new desktop.ListFilter({ ...filter })),
    refetchInterval: 30000,
  });
}

export function useJob(id: number) {
  return useQuery({
    queryKey: ["job", id],
    queryFn: () => GetJob(id),
    enabled: id > 0,
  });
}

export function useStats() {
  return useQuery({
    queryKey: ["stats"],
    queryFn: () => GetStats(),
    refetchInterval: 30000,
  });
}

// 状态变更：乐观更新所有列表缓存，失败回滚 + toast
export function useUpdateStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: number; status: string }) =>
      UpdateStatus(id, status),
    onMutate: async ({ id, status }) => {
      await qc.cancelQueries({ queryKey: ["jobs"] });
      const prev = qc.getQueriesData<desktop.JobListResult>({
        queryKey: ["jobs"],
      });
      qc.setQueriesData<desktop.JobListResult>(
        { queryKey: ["jobs"] },
        (old) =>
          old
            ? desktop.JobListResult.createFrom({
                ...old,
                items: old.items.map((j) =>
                  j.id === id ? domain.Job.createFrom({ ...j, status }) : j,
                ),
              })
            : old,
      );
      return { prev };
    },
    onError: (err, _vars, ctx) => {
      ctx?.prev?.forEach(([key, data]) => qc.setQueryData(key, data));
      toast.error("状态更新失败: " + String(err));
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ["jobs"] });
      qc.invalidateQueries({ queryKey: ["job"] });
      qc.invalidateQueries({ queryKey: ["stats"] });
    },
  });
}

export function useRunNow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => RunNow(),
    onSuccess: (data) => {
      // 三态反馈：成功N条 / 成功0条 / 失败+原因
      if (data.success) {
        toast.success(data.message);
      } else {
        toast.error(data.message);
      }
      qc.invalidateQueries();
    },
    onError: (err) => toast.error(String(err)),
  });
}
