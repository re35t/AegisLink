import { useCallback, useEffect, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { api, type RunEvent } from "../api/client";
import { queryClient } from "../app/queryClient";
import { ConversationSidebar } from "../features/conversation/ConversationSidebar";
import { ConversationWorkspace } from "../features/conversation/ConversationWorkspace";
import { RunRecovery } from "../features/conversation/RunRecovery";
import { AppShell } from "./layout/AppShell";
import { useRunEvents } from "./useRunEvents";

interface WorkspaceProps {
  conversationId?: string;
}

export function Workspace({ conversationId }: WorkspaceProps) {
  const navigate = useNavigate();
  const [navigationOpen, setNavigationOpen] = useState(false);
  const [assistantRunning, setAssistantRunning] = useState(false);
  const [recoveryText, setRecoveryText] = useState("");
  const [recoveryConnected, setRecoveryConnected] = useState(false);

  const bootstrap = useQuery({
    queryKey: ["bootstrap"],
    queryFn: api.bootstrap,
  });
  const conversations = useQuery({
    queryKey: ["conversations"],
    queryFn: api.listConversations,
  });
  const detail = useQuery({
    queryKey: ["conversation", conversationId],
    queryFn: () => api.getConversation(conversationId!),
    enabled: Boolean(conversationId),
  });

  useEffect(() => {
    setAssistantRunning(false);
    setRecoveryText("");
  }, [conversationId]);

  const refreshConversation = useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: ["conversation", conversationId],
      }),
      queryClient.invalidateQueries({ queryKey: ["conversations"] }),
    ]);
  }, [conversationId]);

  const createConversation = useMutation({
    mutationFn: () => api.createConversation(),
    onSuccess: async (created) => {
      await queryClient.invalidateQueries({ queryKey: ["conversations"] });
      setNavigationOpen(false);
      navigate({
        to: "/conversations/$conversationId",
        params: { conversationId: created.id },
      });
    },
  });
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: async () => {
      await navigate({ to: "/" });
      queryClient.removeQueries({
        predicate: (query) => query.queryKey[0] !== "session",
      });
      await queryClient.resetQueries({ queryKey: ["session"] });
    },
  });

  const startCreate = useCallback(() => {
    createConversation.reset();
    createConversation.mutate();
  }, [createConversation]);

  const selectConversation = useCallback(
    (selectedConversationId: string) => {
      setNavigationOpen(false);
      navigate({
        to: "/conversations/$conversationId",
        params: { conversationId: selectedConversationId },
      });
    },
    [navigate],
  );

  const activeRunId = detail.data?.activeRun?.id;
  const finishRecoveredRun = useCallback(
    async (_event: RunEvent) => {
      setRecoveryConnected(false);
      setRecoveryText("");
      await refreshConversation();
    },
    [refreshConversation],
  );
  useRunEvents(activeRunId, {
    onDelta: (delta) => setRecoveryText((current) => current + delta),
    onTerminal: (event) => void finishRecoveredRun(event),
    onConnectionChange: setRecoveryConnected,
  });

  const streamState = activeRunId
    ? recoveryConnected
      ? "Recovering"
      : "Reconnecting"
    : assistantRunning
      ? "Streaming"
      : "Ready";
  const agentName = bootstrap.data?.agent.name ?? "Aegis";
  const agentDescription =
    bootstrap.data?.agent.description ?? "A private, focused personal agent.";
  const modelLabel = bootstrap.data?.model.name ?? "Model unavailable";
  const recovery =
    detail.data && activeRunId ? (
      <RunRecovery
        messages={detail.data.messages}
        streamed={recoveryText}
        agentName={agentName}
      />
    ) : undefined;

  return (
    <AppShell
      activeArea="chat"
      navigationOpen={navigationOpen}
      onCloseNavigation={() => setNavigationOpen(false)}
      navigation={
        <ConversationSidebar
          conversations={conversations.data}
          activeConversationId={conversationId}
          loading={conversations.isLoading}
          failed={conversations.isError}
          creating={createConversation.isPending}
          loggingOut={logout.isPending}
          creationError={createConversation.error?.message}
          agentName={agentName}
          modelLabel={modelLabel}
          onCreate={startCreate}
          onLogout={() => logout.mutate()}
          onRetry={() => void conversations.refetch()}
          onSelect={selectConversation}
        />
      }
    >
      <ConversationWorkspace
        conversationId={conversationId}
        detail={detail.data}
        loading={detail.isLoading}
        failed={detail.isError}
        streamState={streamState}
        agentId={bootstrap.data?.agent.id ?? "aegis"}
        agentName={agentName}
        agentDescription={agentDescription}
        modelLabel={modelLabel}
        creating={createConversation.isPending}
        recovery={recovery}
        onCreate={startCreate}
        onOpenNavigation={() => setNavigationOpen(true)}
        onRetry={() => void detail.refetch()}
        onRunningChange={setAssistantRunning}
        onSettled={refreshConversation}
      />
    </AppShell>
  );
}
