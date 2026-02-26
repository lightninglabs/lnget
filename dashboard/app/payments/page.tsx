"use client";

import { useState, useMemo } from "react";
import styled from "@emotion/styled";
import PageHeader from "@/components/PageHeader";
import StatTile from "@/components/StatTile";
import ErrorBanner from "@/components/ErrorBanner";
import PaymentVolumeChart from "@/components/PaymentVolumeChart";
import Button from "@/components/Button";
import { useEvents, useStats } from "@/lib/api";

const Styled = {
  Filters: styled.div`
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 24px;
    flex-wrap: wrap;
  `,
  FilterLabel: styled.label`
    font-size: 12px;
    color: ${(p) => p.theme.colors.gray};
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  `,
  FilterSelect: styled.select`
    background: ${(p) => p.theme.colors.lightNavy};
    border: 1px solid ${(p) => p.theme.colors.lightBlue};
    color: ${(p) => p.theme.colors.offWhite};
    padding: 6px 12px;
    border-radius: 6px;
    font-size: 13px;
    font-family: ${(p) => p.theme.fonts.open};
    cursor: pointer;

    &:focus {
      outline: none;
      border-color: ${(p) => p.theme.colors.purple};
    }
  `,
  FilterInput: styled.input`
    background: ${(p) => p.theme.colors.lightNavy};
    border: 1px solid ${(p) => p.theme.colors.lightBlue};
    color: ${(p) => p.theme.colors.offWhite};
    padding: 6px 12px;
    border-radius: 6px;
    font-size: 13px;
    font-family: ${(p) => p.theme.fonts.open};
    width: 160px;

    &:focus {
      outline: none;
      border-color: ${(p) => p.theme.colors.purple};
    }

    &::placeholder {
      color: ${(p) => p.theme.colors.gray};
    }
  `,
  StatsRow: styled.div`
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 16px;
    margin-bottom: 24px;
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
    overflow: hidden;
  `,
  Table: styled.table`
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  `,
  Th: styled.th`
    text-align: left;
    padding: 12px 16px;
    color: ${(p) => p.theme.colors.gray};
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    border-bottom: 1px solid ${(p) => p.theme.colors.blue};
  `,
  Td: styled.td`
    padding: 12px 16px;
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
  Pagination: styled.div`
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-top: 1px solid ${(p) => p.theme.colors.blue};
    font-size: 13px;
    color: ${(p) => p.theme.colors.gray};
  `,
  Mono: styled.span`
    font-family: monospace;
    font-size: 12px;
    color: ${(p) => p.theme.colors.gray};
  `,
};

const PAGE_SIZE = 25;

export default function PaymentsPage() {
  const [page, setPage] = useState(0);
  const [domainFilter, setDomainFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");

  const {
    data: events,
    error,
    mutate,
  } = useEvents({
    limit: PAGE_SIZE,
    offset: page * PAGE_SIZE,
    domain: domainFilter || undefined,
    status: statusFilter || undefined,
  });

  const { data: allEvents } = useEvents({ limit: 200 });
  const { data: stats } = useStats();

  const successRate = useMemo(() => {
    if (!stats || stats.total_payments === 0) return "—";
    const total = stats.total_payments + stats.failed_payments;
    const rate = (stats.total_payments / total) * 100;
    return `${rate.toFixed(1)}%`;
  }, [stats]);

  return (
    <>
      <PageHeader
        title="Payments"
        description="Full history of your L402 payment events."
      />

      {error && (
        <ErrorBanner
          message="Failed to load payments. Is `lnget serve` running?"
          onRetry={() => mutate()}
        />
      )}

      <div className="animate-in">
        <Styled.StatsRow>
          <StatTile
            title="Total Payments"
            text={stats ? stats.total_payments.toLocaleString() : "—"}
          />
          <StatTile
            title="Total Spent"
            text={stats ? stats.total_spent_sat.toLocaleString() : "—"}
            suffix="sats"
          />
          <StatTile title="Success Rate" text={successRate} />
        </Styled.StatsRow>
      </div>

      {allEvents && allEvents.length > 0 && (
        <Styled.Section>
          <Styled.SectionTitle>Payment Volume</Styled.SectionTitle>
          <Styled.Card style={{ padding: 16 }}>
            <PaymentVolumeChart events={allEvents} />
          </Styled.Card>
        </Styled.Section>
      )}

      <Styled.Filters>
        <Styled.FilterLabel>Filters:</Styled.FilterLabel>
        <Styled.FilterInput
          type="text"
          placeholder="Domain..."
          value={domainFilter}
          onChange={(e) => {
            setDomainFilter(e.target.value);
            setPage(0);
          }}
        />
        <Styled.FilterSelect
          value={statusFilter}
          onChange={(e) => {
            setStatusFilter(e.target.value);
            setPage(0);
          }}
        >
          <option value="">All statuses</option>
          <option value="success">Success</option>
          <option value="failed">Failed</option>
        </Styled.FilterSelect>
      </Styled.Filters>

      <Styled.Card>
        <Styled.Table>
          <thead>
            <tr>
              <Styled.Th>Domain</Styled.Th>
              <Styled.Th>URL</Styled.Th>
              <Styled.Th>Amount</Styled.Th>
              <Styled.Th>Fee</Styled.Th>
              <Styled.Th>Status</Styled.Th>
              <Styled.Th>Duration</Styled.Th>
              <Styled.Th>Time</Styled.Th>
            </tr>
          </thead>
          <tbody>
            {(events || []).map((evt) => (
              <tr key={evt.id}>
                <Styled.Td>{evt.domain}</Styled.Td>
                <Styled.Td>
                  <Styled.Mono>
                    {evt.method} {evt.url || "—"}
                  </Styled.Mono>
                </Styled.Td>
                <Styled.Td>
                  {evt.amount_sat.toLocaleString()} sats
                </Styled.Td>
                <Styled.Td>
                  {evt.fee_sat.toLocaleString()} sats
                </Styled.Td>
                <Styled.Td>
                  <Styled.StatusBadge $status={evt.status}>
                    {evt.status}
                  </Styled.StatusBadge>
                </Styled.Td>
                <Styled.Td>{evt.duration_ms}ms</Styled.Td>
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
            {events && events.length === 0 && (
              <tr>
                <Styled.Td
                  colSpan={7}
                  style={{ textAlign: "center", color: "#848a99" }}
                >
                  No payments found.
                </Styled.Td>
              </tr>
            )}
          </tbody>
        </Styled.Table>

        <Styled.Pagination>
          <span>
            Page {page + 1}
            {events && events.length === PAGE_SIZE && "+"}
          </span>
          <div style={{ display: "flex", gap: 8 }}>
            <Button
              variant="ghost"
              compact
              disabled={page === 0}
              onClick={() => setPage((p) => Math.max(0, p - 1))}
            >
              Previous
            </Button>
            <Button
              variant="ghost"
              compact
              disabled={!events || events.length < PAGE_SIZE}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
            </Button>
          </div>
        </Styled.Pagination>
      </Styled.Card>
    </>
  );
}
