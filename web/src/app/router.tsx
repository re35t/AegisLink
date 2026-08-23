import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
} from "@tanstack/react-router";

import { Workspace } from "../components/Workspace";
import { AuthGate } from "../features/account/AuthGate";

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

function ConversationRoute() {
  const { conversationId } = conversationRoute.useParams();
  return <Workspace conversationId={conversationId} />;
}

const routeTree = rootRoute.addChildren([indexRoute, conversationRoute]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
