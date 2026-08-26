import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
} from "@tanstack/react-router";

import { Workspace } from "../components/Workspace";
import {
  AgentProfilePage,
  type AgentProfileSection,
} from "../features/agent/AgentProfilePage";
import { AuthGate } from "../features/account/AuthGate";
import { McpPage } from "../features/capabilities/McpPage";
import { MemoryPage } from "../features/capabilities/MemoryPage";
import { SkillsPage } from "../features/capabilities/SkillsPage";
import { SettingsPage } from "../features/settings/SettingsPage";

const rootRoute = createRootRoute({
  component: () => (
    <AuthGate>
      <Outlet />
    </AuthGate>
  ),
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: () => <Workspace />,
});

const conversationRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/conversations/$conversationId",
  component: ConversationRoute,
});

const memoryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/memory",
  component: MemoryPage,
});

const skillsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/skills",
  component: SkillsPage,
});

const mcpRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/mcp",
  component: McpPage,
});

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings",
  component: SettingsPage,
});

const agentProfileRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings/agent-profile",
  validateSearch: (search: Record<string, unknown>) => ({
    agentId:
      typeof search.agentId === "string" && search.agentId.length > 0
        ? search.agentId
        : undefined,
    section:
      search.section === "impressions" ||
      search.section === "facts" ||
      search.section === "publication"
        ? (search.section as AgentProfileSection)
        : ("overview" as AgentProfileSection),
  }),
  component: AgentProfileRoute,
});

function ConversationRoute() {
  const { conversationId } = conversationRoute.useParams();
  return <Workspace conversationId={conversationId} />;
}

function AgentProfileRoute() {
  const { agentId, section } = agentProfileRoute.useSearch();
  return <AgentProfilePage agentId={agentId} section={section} />;
}

const routeTree = rootRoute.addChildren([
  indexRoute,
  conversationRoute,
  memoryRoute,
  skillsRoute,
  mcpRoute,
  settingsRoute,
  agentProfileRoute,
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
