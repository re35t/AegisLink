import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useQuery } from "@tanstack/react-query";

import { api } from "../../api/client";

export type EffectiveLanguage = "en" | "zh-CN";
export type EffectiveTheme = "light" | "dark";

interface InterfacePreferences {
  language: EffectiveLanguage;
  theme: EffectiveTheme;
  t(english: string, chinese: string): string;
}

const PreferencesContext = createContext<InterfacePreferences>({
  language: "en",
  theme: "light",
  t: (english) => english,
});

export function InterfacePreferencesProvider({
  children,
}: {
  children: ReactNode;
}) {
  const settings = useQuery({
    queryKey: ["account-settings"],
    queryFn: api.getAccountSettings,
    retry: false,
  });
  const [systemDark, setSystemDark] = useState(() => systemPrefersDark());

  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = () => setSystemDark(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  const language = resolveLanguage(settings.data?.preferences.language);
  const theme = resolveTheme(settings.data?.preferences.theme, systemDark);

  useEffect(() => {
    document.documentElement.lang = language;
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
  }, [language, theme]);

  const value = useMemo<InterfacePreferences>(
    () => ({
      language,
      theme,
      t: (english, chinese) => (language === "zh-CN" ? chinese : english),
    }),
    [language, theme],
  );

  return (
    <PreferencesContext.Provider value={value}>
      {children}
    </PreferencesContext.Provider>
  );
}

export function useInterfacePreferences() {
  return useContext(PreferencesContext);
}

export function resolveLanguage(
  preference: "system" | "en" | "zh-CN" = "system",
  browserLanguage = typeof navigator === "undefined"
    ? "en"
    : navigator.language,
): EffectiveLanguage {
  if (preference === "zh-CN" || preference === "en") return preference;
  return browserLanguage.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

export function resolveTheme(
  preference: "system" | "light" | "dark" = "system",
  prefersDark = systemPrefersDark(),
): EffectiveTheme {
  if (preference === "light" || preference === "dark") return preference;
  return prefersDark ? "dark" : "light";
}

function systemPrefersDark() {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-color-scheme: dark)").matches
  );
}
