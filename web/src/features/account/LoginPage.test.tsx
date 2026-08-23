import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LoginPage } from "./LoginPage";

describe("LoginPage", () => {
  it("switches from sign in to account and agent registration", () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <LoginPage onAuthenticated={vi.fn()} />
      </QueryClientProvider>,
    );

    expect(
      screen.queryByRole("textbox", { name: "Display name" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "Create account" }));

    expect(
      screen.getByRole("textbox", { name: "Display name" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create account and agent" }),
    ).toBeInTheDocument();
  });
});
