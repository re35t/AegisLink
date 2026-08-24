import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  ComposerPrimitive,
  type Unstable_TriggerItem,
  unstable_useMentionAdapter,
  unstable_useTriggerPopoverTriggers,
  useAui,
} from "@assistant-ui/react";
import { ArrowLeft, ChevronRight, PlugZap, Wrench, X } from "lucide-react";

import { api, type MentionItem } from "../api/client";
import { queryClient } from "../app/queryClient";

export interface SelectedMention {
  id: string;
  action: "force-tool-once";
  label: string;
  groupLabel: string;
}

interface MentionCatalogProps {
  agentId: string;
  selected?: SelectedMention;
  onSelect(item: SelectedMention): void;
  onClear(): void;
  children: React.ReactNode;
}

interface MentionMetadata {
  availability: MentionItem["availability"];
  disabledReason?: string;
  toolId: string;
  groupLabel: string;
  action: MentionItem["action"];
}

export function MentionCatalog({
  agentId,
  selected,
  onSelect,
  onClear,
  children,
}: MentionCatalogProps) {
  const [enableError, setEnableError] = useState<string>();
  const [retryItem, setRetryItem] = useState<Unstable_TriggerItem>();
  const mentions = useQuery({
    queryKey: ["agent-mentions", agentId],
    queryFn: () => api.listAgentMentions(agentId),
    staleTime: 15_000,
  });
  const enable = useMutation({
    mutationFn: (item: Unstable_TriggerItem) => {
      const metadata = mentionMetadata(item);
      return api.bindAgentMcpTool(agentId, metadata.toolId);
    },
    onMutate: () => setEnableError(undefined),
    onSuccess: async (_, item) => {
      const metadata = mentionMetadata(item);
      onSelect({
        id: item.id,
        action: "force-tool-once",
        label: item.label,
        groupLabel: metadata.groupLabel,
      });
      setRetryItem(undefined);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["agent-mentions", agentId],
        }),
        queryClient.invalidateQueries({ queryKey: ["mcp-servers", agentId] }),
      ]);
    },
    onError: (error, item) => {
      setEnableError(error.message);
      setRetryItem(item);
    },
  });
  const categories = useMemo(
    () => [
      {
        id: "tools",
        label: "Tools",
        items: (mentions.data?.items ?? []).map((item) => ({
          id: item.id,
          type: item.kind,
          label: item.label,
          description: item.description,
          metadata: {
            availability: item.availability,
            disabledReason: item.disabledReason ?? "",
            toolId: item.toolId ?? "",
            groupLabel: item.group.label,
            action: item.action,
          },
        })),
      },
    ],
    [mentions.data],
  );
  const mention = unstable_useMentionAdapter({
    categories,
    includeModelContextTools: false,
  });

  const execute = (item: Unstable_TriggerItem) => {
    const metadata = mentionMetadata(item);
    if (metadata.availability === "ready") {
      setEnableError(undefined);
      setRetryItem(undefined);
      onSelect({
        id: item.id,
        action: "force-tool-once",
        label: item.label,
        groupLabel: metadata.groupLabel,
      });
      return;
    }
    if (
      metadata.availability === "needs-agent-enable" ||
      metadata.availability === "tool-disabled"
    ) {
      enable.mutate(item);
    }
  };

  return (
    <ComposerPrimitive.Unstable_TriggerPopoverRoot>
      <ComposerPrimitive.Unstable_TriggerPopover
        char="@"
        adapter={mention.adapter}
        isLoading={mentions.isLoading}
        className="mention-popover"
        aria-label="Mention catalog"
      >
        <ComposerPrimitive.Unstable_TriggerPopover.Action
          onExecute={execute}
          removeOnExecute
        />
        <ComposerPrimitive.Unstable_TriggerPopoverCategories className="mention-list">
          {(items) => [
            <div className="mention-popover-header" key="header">
              <div>
                <strong>Mention catalog</strong>
                <span>Choose one capability for this run</span>
              </div>
            </div>,
            ...items.map((category) => (
              <ComposerPrimitive.Unstable_TriggerPopoverCategoryItem
                key={category.id}
                categoryId={category.id}
                className="mention-category"
              >
                <span className="mention-item-icon">
                  <Wrench size={16} />
                </span>
                <span>
                  <strong>{category.label}</strong>
                  <small>{mentions.data?.items.length ?? 0} available</small>
                </span>
                <ChevronRight size={16} />
              </ComposerPrimitive.Unstable_TriggerPopoverCategoryItem>
            )),
          ]}
        </ComposerPrimitive.Unstable_TriggerPopoverCategories>
        <ComposerPrimitive.Unstable_TriggerPopoverItems className="mention-list mention-tool-list">
          {(items) => [
            <div className="mention-popover-header" key="header">
              <ComposerPrimitive.Unstable_TriggerPopoverBack
                className="mention-back"
                aria-label="Back to categories"
              >
                <ArrowLeft size={15} />
              </ComposerPrimitive.Unstable_TriggerPopoverBack>
              <div>
                <strong>Tools</strong>
                <span>Search by tool, plugin, or description</span>
              </div>
            </div>,
            ...(items.length === 0
              ? [
                  <div className="mention-empty" key="empty">
                    {mentions.isLoading
                      ? "Loading tools…"
                      : mentions.isError
                        ? "Tool catalog could not be loaded."
                        : "No matching MCP tools."}
                  </div>,
                ]
              : items.map((item, index) => {
                  const metadata = mentionMetadata(item);
                  const blocked =
                    metadata.availability === "server-offline" ||
                    metadata.availability === "approval-required";
                  return (
                    <ComposerPrimitive.Unstable_TriggerPopoverItem
                      key={item.id}
                      item={item}
                      index={index}
                      className="mention-tool"
                      disabled={blocked || enable.isPending}
                    >
                      <span className="mention-item-icon">
                        <PlugZap size={16} />
                      </span>
                      <span className="mention-tool-copy">
                        <span>
                          <strong>{item.label}</strong>
                          <em>{metadata.groupLabel}</em>
                        </span>
                        <small>
                          {item.description ||
                            metadata.disabledReason ||
                            "MCP Tool"}
                        </small>
                      </span>
                      <AvailabilityLabel availability={metadata.availability} />
                    </ComposerPrimitive.Unstable_TriggerPopoverItem>
                  );
                })),
          ]}
        </ComposerPrimitive.Unstable_TriggerPopoverItems>
      </ComposerPrimitive.Unstable_TriggerPopover>

      <div className="mention-composer-content">
        {selected && (
          <div className="composer-selection-chip" aria-label="Selected tool">
            <Wrench size={14} />
            <span>{selected.label}</span>
            <small>{selected.groupLabel}</small>
            <button
              type="button"
              onClick={onClear}
              aria-label="Remove selected tool"
            >
              <X size={13} />
            </button>
          </div>
        )}
        {children}
        {enableError && retryItem && (
          <div className="mention-enable-error" role="alert">
            <span>{enableError}</span>
            <button type="button" onClick={() => enable.mutate(retryItem)}>
              Retry enable
            </button>
          </div>
        )}
      </div>
    </ComposerPrimitive.Unstable_TriggerPopoverRoot>
  );
}

