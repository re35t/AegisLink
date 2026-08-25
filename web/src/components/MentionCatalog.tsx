import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  ComposerPrimitive,
  type Unstable_TriggerItem,
  unstable_useMentionAdapter,
} from "@assistant-ui/react";
import {
  ArrowLeft,
  BookOpen,
  ChevronRight,
  Compass,
  Plus,
  PlugZap,
  X,
} from "lucide-react";

import { api, type MentionItem } from "../api/client";
import { queryClient } from "../app/queryClient";

type MentionCategory = MentionItem["category"];

export interface SelectedMention {
  id: string;
  kind: MentionItem["kind"];
  action: MentionItem["action"];
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
  resourceId: string;
  groupLabel: string;
  action: MentionItem["action"];
  kind: MentionItem["kind"];
  category: MentionCategory;
  source: MentionItem;
}

interface PickerContextValue {
  buttonRef: RefObject<HTMLButtonElement | null>;
  open: boolean;
  toggle(): void;
}

const PickerContext = createContext<PickerContextValue | null>(null);

const categoryDefinitions: Array<{
  id: MentionCategory;
  label: string;
  description: string;
}> = [
  { id: "mcp", label: "MCP", description: "Connected tools" },
  { id: "skills", label: "Skills", description: "Agent instructions" },
  { id: "discovery", label: "Discovery", description: "Find capabilities" },
];

