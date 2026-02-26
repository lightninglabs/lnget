"use client";

import styled from "@emotion/styled";
import PageHeader from "@/components/PageHeader";
import StatTile from "@/components/StatTile";
import ErrorBanner from "@/components/ErrorBanner";
import { useStatus, useConfig } from "@/lib/api";

const Styled = {
  Grid: styled.div`
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 16px;
    margin-bottom: 32px;
  `,
  Section: styled.div`
    margin-bottom: 32px;
    animation: fade-in-up 0.4s ease-out both;
  `,
  SectionTitle: styled.h2`
    font-family: ${(p) => p.theme.fonts.work};
    font-size: ${(p) => p.theme.sizes.s}px;
    font-weight: 500;
    margin: 0 0 16px;
    color: ${(p) => p.theme.colors.offWhite};
  `,
  Card: styled.div`
    background-color: ${(p) => p.theme.colors.lightNavy};
    border: 1px solid ${(p) => p.theme.colors.lightningBlack};
    border-radius: 8px;
    padding: 24px;
  `,
  Row: styled.div`
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 0;
    border-bottom: 1px solid ${(p) => p.theme.colors.blue};

    &:last-child {
      border-bottom: none;
    }
  `,
  Label: styled.span`
    color: ${(p) => p.theme.colors.gray};
    font-size: 13px;
    font-weight: 500;
  `,
  Value: styled.span`
    color: ${(p) => p.theme.colors.offWhite};
    font-size: 13px;
    font-weight: 600;
  `,
  Mono: styled.span`
    font-family: monospace;
    font-size: 12px;
    color: ${(p) => p.theme.colors.offWhite};
    word-break: break-all;
  `,
  StatusIndicator: styled.span<{ $connected: boolean }>`
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 600;
    color: ${(p) =>
      p.$connected
        ? p.theme.colors.lightningGreen
        : p.theme.colors.lightningRed};
  `,
  Dot: styled.span<{ $connected: boolean }>`
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: ${(p) =>
      p.$connected
        ? p.theme.colors.lightningGreen
        : p.theme.colors.lightningRed};
    box-shadow: 0 0 6px
      ${(p) =>
        p.$connected
          ? p.theme.colors.lightningGreen
          : p.theme.colors.lightningRed};
  `,
  TwoCol: styled.div`
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  `,
  Badge: styled.span`
    display: inline-block;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    background: rgba(93, 95, 239, 0.15);
    color: ${(p) => p.theme.colors.lightPurple};
  `,
};

export default function StatusPage() {
  const { data: status, error: statusError, mutate: mutateStatus } = useStatus();
  const { data: config, error: configError } = useConfig();

  const error = statusError || configError;

  return (
    <>
      <PageHeader
        title="Status"
        description="Lightning backend and configuration overview."
      />

      {error && (
        <ErrorBanner
          message="Failed to load status. Is `lnget serve` running?"
          onRetry={() => mutateStatus()}
        />
      )}

      <div className="animate-in">
        <Styled.Grid>
          <StatTile
            title="Backend Type"
            text={status?.type?.toUpperCase() || "—"}
          />
          <StatTile
            title="Connection"
            text={
              status?.connected ? "Connected" : "Disconnected"
            }
            subText={status?.error || undefined}
          />
          <StatTile
            title="Wallet Balance"
            text={
              status?.connected && status.balance_sat != null
                ? status.balance_sat.toLocaleString()
                : "—"
            }
            suffix={status?.connected ? "sats" : undefined}
          />
        </Styled.Grid>
      </div>

      <Styled.TwoCol>
        <Styled.Section>
          <Styled.SectionTitle>Lightning Backend</Styled.SectionTitle>
          <Styled.Card>
            <Styled.Row>
              <Styled.Label>Status</Styled.Label>
              <Styled.StatusIndicator $connected={!!status?.connected}>
                <Styled.Dot $connected={!!status?.connected} />
                {status?.connected ? "Connected" : "Disconnected"}
              </Styled.StatusIndicator>
            </Styled.Row>
            <Styled.Row>
              <Styled.Label>Backend Type</Styled.Label>
              <Styled.Value>
                <Styled.Badge>{status?.type || "—"}</Styled.Badge>
              </Styled.Value>
            </Styled.Row>
            {status?.node_pubkey && (
              <Styled.Row>
                <Styled.Label>Node Pubkey</Styled.Label>
                <Styled.Mono>{status.node_pubkey}</Styled.Mono>
              </Styled.Row>
            )}
            {status?.alias && (
              <Styled.Row>
                <Styled.Label>Alias</Styled.Label>
                <Styled.Value>{status.alias}</Styled.Value>
              </Styled.Row>
            )}
            {status?.network && (
              <Styled.Row>
                <Styled.Label>Network</Styled.Label>
                <Styled.Value>{status.network}</Styled.Value>
              </Styled.Row>
            )}
            {status?.synced_to_chain != null && (
              <Styled.Row>
                <Styled.Label>Synced to Chain</Styled.Label>
                <Styled.Value>
                  {status.synced_to_chain ? "Yes" : "No"}
                </Styled.Value>
              </Styled.Row>
            )}
            {status?.error && (
              <Styled.Row>
                <Styled.Label>Error</Styled.Label>
                <Styled.Value style={{ color: "#EF4444" }}>
                  {status.error}
                </Styled.Value>
              </Styled.Row>
            )}
          </Styled.Card>
        </Styled.Section>

        <Styled.Section>
          <Styled.SectionTitle>Configuration</Styled.SectionTitle>
          <Styled.Card>
            <Styled.Row>
              <Styled.Label>Max Auto-Pay</Styled.Label>
              <Styled.Value>
                {config
                  ? `${config.max_cost_sats.toLocaleString()} sats`
                  : "—"}
              </Styled.Value>
            </Styled.Row>
            <Styled.Row>
              <Styled.Label>Max Fee</Styled.Label>
              <Styled.Value>
                {config
                  ? `${config.max_fee_sats.toLocaleString()} sats`
                  : "—"}
              </Styled.Value>
            </Styled.Row>
            <Styled.Row>
              <Styled.Label>Payment Timeout</Styled.Label>
              <Styled.Value>
                {config?.payment_timeout || "—"}
              </Styled.Value>
            </Styled.Row>
            <Styled.Row>
              <Styled.Label>Auto Pay</Styled.Label>
              <Styled.Value>
                {config ? (config.auto_pay ? "Enabled" : "Disabled") : "—"}
              </Styled.Value>
            </Styled.Row>
            <Styled.Row>
              <Styled.Label>Event Logging</Styled.Label>
              <Styled.Value>
                {config
                  ? config.events_enabled
                    ? "Enabled"
                    : "Disabled"
                  : "—"}
              </Styled.Value>
            </Styled.Row>
            <Styled.Row>
              <Styled.Label>Token Directory</Styled.Label>
              <Styled.Mono>{config?.token_dir || "—"}</Styled.Mono>
            </Styled.Row>
          </Styled.Card>
        </Styled.Section>
      </Styled.TwoCol>
    </>
  );
}
