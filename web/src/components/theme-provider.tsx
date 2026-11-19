import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react"

type Theme = "dark" | "light" | "system"

type ThemeProviderProps = {
  children: ReactNode
  defaultTheme?: Theme
  storageKey?: string
}

type ThemeProviderState = {
  theme: Theme
  setTheme: (theme: Theme) => void
  resolvedTheme: Exclude<Theme, "system">
}

const initialState: ThemeProviderState = {
  theme: "system",
  setTheme: () => null,
  resolvedTheme: "light",
}

const ThemeProviderContext = createContext<ThemeProviderState>(initialState)

export function ThemeProvider({
  children,
  defaultTheme = "dark",
  storageKey = "s-ui-theme",
  ...props
}: ThemeProviderProps) {
  const [theme, setThemeState] = useState<Theme>(
    () => (localStorage.getItem(storageKey) as Theme) || defaultTheme
  )
  const [resolvedTheme, setResolvedTheme] = useState<"dark" | "light">("light")

  useEffect(() => {
    const root = window.document.documentElement
    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)")

    const applyTheme = (nextTheme: Theme) => {
      const actualTheme =
        nextTheme === "system"
          ? mediaQuery.matches
            ? "dark"
            : "light"
          : nextTheme

      root.classList.remove("light", "dark")
      root.classList.add(actualTheme)
      root.setAttribute("data-theme", actualTheme)
      root.style.colorScheme = actualTheme
      setResolvedTheme(actualTheme)
    }

    applyTheme(theme)

    const handleChange = () => applyTheme(theme)
    mediaQuery.addEventListener("change", handleChange)

    return () => mediaQuery.removeEventListener("change", handleChange)
  }, [theme])

  const value = useMemo(
    () => ({
      theme,
      resolvedTheme,
      setTheme: (t: Theme) => {
        localStorage.setItem(storageKey, t)
        setThemeState(t)
      },
    }),
    [resolvedTheme, storageKey, theme]
  )

  return (
    <ThemeProviderContext.Provider {...props} value={value}>
      {children}
    </ThemeProviderContext.Provider>
  )
}

export const useTheme = () => {
  const context = useContext(ThemeProviderContext)
  if (!context) {
    throw new Error("useTheme must be used within a ThemeProvider")
  }
  return context
}