export function MentionCatalog({
  agentId,
  selected,
  onSelect,
  onClear,
  children,
}: MentionCatalogProps) {
  const [enableError, setEnableError] = useState<string>();
  const [retryItem, setRetryItem] = useState<MentionItem>();
  const [plusOpen, setPlusOpen] = useState(false);
  const [plusCategory, setPlusCategory] = useState<MentionCategory>();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const mentions = useQuery({
    queryKey: ["agent-mentions", agentId],
    queryFn: () => api.listAgentMentions(agentId),
    staleTime: 15_000,
  });

  const selectItem = (item: MentionItem) => {
    setEnableError(undefined);
    setRetryItem(undefined);
    setPlusOpen(false);
    onSelect({
      id: item.id,
      kind: item.kind,
      action: item.action,
      label: item.label,
      groupLabel: item.group.label,
    });
  };

  const enable = useMutation({
    mutationFn: async (item: MentionItem) => {
      if (item.kind === "mcp-tool") {
        await api.bindAgentMcpTool(agentId, item.resourceId);
        return;
      }
      if (item.kind === "skill") {
        await api.updateSkill(agentId, item.resourceId, true);
        return;
      }
      throw new Error("This capability cannot be enabled.");
    },
    onMutate: () => setEnableError(undefined),
    onSuccess: async (_, item) => {
      selectItem(item);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["agent-mentions", agentId],
        }),
        queryClient.invalidateQueries({ queryKey: ["mcp-servers", agentId] }),
        queryClient.invalidateQueries({ queryKey: ["skills", agentId] }),
      ]);
    },
    onError: (error, item) => {
      setEnableError(error.message);
      setRetryItem(item);
    },
  });

  const execute = (item: MentionItem) => {
    if (item.availability === "ready") {
      selectItem(item);
      return;
    }
    if (
      item.availability === "needs-agent-enable" ||
      item.availability === "tool-disabled" ||
      item.availability === "skill-disabled"
    ) {
      enable.mutate(item);
    }
  };

  const categories = useMemo(
    () =>
      categoryDefinitions.map((category) => ({
        id: category.id,
        label: category.label,
        items: (mentions.data?.items ?? [])
          .filter((item) => item.category === category.id)
          .map(toTriggerItem),
      })),
    [mentions.data],
  );
  const mention = unstable_useMentionAdapter({
    categories,
    includeModelContextTools: false,
  });

  useEffect(() => {
    if (!plusOpen) return;
    const closeOnOutsidePointer = (event: PointerEvent) => {
      const target = event.target as Node;
      if (
        !menuRef.current?.contains(target) &&
        !buttonRef.current?.contains(target)
      ) {
        setPlusOpen(false);
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setPlusOpen(false);
      buttonRef.current?.focus();
    };
    document.addEventListener("pointerdown", closeOnOutsidePointer);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [plusOpen]);

  const triggerExecute = (item: Unstable_TriggerItem) =>
    execute(mentionMetadata(item).source);

  return (
    <PickerContext.Provider
      value={{
        buttonRef,
        open: plusOpen,
        toggle: () => {
          setPlusCategory(undefined);
          setPlusOpen((open) => !open);
        },
      }}
    >
      <ComposerPrimitive.Unstable_TriggerPopoverRoot>
        <ComposerPrimitive.Unstable_TriggerPopover
          char="@"
          adapter={mention.adapter}
          isLoading={mentions.isLoading}
          className="mention-popover"
          aria-label="Capability catalog"
        >
          <ComposerPrimitive.Unstable_TriggerPopover.Action
            onExecute={triggerExecute}
            removeOnExecute
          />
          <ComposerPrimitive.Unstable_TriggerPopoverCategories className="mention-list">
            {(items) => [
              <div className="mention-menu-label" key="label">
                Add capability
              </div>,
              ...items.map((category) => {
                const definition = categoryDefinitions.find(
                  (item) => item.id === category.id,
                );
                const count =
                  mentions.data?.items.filter(
                    (item) => item.category === category.id,
                  ).length ?? 0;
                return (
                  <ComposerPrimitive.Unstable_TriggerPopoverCategoryItem
                    key={category.id}
                    categoryId={category.id}
                    className="mention-category"
                  >
                    <span className="mention-item-icon">
                      <CategoryIcon category={category.id as MentionCategory} />
                    </span>
                    <span>
                      <strong>{category.label}</strong>
                      <small>{definition?.description}</small>
                    </span>
                    <span className="mention-category-count">{count}</span>
                    <ChevronRight size={14} />
                  </ComposerPrimitive.Unstable_TriggerPopoverCategoryItem>
                );
              }),
            ]}
          </ComposerPrimitive.Unstable_TriggerPopoverCategories>
          <ComposerPrimitive.Unstable_TriggerPopoverItems className="mention-list mention-capability-list">
            {(items) => [
              <div className="mention-submenu-heading" key="heading">
                <ComposerPrimitive.Unstable_TriggerPopoverBack
                  className="mention-back"
                  aria-label="Back to capability types"
                >
                  <ArrowLeft size={14} />
                </ComposerPrimitive.Unstable_TriggerPopoverBack>
                <strong>Choose capability</strong>
              </div>,
              ...(items.length === 0
                ? [
                    <CatalogEmptyState
                      key="empty"
                      loading={mentions.isLoading}
                      error={mentions.isError}
                    />,
                  ]
                : items.map((item, index) => (
                    <ComposerPrimitive.Unstable_TriggerPopoverItem
                      key={item.id}
                      item={item}
                      index={index}
                      className="mention-capability"
                      disabled={
                        isBlocked(mentionMetadata(item).availability) ||
                        enable.isPending
                      }
                    >
                      <MentionRow item={mentionMetadata(item).source} />
                    </ComposerPrimitive.Unstable_TriggerPopoverItem>
                  ))),
            ]}
          </ComposerPrimitive.Unstable_TriggerPopoverItems>
        </ComposerPrimitive.Unstable_TriggerPopover>

        {plusOpen && (
          <PlusCapabilityMenu
            ref={menuRef}
            items={mentions.data?.items ?? []}
            activeCategory={plusCategory}
            onCategoryChange={setPlusCategory}
            onExecute={execute}
            onClose={() => {
              setPlusOpen(false);
              buttonRef.current?.focus();
            }}
            pending={enable.isPending}
            loading={mentions.isLoading}
            error={mentions.isError}
          />
        )}

        <div className="mention-composer-content">
          {selected && (
            <div
              className={`composer-selection-chip ${selected.kind}`}
              aria-label="Selected capability"
            >
              <CategoryIcon
                category={categoryForKind(selected.kind)}
                size={14}
              />
              <span>{selected.label}</span>
              <small>{selected.groupLabel}</small>
              <button
                type="button"
                onClick={onClear}
                aria-label="Remove selected capability"
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
    </PickerContext.Provider>
  );
}

export function MentionCatalogButton() {
  const picker = useContext(PickerContext);
  if (!picker) {
    throw new Error(
      "MentionCatalogButton must be rendered inside MentionCatalog",
    );
  }
  return (
    <button
      ref={picker.buttonRef}
      type="button"
      className="aui-composer-plus-button"
      aria-label="Add capability"
      aria-haspopup="menu"
      aria-expanded={picker.open}
      onClick={picker.toggle}
    >
      <Plus size={18} />
    </button>
  );
}

const PlusCapabilityMenu = function PlusCapabilityMenu({
  ref,
  items,
  activeCategory,
  onCategoryChange,
  onExecute,
  onClose,
  pending,
  loading,
  error,
}: {
  ref: RefObject<HTMLDivElement | null>;
  items: MentionItem[];
  activeCategory?: MentionCategory;
  onCategoryChange(category?: MentionCategory): void;
  onExecute(item: MentionItem): void;
  onClose(): void;
  pending: boolean;
  loading: boolean;
  error: boolean;
}) {
  const categoryRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const itemRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const activeItems = items.filter((item) => item.category === activeCategory);

  useEffect(() => {
    categoryRefs.current[0]?.focus();
  }, []);

  const openCategory = (category: MentionCategory, focusItems = false) => {
    onCategoryChange(category);
    if (focusItems) {
      requestAnimationFrame(() => itemRefs.current[0]?.focus());
    }
  };

  return (
    <div className="plus-capability-menu" ref={ref}>
      <div
        className="plus-capability-types"
        role="menu"
        aria-label="Add capability"
      >
        {categoryDefinitions.map((category, index) => {
          const count = items.filter(
            (item) => item.category === category.id,
          ).length;
          return (
            <button
              key={category.id}
              ref={(element) => {
                categoryRefs.current[index] = element;
              }}
              type="button"
              role="menuitem"
              className={activeCategory === category.id ? "active" : ""}
              aria-haspopup="menu"
              aria-expanded={activeCategory === category.id}
              onClick={() => openCategory(category.id)}
              onKeyDown={(event) => {
                if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                  event.preventDefault();
                  const direction = event.key === "ArrowDown" ? 1 : -1;
                  const next =
                    (index + direction + categoryDefinitions.length) %
                    categoryDefinitions.length;
                  categoryRefs.current[next]?.focus();
                }
                if (event.key === "ArrowRight" || event.key === "Enter") {
                  event.preventDefault();
                  openCategory(category.id, true);
                }
              }}
            >
              <CategoryIcon category={category.id} size={17} />
              <span>
                <strong>{category.label}</strong>
                <small>{count}</small>
              </span>
              <ChevronRight size={14} />
            </button>
          );
        })}
      </div>

      {activeCategory && (
        <div
          className="plus-capability-submenu"
          role="menu"
          aria-label={`${categoryLabel(activeCategory)} capabilities`}
        >
          <div className="plus-submenu-header">
            <button
              type="button"
              aria-label="Back to capability types"
              onClick={() => onCategoryChange(undefined)}
            >
              <ArrowLeft size={14} />
            </button>
            <strong>{categoryLabel(activeCategory)}</strong>
          </div>
          {activeItems.length === 0 ? (
            <CatalogEmptyState loading={loading} error={error} />
          ) : (
            <div className="plus-submenu-items">
              {activeItems.map((item, index) => (
                <button
                  key={item.id}
                  ref={(element) => {
                    itemRefs.current[index] = element;
                  }}
                  type="button"
                  role="menuitem"
                  disabled={isBlocked(item.availability) || pending}
                  onClick={() => onExecute(item)}
                  onKeyDown={(event) => {
                    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
                      event.preventDefault();
                      const direction = event.key === "ArrowDown" ? 1 : -1;
                      const next =
                        (index + direction + activeItems.length) %
                        activeItems.length;
                      itemRefs.current[next]?.focus();
                    }
                    if (event.key === "ArrowLeft") {
                      event.preventDefault();
                      onCategoryChange(undefined);
                      categoryRefs.current[
                        categoryDefinitions.findIndex(
                          (category) => category.id === activeCategory,
                        )
                      ]?.focus();
                    }
                    if (event.key === "Escape") onClose();
                  }}
                >
                  <MentionRow item={item} />
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

function MentionRow({ item }: { item: MentionItem }) {
  return (
    <>
      <span className="mention-item-icon">
        <CategoryIcon category={item.category} />
      </span>
      <span className="mention-capability-copy">
        <span>
          <strong>{item.label}</strong>
          <em>{item.group.label}</em>
        </span>
        <small>{item.description || item.disabledReason || item.kind}</small>
      </span>
      <AvailabilityLabel availability={item.availability} />
    </>
  );
}

function CatalogEmptyState({
  loading,
  error,
}: {
  loading: boolean;
  error: boolean;
}) {
  return (
    <div className="mention-empty">
      {loading
        ? "Loading…"
        : error
          ? "Capabilities could not be loaded."
          : "No capabilities in this category."}
    </div>
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
    "skill-disabled": "Enable",
    "server-offline": "Offline",
    "approval-required": "Approval",
  };
  return (
    <span className={`mention-availability ${availability}`}>
      {labels[availability]}
    </span>
  );
}

function CategoryIcon({
  category,
  size = 16,
}: {
  category: MentionCategory;
  size?: number;
}) {
  if (category === "mcp") return <PlugZap size={size} />;
  if (category === "skills") return <BookOpen size={size} />;
  return <Compass size={size} />;
}

function categoryForKind(kind: MentionItem["kind"]): MentionCategory {
  if (kind === "mcp-tool") return "mcp";
  if (kind === "skill") return "skills";
  return "discovery";
}

function categoryLabel(category: MentionCategory) {
  return (
    categoryDefinitions.find((definition) => definition.id === category)
      ?.label ?? category
  );
}

function isBlocked(availability: MentionItem["availability"]) {
  return (
    availability === "server-offline" || availability === "approval-required"
  );
}

function toTriggerItem(item: MentionItem): Unstable_TriggerItem {
  return {
    id: item.id,
    type: item.kind,
    label: item.label,
    description: item.description,
    metadata: {
      availability: item.availability,
      disabledReason: item.disabledReason ?? "",
      resourceId: item.resourceId,
      groupLabel: item.group.label,
      action: item.action,
      kind: item.kind,
      category: item.category,
      source: item,
    } satisfies MentionMetadata,
  };
}

function mentionMetadata(item: Unstable_TriggerItem): MentionMetadata {
  return item.metadata as unknown as MentionMetadata;
}
