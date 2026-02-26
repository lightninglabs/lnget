"use client";

import React, { useState } from "react";
import styled from "@emotion/styled";
import PageHeader from "@/components/PageHeader";
import ErrorBanner from "@/components/ErrorBanner";
import OverflowMenu from "@/components/OverflowMenu";
import { toast } from "@/components/Toast";
import { useTokens, removeToken } from "@/lib/api";

const Styled = {
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
    padding: 14px 16px;
    border-bottom: 1px solid ${(p) => p.theme.colors.blue};
    color: ${(p) => p.theme.colors.offWhite};
    vertical-align: top;
  `,
  StatusBadge: styled.span<{ $pending: boolean }>`
    display: inline-block;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    ${(p) =>
      p.$pending
        ? `
      background: rgba(245,158,11,0.15);
      color: ${p.theme.colors.lightningYellow};
    `
        : `
      background: rgba(16,185,129,0.15);
      color: ${p.theme.colors.lightningGreen};
    `}
  `,
  ExpandBtn: styled.button`
    background: transparent;
    border: none;
    color: ${(p) => p.theme.colors.lightningBlue};
    font-size: 12px;
    font-family: ${(p) => p.theme.fonts.open};
    cursor: pointer;
    padding: 0;
    margin-top: 4px;

    &:hover {
      text-decoration: underline;
    }
  `,
  Detail: styled.div`
    padding: 8px 16px 14px;
    font-size: 12px;
    color: ${(p) => p.theme.colors.gray};
    background: ${(p) => p.theme.colors.lightningBlack};
    border-bottom: 1px solid ${(p) => p.theme.colors.blue};
  `,
  DetailLabel: styled.span`
    color: ${(p) => p.theme.colors.gray};
    margin-right: 8px;
  `,
  DetailValue: styled.span`
    color: ${(p) => p.theme.colors.offWhite};
    font-family: monospace;
    font-size: 12px;
    word-break: break-all;
  `,
  Empty: styled.div`
    text-align: center;
    padding: 60px 20px;
    color: ${(p) => p.theme.colors.gray};
    font-size: 14px;
  `,
};

export default function TokensPage() {
  const { data: tokens, error, mutate } = useTokens();
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const toggleExpand = (domain: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(domain)) {
        next.delete(domain);
      } else {
        next.add(domain);
      }
      return next;
    });
  };

  const handleRemove = async (domain: string) => {
    try {
      await removeToken(domain);
      toast(`Removed token for ${domain}`, "success");
      mutate();
    } catch (err) {
      toast(
        `Failed to remove token: ${err instanceof Error ? err.message : "unknown error"}`,
        "error",
      );
    }
  };

  return (
    <>
      <PageHeader
        title="Tokens"
        description="Cached L402 tokens for authenticated domains."
      />

      {error && (
        <ErrorBanner
          message="Failed to load tokens. Is `lnget serve` running?"
          onRetry={() => mutate()}
        />
      )}

      {tokens && tokens.length === 0 && (
        <Styled.Empty>
          No tokens cached. Use lnget to access L402-protected resources
          and tokens will appear here.
        </Styled.Empty>
      )}

      {tokens && tokens.length > 0 && (
        <Styled.Card>
          <Styled.Table>
            <thead>
              <tr>
                <Styled.Th>Domain</Styled.Th>
                <Styled.Th>Amount</Styled.Th>
                <Styled.Th>Fee</Styled.Th>
                <Styled.Th>Created</Styled.Th>
                <Styled.Th>Status</Styled.Th>
                <Styled.Th />
              </tr>
            </thead>
            <tbody>
              {tokens.map((token) => (
                <React.Fragment key={token.domain}>
                  <tr>
                    <Styled.Td>
                      {token.domain}
                      <br />
                      <Styled.ExpandBtn
                        onClick={() => toggleExpand(token.domain)}
                      >
                        {expanded.has(token.domain)
                          ? "Hide details"
                          : "Show details"}
                      </Styled.ExpandBtn>
                    </Styled.Td>
                    <Styled.Td>
                      {token.amount_sat.toLocaleString()} sats
                    </Styled.Td>
                    <Styled.Td>
                      {token.fee_sat.toLocaleString()} sats
                    </Styled.Td>
                    <Styled.Td>{token.created}</Styled.Td>
                    <Styled.Td>
                      <Styled.StatusBadge $pending={token.pending}>
                        {token.pending ? "pending" : "paid"}
                      </Styled.StatusBadge>
                    </Styled.Td>
                    <Styled.Td>
                      <OverflowMenu
                        items={[
                          {
                            label: "Remove token",
                            onClick: () => handleRemove(token.domain),
                            danger: true,
                          },
                        ]}
                      />
                    </Styled.Td>
                  </tr>
                  {expanded.has(token.domain) && (
                    <tr>
                      <td colSpan={6}>
                        <Styled.Detail>
                          <Styled.DetailLabel>
                            Payment Hash:
                          </Styled.DetailLabel>
                          <Styled.DetailValue>
                            {token.payment_hash}
                          </Styled.DetailValue>
                        </Styled.Detail>
                      </td>
                    </tr>
                  )}
                </React.Fragment>
              ))}
            </tbody>
          </Styled.Table>
        </Styled.Card>
      )}
    </>
  );
}
