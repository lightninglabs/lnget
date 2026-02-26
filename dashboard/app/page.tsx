"use client";

import styled from "@emotion/styled";
import PageHeader from "@/components/PageHeader";
import StatTile from "@/components/StatTile";
import ErrorBanner from "@/components/ErrorBanner";
import SpendingChart from "@/components/SpendingChart";
import DomainChart from "@/components/DomainChart";
import { useStats, useEvents, useDomains, useStatus } from "@/lib/api";

const Styled = {
  Grid: styled.div`
    display: grid;
    grid-template-columns: repeat(4, 1fr);
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
    padding: 20px;
  `,
  TwoCol: styled.div`
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
    margin-bottom: 32px;
  `,
  Table: styled.table`
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  `,
  Th: styled.th`
    text-align: left;
    padding: 8px 12px;
    color: ${(p) => p.theme.colors.gray};
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    border-bottom: 1px solid ${(p) => p.theme.colors.blue};
  `,
  Td: styled.td`
    padding: 10px 12px;
    border-bottom: 1px solid ${(p) => p.theme.colors.blue};
    color: ${(p) => p.theme.colors.offWhite};
  `,
  StatusBadge: styled.span<{ $status: string }>`
    display: inline-block;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    ${(p) =>
      p.$status === "success"
        ? `
      background: rgba(16,185,129,0.15);
      color: ${p.theme.colors.lightningGreen};
    `
        : `
      background: rgba(239,68,68,0.15);
      color: ${p.theme.colors.lightningRed};
    `}
  `,
  Welcome: styled.div`
    text-align: center;
    padding: 60px 20px;
    animation: fade-in-up 0.4s ease-out both;
  `,
  WelcomeTitle: styled.h2`
    font-family: ${(p) => p.theme.fonts.work};
    font-size: ${(p) => p.theme.sizes.xl}px;
    font-weight: 300;
    margin: 0 0 12px;
  `,
  WelcomeText: styled.p`
    color: ${(p) => p.theme.colors.gray};
    font-size: ${(p) => p.theme.sizes.s}px;
    margin: 0;
    max-width: 500px;
    margin-left: auto;
    margin-right: auto;
    line-height: 1.6;
  `,
};

export default function DashboardPage() {
  const { data: stats, error: statsError } = useStats();
  const { data: recentEvents, error: eventsError } = useEvents({ limit: 5 });
  const { data: chartEvents } = useEvents({ limit: 200 });
  const { data: domains, error: domainsError } = useDomains();
  const { data: status } = useStatus();

  const error = statsError || eventsError || domainsError;

  const hasData = stats && stats.total_payments > 0;

  return (
    <>
      <PageHeader
        title="Dashboard"
        description="Your L402 spending overview and recent activity."
      />

      {error && (
        <ErrorBanner
          message="Failed to load dashboard data. Is `lnget serve` running?"
          onRetry={() => window.location.reload()}
        />
      )}

      <div className="animate-in">
        <Styled.Grid>
          <StatTile
            title="Total Spent"
            text={stats ? stats.total_spent_sat.toLocaleString() : "—"}
            suffix="sats"
            subText={
              stats && stats.total_fees_sat > 0
                ? `${stats.total_fees_sat.toLocaleString()} sats in fees`
                : undefined
            }
          />
          <StatTile
            title="Total Payments"
            text={stats ? stats.total_payments.toLocaleString() : "—"}
            subText={
              stats && stats.failed_payments > 0
                ? `${stats.failed_payments} failed`
                : undefined
            }
          />
          <StatTile
            title="Active Tokens"
            text={stats ? String(stats.active_tokens) : "—"}
            subText={
              stats
                ? `${stats.domains_accessed} domains accessed`
                : undefined
            }
          />
          <StatTile
            title="Wallet Balance"
            text={
              status?.connected && status.balance_sat != null
                ? status.balance_sat.toLocaleString()
                : "—"
            }
            suffix={status?.connected ? "sats" : undefined}
            subText={
              status?.connected
                ? status.alias || status.type
                : "Not connected"
            }
          />
        </Styled.Grid>
      </div>

      {!hasData && !error && (
        <Styled.Welcome>
          <Styled.WelcomeTitle>Welcome to lnget</Styled.WelcomeTitle>
          <Styled.WelcomeText>
            No payments recorded yet. Use lnget to access L402-protected
            resources and your spending activity will appear here.
          </Styled.WelcomeText>
        </Styled.Welcome>
      )}

      {hasData && (
        <>
          <Styled.Section>
            <Styled.SectionTitle>Spending Over Time</Styled.SectionTitle>
            <Styled.Card>
              <SpendingChart events={chartEvents || []} />
            </Styled.Card>
          </Styled.Section>

          <Styled.TwoCol>
            <Styled.Section>
              <Styled.SectionTitle>
                Spending by Domain
              </Styled.SectionTitle>
              <Styled.Card>
                <DomainChart data={domains || []} />
              </Styled.Card>
            </Styled.Section>

            <Styled.Section>
              <Styled.SectionTitle>Recent Payments</Styled.SectionTitle>
              <Styled.Card>
                <Styled.Table>
                  <thead>
                    <tr>
                      <Styled.Th>Domain</Styled.Th>
                      <Styled.Th>Amount</Styled.Th>
                      <Styled.Th>Status</Styled.Th>
                      <Styled.Th>Time</Styled.Th>
                    </tr>
                  </thead>
                  <tbody>
                    {(recentEvents || []).map((evt) => (
                      <tr key={evt.id}>
                        <Styled.Td>{evt.domain}</Styled.Td>
                        <Styled.Td>
                          {evt.amount_sat.toLocaleString()} sats
                        </Styled.Td>
                        <Styled.Td>
                          <Styled.StatusBadge $status={evt.status}>
                            {evt.status}
                          </Styled.StatusBadge>
                        </Styled.Td>
                        <Styled.Td>
                          {new Date(evt.created_at).toLocaleString(undefined, {
                            month: "short",
                            day: "numeric",
                            hour: "numeric",
                            minute: "2-digit",
                          })}
                        </Styled.Td>
                      </tr>
                    ))}
                    {(!recentEvents || recentEvents.length === 0) && (
                      <tr>
                        <Styled.Td
                          colSpan={4}
                          style={{ textAlign: "center", color: "#848a99" }}
                        >
                          No recent payments.
                        </Styled.Td>
                      </tr>
                    )}
                  </tbody>
                </Styled.Table>
              </Styled.Card>
            </Styled.Section>
          </Styled.TwoCol>
        </>
      )}
    </>
  );
}