export function MentionCatalogButton() {
  const aui = useAui();
  const triggers = unstable_useTriggerPopoverTriggers();
  return (
    <button
      type="button"
      className="aui-composer-tool-button"
      aria-label="Open tools"
      onClick={() => {
        const text = aui.composer.getState().text;
        const prefix = text.length === 0 || /\s$/.test(text) ? "" : " ";
        const nextText = `${text}${prefix}@`;
        aui.composer.setText(nextText);
        triggers.get("@")?.resource.setCursorPosition(nextText.length);
        requestAnimationFrame(() => {
          document
            .querySelector<HTMLTextAreaElement>(".aui-composer-input")
            ?.focus();
        });
      }}
    >
      <Wrench size={15} />
      <span>Tools</span>
    </button>
  );
}

function AvailabilityLabel({
  availability,
}: {
  availability: MentionItem["availability"];
}) {
  const labels: Record<MentionItem["availability"], string> = {
    ready: "Ready",
    "needs-agent-enable": "Enable",
    "tool-disabled": "Enable",
    "server-offline": "Offline",
    "approval-required": "Approval",
  };
  return (
    <span className={`mention-availability ${availability}`}>
      {labels[availability]}
    </span>
  );
}

function mentionMetadata(item: Unstable_TriggerItem): MentionMetadata {
  return item.metadata as unknown as MentionMetadata;
}
