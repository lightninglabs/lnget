export interface Event {
  id: number;
  domain: string;
  url: string;
  method: string;
  payment_hash: string;
  amount_sat: number;
  fee_sat: number;
  status: string;
  error_message?: string;
  duration_ms: number;
  content_type?: string;
  response_size?: number;
  status_code?: number;
  created_at: string;
}

export interface TokenInfo {
  domain: string;
  payment_hash: string;
  amount_sat: number;
  fee_sat: number;
  created: string;
  pending: boolean;
}

export interface Stats {
  total_spent_sat: number;
  total_fees_sat: number;
  total_payments: number;
  failed_payments: number;
  active_tokens: number;
  domains_accessed: number;
}

export interface DomainSpending {
  domain: string;
  total_sat: number;
  total_fees: number;
  payment_count: number;
  last_used: string;
}

export interface BackendStatus {
  type: string;
  connected: boolean;
  node_pubkey?: string;
  alias?: string;
  network?: string;
  synced_to_chain?: boolean;
  balance_sat?: number;
  error?: string;
}

export interface ConfigInfo {
  ln_mode: string;
  max_cost_sats: number;
  max_fee_sats: number;
  payment_timeout: string;
  auto_pay: boolean;
  events_enabled: boolean;
  token_dir: string;
}
