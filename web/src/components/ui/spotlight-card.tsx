import { useCallback, useRef, type ReactNode } from "react"
import { cn } from "@/lib/utils"

interface SpotlightCardProps {
  children: ReactNode
  className?: string
  spotlightColor?: string
}

const prefersReducedMotion = () =>
  typeof window !== "undefined" &&
  window.matchMedia("(prefers-reduced-motion: reduce)").matches

export function SpotlightCard({
  children,
  className,
  spotlightColor,
}: SpotlightCardProps) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const colorRef = useRef<string>("")

  const handleMouseMove = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      if (!overlayRef.current || !containerRef.current) return
      const rect = containerRef.current.getBoundingClientRect()
      const x = e.clientX - rect.left
      const y = e.clientY - rect.top
      overlayRef.current.style.background = `radial-gradient(circle 250px at ${x}px ${y}px, ${colorRef.current}, transparent)`
    },
    [],
  )

  const handleMouseEnter = useCallback(() => {
    if (prefersReducedMotion() || !overlayRef.current) return
    const isDark = document.documentElement.classList.contains("dark")
    colorRef.current =
      spotlightColor ??
      (isDark ? "rgba(139, 92, 246, 0.15)" : "rgba(139, 92, 246, 0.08)")
    overlayRef.current.style.opacity = "1"
  }, [spotlightColor])

  const handleMouseLeave = useCallback(() => {
    if (!overlayRef.current) return
    overlayRef.current.style.opacity = "0"
  }, [])

  return (
    <div
      ref={containerRef}
      className={cn("relative overflow-hidden rounded-xl", className)}
      onMouseMove={handleMouseMove}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <div
        ref={overlayRef}
        className="pointer-events-none absolute inset-0 z-10 opacity-0"
        style={{ transition: "opacity 300ms ease" }}
      />
      {children}
    </div>
  )
}
