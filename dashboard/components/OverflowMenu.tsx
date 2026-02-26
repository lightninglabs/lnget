"use client";

import React, {
  useRef,
  useState,
  useEffect,
  useLayoutEffect,
  useCallback,
} from "react";
import { createPortal } from "react-dom";
import styled from "@emotion/styled";

export interface MenuItem {
  label: string;
  onClick: () => void;
  danger?: boolean;
}

interface Props {
  items: MenuItem[];
}

const sfp = {
  shouldForwardProp: (prop: string) => !prop.startsWith("$"),
};

const Styled = {
  Trigger: styled.button`
    background: transparent;
    border: none;
    color: ${(p) => p.theme.colors.gray};
    cursor: pointer;
    font-size: 18px;
    padding: 4px 8px;
    border-radius: 4px;
    line-height: 1;
    letter-spacing: 2px;
    transition: all 0.15s;

    &:hover {
      background: ${(p) => p.theme.colors.overlay};
      color: ${(p) => p.theme.colors.white};
    }
  `,
  Menu: styled("div", sfp)<{ $visible: boolean }>`
    position: fixed;
    z-index: 9999;
    min-width: 140px;
    background: ${(p) => p.theme.colors.lightNavy};
    border: 1px solid ${(p) => p.theme.colors.lightBlue};
    border-radius: 6px;
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.6);
    overflow: hidden;
    visibility: ${(p) => (p.$visible ? "visible" : "hidden")};
    opacity: ${(p) => (p.$visible ? 1 : 0)};
    transition: opacity 0.12s ease;
  `,
  Item: styled("button", sfp)<{ $danger?: boolean }>`
    display: block;
    width: 100%;
    padding: 10px 16px;
    background: transparent;
    border: none;
    color: ${(p) =>
      p.$danger ? p.theme.colors.lightningRed : p.theme.colors.offWhite};
    font-size: 13px;
    font-family: ${(p) => p.theme.fonts.open};
    font-weight: 500;
    text-align: left;
    cursor: pointer;
    transition: background-color 0.1s;

    &:hover {
      background: ${(p) => p.theme.colors.overlay};
    }
  `,
};

const OverflowMenu: React.FC<Props> = ({ items }) => {
  const [open, setOpen] = useState(false);
  const [visible, setVisible] = useState(false);
  const [pos, setPos] = useState({ top: 0, left: 0 });
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const toggle = useCallback(() => setOpen((p) => !p), []);

  useLayoutEffect(() => {
    if (!open || !triggerRef.current || !menuRef.current) return;
    const rect = triggerRef.current.getBoundingClientRect();
    const menuRect = menuRef.current.getBoundingClientRect();
    let left = rect.right - menuRect.width;
    if (left < 8) left = rect.left;
    setPos({ top: rect.bottom + 4, left });
    setVisible(true);
  }, [open]);

  useEffect(() => {
    if (!open) {
      setVisible(false);
      return;
    }
    const handler = (e: MouseEvent) => {
      if (
        menuRef.current &&
        !menuRef.current.contains(e.target as Node) &&
        triggerRef.current &&
        !triggerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  return (
    <>
      <Styled.Trigger ref={triggerRef} onClick={toggle}>
        &middot;&middot;&middot;
      </Styled.Trigger>
      {open &&
        createPortal(
          <Styled.Menu
            ref={menuRef}
            $visible={visible}
            style={{ top: pos.top, left: pos.left }}
          >
            {items.map((item) => (
              <Styled.Item
                key={item.label}
                $danger={item.danger}
                onClick={() => {
                  item.onClick();
                  setOpen(false);
                }}
              >
                {item.label}
              </Styled.Item>
            ))}
          </Styled.Menu>,
          document.body,
        )}
    </>
  );
};

export default OverflowMenu;
