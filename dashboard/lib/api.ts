import useSWR, { mutate } from "swr";
import type {
  Event,
  TokenInfo,
  Stats,
  DomainSpending,
  BackendStatus,
  ConfigInfo,
} from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:2402";

async function fetcher<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

export function useEvents(opts?: {
  limit?: number;
  offset?: number;
  domain?: string;
  status?: string;
}) {
  const params = new URLSearchParams();
  if (opts?.limit) params.set("limit", String(opts.limit));
  if (opts?.offset) params.set("offset", String(opts.offset));
  if (opts?.domain) params.set("domain", opts.domain);
  if (opts?.status) params.set("status", opts.status);
  const qs = params.toString();
  const key = `/api/events${qs ? `?${qs}` : ""}`;

  return useSWR<Event[]>(key, fetcher, { refreshInterval: 10_000 });
}

export function useStats() {
  return useSWR<Stats>("/api/events/stats", fetcher, {
    refreshInterval: 10_000,
  });
}

export function useDomains() {
  return useSWR<DomainSpending[]>("/api/events/domains", fetcher, {
    refreshInterval: 30_000,
  });
}

export function useTokens() {
  return useSWR<TokenInfo[]>("/api/tokens", fetcher, {
    refreshInterval: 30_000,
  });
}

export function useStatus() {
  return useSWR<BackendStatus>("/api/status", fetcher, {
    refreshInterval: 60_000,
  });
}

export function useConfig() {
  return useSWR<ConfigInfo>("/api/config", fetcher, {
    refreshInterval: 60_000,
  });
}

export async function removeToken(domain: string) {
  const res = await fetch(
    `${API_BASE}/api/tokens/${encodeURIComponent(domain)}`,
    { method: "DELETE" },
  );
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "request failed" }));
    throw new Error(err.error ?? `API error: ${res.status}`);
  }
  await mutate("/api/tokens");
  return res.json();
}
